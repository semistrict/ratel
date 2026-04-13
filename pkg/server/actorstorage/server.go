// Copyright 2026 The Ratel Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package actorstorage

import (
	"context"
	"fmt"

	capnp "capnproto.org/go/capnp/v3"
	"github.com/semistrict/ratel/pkg/keys"
	"github.com/semistrict/ratel/pkg/kv"
	"github.com/semistrict/ratel/pkg/roachpb"
	"github.com/semistrict/ratel/pkg/util/encoding"
	"github.com/semistrict/ratel/pkg/util/log"
	"github.com/semistrict/ratel/pkg/util/tracing"
)

const maxListLimit = 10000

// StorageServer implements the ActorStorage capnp interface. It
// dispatches getStage calls to per-actor Stage implementations backed
// by Ratel KV.
type StorageServer struct {
	DB     *kv.DB
	Codec  keys.SQLCodec
	Tracer *tracing.Tracer

	// ParentSpan, if set, is used as the parent for all storage operation
	// spans. This is useful for tests that want to collect a complete
	// trace tree. In production this is nil and spans are roots.
	ParentSpan *tracing.Span
}

// GetStage returns a Stage capability scoped to the given actor.
func (s *StorageServer) GetStage(ctx context.Context, call ActorStorage_getStage) error {
	args := call.Args()
	stableID, err := args.StableId()
	if err != nil {
		return fmt.Errorf("reading stableId: %w", err)
	}

	hash := actorHashFromStableId(stableID)

	stage := &stageServer{
		db:         s.DB,
		codec:      s.Codec,
		hash:       hash,
		tracer:     s.Tracer,
		parentSpan: s.ParentSpan,
	}

	results, err := call.AllocResults()
	if err != nil {
		return err
	}
	client := ActorStorage_Stage_ServerToClient(stage)
	return results.SetStage(client)
}

// actorHashFromStableId converts a workerd stableId into a 16-byte actor
// hash for use as a KV key prefix. workerd sends the stableId as a hex
// string whose length depends on the hash algorithm used (typically 32
// bytes / 64 hex chars for SHA-256). We use ActorHash to produce a
// consistent 16-byte prefix regardless of input length.
func actorHashFromStableId(stableId string) [keys.ActorHashLen]byte {
	return keys.ActorHash(stableId)
}

// stageServer implements ActorStorage_Stage_Server for a single actor.
type stageServer struct {
	db         *kv.DB
	codec      keys.SQLCodec
	hash       [keys.ActorHashLen]byte
	tracer     *tracing.Tracer
	parentSpan *tracing.Span
}

// startSpan creates a child span if a tracer is configured.
func (s *stageServer) startSpan(ctx context.Context, opName string) (context.Context, *tracing.Span) {
	if s.tracer == nil {
		return ctx, nil
	}
	var opts []tracing.SpanOption
	if s.parentSpan != nil {
		opts = append(opts, tracing.WithParent(s.parentSpan))
	}
	return s.tracer.StartSpanCtx(ctx, opName, opts...)
}

func (s *stageServer) Get(ctx context.Context, call ActorStorage_Operations_get) error {
	ctx, span := s.startSpan(ctx, "do.storage.get")
	if span != nil {
		defer span.Finish()
	}
	args := call.Args()
	userKey, err := args.Key()
	if err != nil {
		return err
	}
	kvKey := keys.MakeDOKVKey(s.codec.TenantPrefix(), s.hash, userKey)
	result, err := s.db.Get(ctx, kvKey)
	if err != nil {
		log.Warningf(ctx, "DO storage get error: %v", err)
		return fmt.Errorf("storage error")
	}

	if result.Value != nil {
		valBytes, err := result.Value.GetBytes()
		if err != nil {
			return fmt.Errorf("storage error")
		}
		results, err := call.AllocResults()
		if err != nil {
			return err
		}
		return results.SetValue(valBytes)
	}
	return nil
}

func (s *stageServer) Put(ctx context.Context, call ActorStorage_Operations_put) error {
	ctx, span := s.startSpan(ctx, "do.storage.put")
	if span != nil {
		defer span.Finish()
	}
	args := call.Args()
	entries, err := args.Entries()
	if err != nil {
		return err
	}

	b := &kv.Batch{}
	for i := 0; i < entries.Len(); i++ {
		entry := entries.At(i)
		userKey, err := entry.Key()
		if err != nil {
			return err
		}
		val, err := entry.Value()
		if err != nil {
			return err
		}
		kvKey := keys.MakeDOKVKey(s.codec.TenantPrefix(), s.hash, userKey)
		b.Put(kvKey, val)
	}

	if err := s.db.Run(ctx, b); err != nil {
		log.Warningf(ctx, "DO storage put error: %v", err)
		return fmt.Errorf("storage error")
	}
	return nil
}

