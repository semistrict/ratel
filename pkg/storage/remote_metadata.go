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
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/cockroachdb/pebble/objstorage/remote"
	"github.com/cockroachdb/pebble/vfs"
)

// manifestBundleName returns the object name for a store's manifest bundle.
// When storeID is 0 (first boot, before the store identity is known), it
// falls back to the legacy name for backwards compatibility.
func manifestBundleName(storeID int32) string {
	if storeID == 0 {
		return "manifest-bundle.zip"
	}
	return fmt.Sprintf("manifest-bundle-%d.zip", storeID)
}

// isManifestBundleFile returns true if the filename is a Pebble metadata file
// that should be included in the manifest bundle. These are the files needed
// to reopen a Pebble DB whose SSTables live on remote storage.
func isManifestBundleFile(name string) bool {
	switch {
	case strings.HasPrefix(name, "MANIFEST-"):
		return true
	case strings.HasPrefix(name, "OPTIONS-"):
		return true
	case strings.HasPrefix(name, "marker."):
		return true
	case strings.HasPrefix(name, "REMOTE-OBJ-CATALOG-"):
		return true
	default:
		return false
	}
}

// UploadManifestBundle reads the Pebble metadata files from dir on the given
// filesystem and uploads them as a zip archive to remote storage under the
// _manifest/ prefix. This captures the MANIFEST, OPTIONS, format-version
// marker, manifest marker, and remote object catalog — everything needed to
// reopen the DB with its remote SSTables.
func UploadManifestBundle(
	ctx context.Context, fs vfs.FS, dir string, store remote.Storage, storeID int32,
) error {
	ls, err := fs.List(dir)
	if err != nil {
		return errors.Wrap(err, "listing pebble dir for manifest bundle")
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	for _, name := range ls {
		if !isManifestBundleFile(name) {
			continue
		}
		path := fs.PathJoin(dir, name)
		data, err := readVFSFile(fs, path)
		if err != nil {
			return errors.Wrapf(err, "reading %s for manifest bundle", name)
		}
		fw, err := zw.Create(name)
		if err != nil {
			return errors.Wrapf(err, "creating zip entry %s", name)
		}
		if _, err := fw.Write(data); err != nil {
			return errors.Wrapf(err, "writing zip entry %s", name)
		}
	}
	if err := zw.Close(); err != nil {
		return errors.Wrap(err, "closing zip writer")
	}

	w, err := store.CreateObject(manifestBundleName(storeID))
	if err != nil {
		return errors.Wrap(err, "creating manifest bundle object")
	}
	if _, err := w.Write(buf.Bytes()); err != nil {
		_ = w.Close()
		return errors.Wrap(err, "writing manifest bundle")
	}
	return errors.Wrap(w.Close(), "closing manifest bundle writer")
}

// DownloadManifestBundle downloads the manifest bundle from remote storage and
// extracts the Pebble metadata files into dir on the given filesystem. The
// directory must already exist.
func DownloadManifestBundle(
	ctx context.Context, fs vfs.FS, dir string, store remote.Storage, storeID int32,
) error {
	reader, size, err := store.ReadObject(ctx, manifestBundleName(storeID))
	if err != nil {
		return errors.Wrap(err, "reading manifest bundle object")
	}
	defer reader.Close()

	data := make([]byte, size)
	if err := reader.ReadAt(ctx, data, 0); err != nil {
		return errors.Wrap(err, "downloading manifest bundle")
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return errors.Wrap(err, "opening zip reader")
	}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return errors.Wrapf(err, "opening zip entry %s", f.Name)
		}
		body, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return errors.Wrapf(err, "reading zip entry %s", f.Name)
		}
		path := fs.PathJoin(dir, f.Name)
		if err := writeVFSFile(fs, path, body); err != nil {
			return errors.Wrapf(err, "writing %s from manifest bundle", f.Name)
		}
	}
	return nil
}

// ManifestBundleExists returns true if a manifest bundle has been uploaded
// to the given remote storage.
func ManifestBundleExists(ctx context.Context, store remote.Storage, storeID int32) (bool, error) {
	_, err := store.Size(manifestBundleName(storeID))
	if err != nil {
		if store.IsNotExistError(err) {
			return false, nil
		}
		return false, errors.Wrap(err, "checking manifest bundle existence")
	}
	return true, nil
}

// readVFSFile reads the entire contents of a file from a vfs.FS.
func readVFSFile(fs vfs.FS, path string) ([]byte, error) {
	f, err := fs.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

// writeVFSFile writes data to a file on a vfs.FS, creating it if necessary.
func writeVFSFile(fs vfs.FS, path string, data []byte) error {
	f, err := fs.Create(path)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(data)
	syncErr := f.Sync()
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}
