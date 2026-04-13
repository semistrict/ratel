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

package server

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cockroachdb/errors"
	"github.com/klauspost/compress/zstd"
)

const embeddedWorkerdPath = "workerd_bin/workerd.zst"

// extractEmbeddedWorkerd extracts the embedded workerd binary to a
// content-addressed cache directory. Returns the path to the executable.
// Returns an error if no binary is embedded.
func extractEmbeddedWorkerd() (string, error) {
	compressed, err := embeddedWorkerdFS.ReadFile(embeddedWorkerdPath)
	if err != nil {
		return "", errors.Wrap(err, "no embedded workerd binary")
	}

	// Content-addressed: use first 16 bytes of SHA-256 of the compressed data.
	hash := sha256.Sum256(compressed)
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", errors.Wrap(err, "finding cache directory")
	}
	dir := filepath.Join(cacheDir, "ratel")
	cachePath := filepath.Join(dir, fmt.Sprintf("workerd-%x", hash[:16]))

	// Already extracted and executable?
	if info, statErr := os.Stat(cachePath); statErr == nil && info.Mode()&0111 != 0 {
		return cachePath, nil
	}

	// Decompress.
	decoder, err := zstd.NewReader(nil)
	if err != nil {
		return "", errors.Wrap(err, "creating zstd decoder")
	}
	defer decoder.Close()

	decompressed, err := decoder.DecodeAll(compressed, nil)
	if err != nil {
		return "", errors.Wrap(err, "decompressing embedded workerd binary")
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", errors.Wrap(err, "creating cache directory")
	}

	// Write to a temp file then atomic rename.
	tmp, err := os.CreateTemp(dir, "workerd-extract-*")
	if err != nil {
		return "", errors.Wrap(err, "creating temp file for extraction")
	}
	tmpPath := tmp.Name()

	if _, writeErr := tmp.Write(decompressed); writeErr != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return "", errors.Wrap(writeErr, "writing extracted workerd binary")
	}
	if err := tmp.Chmod(0755); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return "", errors.Wrap(err, "setting executable permission")
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return "", errors.Wrap(err, "closing extracted workerd binary")
	}
	if err := os.Rename(tmpPath, cachePath); err != nil {
		os.Remove(tmpPath)
		return "", errors.Wrap(err, "renaming extracted workerd binary to cache path")
	}

	return cachePath, nil
}
