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

package workers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/cockroachdb/cockroach/pkg/kv"
	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/cockroach/pkg/sql/isql"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/sessiondata"
	"github.com/cockroachdb/errors"
)

const maxWorkerScriptSize = 10 << 20 // 10 MiB
const maxWorkerAssetSize = 10 << 20  // 10 MiB per asset

type DeployRequest struct {
	Script     []byte
	CompatDate string
	Bindings   string
	Assets     []DeployAsset
}

type DeployAsset struct {
	Path        string
	ContentType string
	Content     []byte
	ETag        string
	Size        int64
}

type workerDeployMetadata struct {
	CompatDate string                `json:"compat_date"`
	Bindings   json.RawMessage       `json:"bindings"`
	Assets     []DeployAssetMetadata `json:"assets"`
}

type DeployAssetMetadata struct {
	Path        string `json:"path"`
	ContentType string `json:"content_type,omitempty"`
	ETag        string `json:"etag,omitempty"`
	Size        int64  `json:"size,omitempty"`
}

func ParseDeployRequest(r *http.Request) (DeployRequest, error) {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		return parseMultipartWorkerDeployRequest(r)
	}

	compatDate := r.Header.Get("X-Compat-Date")
	if compatDate == "" {
		compatDate = "2024-01-01"
	}
	var bindings string
	if bindingsStr := r.Header.Get("X-Bindings"); bindingsStr != "" {
		if !json.Valid([]byte(bindingsStr)) {
			return DeployRequest{}, errors.New("X-Bindings header must be valid JSON")
		}
		bindings = bindingsStr
	}

	script, err := readLimited(r.Body, maxWorkerScriptSize, "worker script")
	if err != nil {
		return DeployRequest{}, err
	}
	return DeployRequest{
		Script:     script,
		CompatDate: compatDate,
		Bindings:   bindings,
	}, nil
}

func parseMultipartWorkerDeployRequest(r *http.Request) (DeployRequest, error) {
	mr, err := r.MultipartReader()
	if err != nil {
		return DeployRequest{}, err
	}
	var script []byte
	var metadata workerDeployMetadata
	var metadataSeen bool
	var assetContents [][]byte

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return DeployRequest{}, err
		}

		switch part.FormName() {
		case "script":
			if script != nil {
				return DeployRequest{}, errors.New("multipart deploy contains multiple script parts")
			}
			script, err = readLimited(part, maxWorkerScriptSize, "worker script")
			if err != nil {
				return DeployRequest{}, err
			}
		case "metadata":
			if metadataSeen {
				return DeployRequest{}, errors.New("multipart deploy contains multiple metadata parts")
			}
			data, err := readLimited(part, maxWorkerAssetSize, "worker metadata")
			if err != nil {
				return DeployRequest{}, err
			}
			if err := json.Unmarshal(data, &metadata); err != nil {
				return DeployRequest{}, errors.Wrap(err, "parsing worker metadata")
			}
			if len(metadata.Bindings) > 0 && !json.Valid(metadata.Bindings) {
				return DeployRequest{}, errors.New("metadata bindings must be valid JSON")
			}
			metadataSeen = true
		case "asset":
			data, err := readLimitedAllowEmpty(part, maxWorkerAssetSize, "worker asset")
			if err != nil {
				return DeployRequest{}, err
			}
			assetContents = append(assetContents, data)
		default:
			return DeployRequest{}, errors.Newf("unexpected multipart field %q", part.FormName())
		}
	}

	if !metadataSeen {
		return DeployRequest{}, errors.New("multipart deploy missing metadata part")
	}
	if script == nil {
		return DeployRequest{}, errors.New("multipart deploy missing script part")
	}
	if len(assetContents) != len(metadata.Assets) {
		return DeployRequest{}, errors.Newf(
			"multipart deploy has %d asset parts but %d metadata assets",
			len(assetContents), len(metadata.Assets),
		)
	}
	if metadata.CompatDate == "" {
		metadata.CompatDate = "2024-01-01"
	}

	assets := make([]DeployAsset, 0, len(metadata.Assets))
	seenPaths := make(map[string]struct{}, len(metadata.Assets))
	for i, meta := range metadata.Assets {
		path, err := normalizeWorkerAssetPath(meta.Path)
		if err != nil {
			return DeployRequest{}, err
		}
		if _, ok := seenPaths[path]; ok {
			return DeployRequest{}, errors.Newf("duplicate asset path %q", path)
		}
		seenPaths[path] = struct{}{}
		content := assetContents[i]
		sum := sha256.Sum256(content)
		etag := hex.EncodeToString(sum[:])
		if meta.ETag != "" && meta.ETag != etag {
			return DeployRequest{}, errors.Newf("asset %s etag mismatch", path)
		}
		if meta.Size != 0 && meta.Size != int64(len(content)) {
			return DeployRequest{}, errors.Newf("asset %s size mismatch", path)
		}
		assets = append(assets, DeployAsset{
			Path:        path,
			ContentType: meta.ContentType,
			Content:     content,
			ETag:        etag,
			Size:        int64(len(content)),
		})
	}

	bindings := ""
	if len(metadata.Bindings) > 0 && string(metadata.Bindings) != "null" {
		bindings = string(metadata.Bindings)
	}
	return DeployRequest{
		Script:     script,
		CompatDate: metadata.CompatDate,
		Bindings:   bindings,
		Assets:     assets,
	}, nil
}