func (s *stageServer) Delete(ctx context.Context, call ActorStorage_Operations_delete) error {
	ctx, span := s.startSpan(ctx, "do.storage.delete")
	if span != nil {
		defer span.Finish()
	}
	args := call.Args()
	keysList, err := args.Keys()
	if err != nil {
		return err
	}

	kvKeys := make([]interface{}, keysList.Len())
	for i := 0; i < keysList.Len(); i++ {
		userKey, err := keysList.At(i)
		if err != nil {
			return err
		}
		kvKeys[i] = keys.MakeDOKVKey(s.codec.TenantPrefix(), s.hash, userKey)
	}

	if err := s.db.Del(ctx, kvKeys...); err != nil {
		log.Warningf(ctx, "DO storage delete error: %v", err)
		return fmt.Errorf("storage error")
	}

	results, err := call.AllocResults()
	if err != nil {
		return err
	}
	results.SetNumDeleted(int32(keysList.Len()))
	return nil
}

func (s *stageServer) List(ctx context.Context, call ActorStorage_Operations_list) error {
	ctx, span := s.startSpan(ctx, "do.storage.list")
	if span != nil {
		defer span.Finish()
	}
	args := call.Args()
	startKey, err := args.Start()
	if err != nil {
		return err
	}
	endKey, err := args.End()
	if err != nil {
		return err
	}
	limit := args.Limit()
	stream := args.Stream()

	doPrefix := keys.MakeDOKVPrefix(s.codec.TenantPrefix(), s.hash)

	var begin, end roachpb.Key
	if len(startKey) > 0 {
		begin = keys.MakeDOKVKey(s.codec.TenantPrefix(), s.hash, startKey)
	} else {
		begin = doPrefix
	}
	if len(endKey) > 0 {
		end = keys.MakeDOKVKey(s.codec.TenantPrefix(), s.hash, endKey)
	} else {
		end = doPrefix.PrefixEnd()
	}

	if limit <= 0 {
		limit = 1000
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}

	rows, err := s.db.Scan(ctx, begin, end, int64(limit))
	if err != nil {
		log.Warningf(ctx, "DO storage list error: %v", err)
		return fmt.Errorf("storage error")
	}

	if len(rows) > 0 {
		if err := stream.Values(ctx, func(p ActorStorage_ListStream_values_Params) error {
			list, err := p.NewList(int32(len(rows)))
			if err != nil {
				return err
			}
			for i, row := range rows {
				rem := row.Key[len(doPrefix):]
				userKey, decErr := decodeBytesKey(rem)
				if decErr != nil {
					return decErr
				}
				valBytes, valErr := row.Value.GetBytes()
				if valErr != nil {
					return valErr
				}
				if err := list.At(i).SetKey(userKey); err != nil {
					return err
				}
				if err := list.At(i).SetValue(valBytes); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}

	_, release := stream.End(ctx, nil)
	defer release()
	return nil
}

func (s *stageServer) GetMultiple(ctx context.Context, call ActorStorage_Operations_getMultiple) error {
	args := call.Args()
	keysList, err := args.Keys()
	if err != nil {
		return err
	}
	stream := args.Stream()

	if keysList.Len() > 0 {
		kvKeys := make([]roachpb.Key, keysList.Len())
		for i := 0; i < keysList.Len(); i++ {
			userKey, err := keysList.At(i)
			if err != nil {
				return err
			}
			kvKeys[i] = keys.MakeDOKVKey(s.codec.TenantPrefix(), s.hash, userKey)
		}

		b := &kv.Batch{}
		for _, k := range kvKeys {
			b.Get(k)
		}
		if err := s.db.Run(ctx, b); err != nil {
			log.Warningf(ctx, "DO storage getMultiple error: %v", err)
			return fmt.Errorf("storage error")
		}

		doPrefix := keys.MakeDOKVPrefix(s.codec.TenantPrefix(), s.hash)
		var entries []kvEntry
		for i, result := range b.Results {
			if len(result.Rows) == 0 {
				continue
			}
			row := result.Rows[0]
			if row.Value == nil {
				continue
			}
			// Use the original user key from the request.
			userKey, err := keysList.At(i)
			if err != nil {
				return err
			}
			valBytes, err := row.Value.GetBytes()
			if err != nil {
				return err
			}
			_ = doPrefix // used for list decoding, not here
			entries = append(entries, kvEntry{key: userKey, value: valBytes})
		}

		if len(entries) > 0 {
			if err := stream.Values(ctx, func(p ActorStorage_ListStream_values_Params) error {
				list, err := p.NewList(int32(len(entries)))
				if err != nil {
					return err
				}
				for i, e := range entries {
					if err := list.At(i).SetKey(e.key); err != nil {
						return err
					}
					if err := list.At(i).SetValue(e.value); err != nil {
						return err
					}
				}
				return nil
			}); err != nil {
				return err
			}
		}
	}

	_, release := stream.End(ctx, nil)
	defer release()
	return nil
}

type kvEntry struct {
	key   []byte
	value []byte
}

func (s *stageServer) DeleteAll(ctx context.Context, call ActorStorage_Operations_deleteAll) error {
	ctx, span := s.startSpan(ctx, "do.storage.deleteAll")
	if span != nil {
		defer span.Finish()
	}
	doPrefix := keys.MakeDOKVPrefix(s.codec.TenantPrefix(), s.hash)
	_, err := s.db.DelRange(ctx, doPrefix, doPrefix.PrefixEnd(), false)
	if err != nil {
		log.Warningf(ctx, "DO storage deleteAll error: %v", err)
		return fmt.Errorf("storage error")
	}
	return nil
}

func (s *stageServer) Rename(_ context.Context, _ ActorStorage_Operations_rename) error {
	return fmt.Errorf("rename not supported")
}

func (s *stageServer) GetAlarm(_ context.Context, call ActorStorage_Operations_getAlarm) error {
	results, err := call.AllocResults()
	if err != nil {
		return err
	}
	results.SetScheduledTimeMs(0)
	return nil
}

func (s *stageServer) SetAlarm(_ context.Context, _ ActorStorage_Operations_setAlarm) error {
	return nil
}

func (s *stageServer) DeleteAlarm(_ context.Context, call ActorStorage_Operations_deleteAlarm) error {
	results, err := call.AllocResults()
	if err != nil {
		return err
	}
	results.SetDeleted(false)
	return nil
}

func (s *stageServer) Txn(_ context.Context, call ActorStorage_Stage_txn) error {
	txnServer := &transactionServer{stage: s}
	results, err := call.AllocResults()
	if err != nil {
		return err
	}
	client := ActorStorage_Stage_Transaction_ServerToClient(txnServer)
	return results.SetTransaction(client)
}

// transactionServer delegates all operations to the parent stage.
// Commit and rollback are no-ops because workerd's ActorCache handles
// transaction semantics — the server just sees individual operations.
type transactionServer struct {
	stage *stageServer
}

func (t *transactionServer) Get(ctx context.Context, call ActorStorage_Operations_get) error {
	return t.stage.Get(ctx, call)
}
func (t *transactionServer) Put(ctx context.Context, call ActorStorage_Operations_put) error {
	return t.stage.Put(ctx, call)
}
func (t *transactionServer) Delete(ctx context.Context, call ActorStorage_Operations_delete) error {
	return t.stage.Delete(ctx, call)
}
func (t *transactionServer) List(ctx context.Context, call ActorStorage_Operations_list) error {
	return t.stage.List(ctx, call)
}
func (t *transactionServer) GetMultiple(ctx context.Context, call ActorStorage_Operations_getMultiple) error {
	return t.stage.GetMultiple(ctx, call)
}
func (t *transactionServer) DeleteAll(ctx context.Context, call ActorStorage_Operations_deleteAll) error {
	return t.stage.DeleteAll(ctx, call)
}
func (t *transactionServer) GetAlarm(ctx context.Context, call ActorStorage_Operations_getAlarm) error {
	return t.stage.GetAlarm(ctx, call)
}
func (t *transactionServer) SetAlarm(ctx context.Context, call ActorStorage_Operations_setAlarm) error {
	return t.stage.SetAlarm(ctx, call)
}
func (t *transactionServer) DeleteAlarm(ctx context.Context, call ActorStorage_Operations_deleteAlarm) error {
	return t.stage.DeleteAlarm(ctx, call)
}
func (t *transactionServer) Rename(ctx context.Context, call ActorStorage_Operations_rename) error {
	return t.stage.Rename(ctx, call)
}
func (t *transactionServer) Commit(_ context.Context, _ ActorStorage_Stage_Transaction_commit) error {
	return nil
}
func (t *transactionServer) Rollback(_ context.Context, _ ActorStorage_Stage_Transaction_rollback) error {
	return nil
}

// NewClient creates a capnp.Client from a StorageServer for use as the
// bootstrap capability of an RPC connection.
func NewClient(s *StorageServer) capnp.Client {
	return capnp.Client(ActorStorage_ServerToClient(s))
}

// decodeBytesKey decodes a single encoded-bytes key from the given slice.
func decodeBytesKey(b []byte) ([]byte, error) {
	_, val, err := encoding.DecodeBytesAscending(b, nil)
	return val, err
}
