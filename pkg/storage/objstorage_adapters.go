// Copyright 2024 The Cockroach Authors.
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

package storage

import (
	"context"
	"io"

	"github.com/cockroachdb/pebble/objstorage"
	"github.com/cockroachdb/pebble/sstable"
)

// memReadable adapts an in-memory byte slice into an objstorage.Readable.
type memReadable struct {
	data []byte
}

var _ objstorage.Readable = (*memReadable)(nil)

func newMemReadable(data []byte) *memReadable {
	return &memReadable{data: data}
}

func (r *memReadable) ReadAt(_ context.Context, p []byte, off int64) error {
	copy(p, r.data[off:off+int64(len(p))])
	return nil
}

func (r *memReadable) Close() error { return nil }

func (r *memReadable) Size() int64 { return int64(len(r.data)) }

func (r *memReadable) NewReadHandle(_ context.Context) objstorage.ReadHandle {
	return &memReadHandle{r: r}
}

type memReadHandle struct {
	r *memReadable
}

func (h *memReadHandle) ReadAt(_ context.Context, p []byte, off int64) error {
	return h.r.ReadAt(context.Background(), p, off)
}

func (h *memReadHandle) Close() error                                      { return nil }
func (h *memReadHandle) SetupForCompaction()                               {}
func (h *memReadHandle) RecordCacheHit(_ context.Context, _, _ int64)      {}
func (h *memReadHandle) MaxReadahead()                                     {}

// fileReadable adapts an sstable.ReadableFile into an objstorage.Readable.
type fileReadable struct {
	file sstable.ReadableFile
	size int64
}

var _ objstorage.Readable = (*fileReadable)(nil)

func newFileReadable(file sstable.ReadableFile) (*fileReadable, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	return &fileReadable{file: file, size: info.Size()}, nil
}

func (r *fileReadable) ReadAt(_ context.Context, p []byte, off int64) error {
	_, err := r.file.ReadAt(p, off)
	return err
}

func (r *fileReadable) Close() error { return r.file.Close() }

func (r *fileReadable) Size() int64 { return r.size }

func (r *fileReadable) NewReadHandle(_ context.Context) objstorage.ReadHandle {
	return &fileReadHandle{r: r}
}

type fileReadHandle struct {
	r *fileReadable
}

func (h *fileReadHandle) ReadAt(_ context.Context, p []byte, off int64) error {
	return h.r.ReadAt(context.Background(), p, off)
}

func (h *fileReadHandle) Close() error                                      { return nil }
func (h *fileReadHandle) SetupForCompaction()                               {}
func (h *fileReadHandle) RecordCacheHit(_ context.Context, _, _ int64)      {}
func (h *fileReadHandle) MaxReadahead()                                     {}

// writableAdapter wraps an io.Writer into an objstorage.Writable.
type writableAdapter struct {
	w io.Writer
}

var _ objstorage.Writable = (*writableAdapter)(nil)

func newWritableAdapter(w io.Writer) *writableAdapter {
	return &writableAdapter{w: w}
}

func (w *writableAdapter) Write(p []byte) error {
	_, err := w.w.Write(p)
	return err
}

func (w *writableAdapter) Finish() error { return nil }
func (w *writableAdapter) Abort()        {}