func readLimited(r io.Reader, limit int64, name string) ([]byte, error) {
	data, err := readLimitedAllowEmpty(r, limit, name)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, errors.Newf("empty %s", name)
	}
	return data, nil
}

func readLimitedAllowEmpty(r io.Reader, limit int64, name string) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.Newf("%s exceeds %d byte limit", name, limit)
	}
	return data, nil
}

func InsertDeploy(
	ctx context.Context, db *kv.DB, ie isql.Executor, name string, deploy DeployRequest,
) (int64, error) {
	var nextVersion int64
	var workerVersionID int64
	override := sessiondata.InternalExecutorOverride{
		User:     username.RootUserName(),
		Database: "system",
	}

	err := db.Txn(ctx, func(ctx context.Context, txn *kv.Txn) error {
		var bindingsArg interface{}
		if deploy.Bindings != "" {
			bindingsArg = deploy.Bindings
		}

		row, err := ie.QueryRowEx(
			ctx,
			"deploy-worker-atomic",
			txn,
			override,
			`INSERT INTO system.worker_versions (worker_name, version, script, compat_date, bindings)
			 VALUES ($1, COALESCE((SELECT max(version) FROM system.worker_versions WHERE worker_name = $1), 0) + 1, $2, $3, $4)
			 RETURNING id, version`,
			name,
			deploy.Script,
			deploy.CompatDate,
			bindingsArg,
		)
		if err != nil {
			return err
		}
		if row != nil && row[0] != nil && row[1] != nil {
			if v, ok := row[0].(*tree.DInt); ok {
				workerVersionID = int64(*v)
			}
			if v, ok := row[1].(*tree.DInt); ok {
				nextVersion = int64(*v)
			}
		}
		for _, asset := range deploy.Assets {
			if _, err := ie.ExecEx(
				ctx,
				"deploy-worker-asset",
				txn,
				override,
				`INSERT INTO system.worker_assets
				 (etag, content, size)
				 VALUES ($1, $2, $3)
				 ON CONFLICT (etag) DO NOTHING`,
				asset.ETag,
				asset.Content,
				asset.Size,
			); err != nil {
				return err
			}
			if _, err := ie.ExecEx(
				ctx,
				"deploy-worker-version-asset",
				txn,
				override,
				`INSERT INTO system.worker_version_assets
				 (worker_version_id, path, asset_etag, content_type)
				 VALUES ($1, $2, $3, $4)`,
				workerVersionID,
				asset.Path,
				asset.ETag,
				asset.ContentType,
			); err != nil {
				return err
			}
		}
		return nil
	})
	return nextVersion, err
}

func normalizeWorkerAssetPath(path string) (string, error) {
	path = strings.TrimSpace(strings.ReplaceAll(path, "\\", "/"))
	path = "/" + strings.TrimPrefix(path, "/")
	if path == "/" || strings.Contains(path, "/../") || strings.HasSuffix(path, "/..") ||
		strings.Contains(path, "/./") || strings.HasSuffix(path, "/.") {
		return "", errors.Newf("invalid asset path %q", path)
	}
	return path, nil
}
