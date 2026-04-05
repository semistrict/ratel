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
	"os"
	"path/filepath"
	"testing"

	"github.com/cockroachdb/pebble/objstorage/remote"
	"github.com/stretchr/testify/require"
)

func TestGenerateAndUploadCACerts(t *testing.T) {
	ctx := t.Context()
	store := remote.NewInMem()
	defer func() { _ = store.Close() }()

	exists, err := CertsExist(ctx, store)
	require.NoError(t, err)
	require.False(t, exists)

	require.NoError(t, GenerateAndUploadCACerts(ctx, store, nil))

	exists, err = CertsExist(ctx, store)
	require.NoError(t, err)
	require.True(t, exists)

	// CA and client certs are present. Node certs are NOT.
	for _, name := range []string{"ca.crt", "ca.key", "client.root.crt", "client.root.key"} {
		_, size, err := store.ReadObject(ctx, name)
		require.NoError(t, err, "cert %s should exist", name)
		require.Greater(t, size, int64(0), "cert %s should be non-empty", name)
	}
	for _, name := range []string{"node.crt", "node.key"} {
		_, _, err := store.ReadObject(ctx, name)
		require.True(t, store.IsNotExistError(err), "node cert %s should not be in S3", name)
	}
}

func TestGenerateAndUploadCACertsWithPassphrase(t *testing.T) {
	ctx := t.Context()
	store := remote.NewInMem()
	defer func() { _ = store.Close() }()

	passphrase := []byte("test-secret-passphrase")
	require.NoError(t, GenerateAndUploadCACerts(ctx, store, passphrase))

	// The CA key should be encrypted in storage.
	caKeyData, err := ReadObject(ctx, store, "ca.key")
	require.NoError(t, err)
	require.True(t, IsEncrypted(caKeyData), "CA key should be encrypted")

	// Download with correct passphrase should work.
	localDir := t.TempDir()
	require.NoError(t, DownloadCACerts(ctx, store, localDir, passphrase))

	// The local CA key should be decrypted PEM.
	caKeyLocal, err := os.ReadFile(filepath.Join(localDir, "ca.key"))
	require.NoError(t, err)
	require.False(t, IsEncrypted(caKeyLocal), "local CA key should be decrypted")
	require.Contains(t, string(caKeyLocal), "PRIVATE KEY")

	// Download with wrong passphrase should fail.
	localDir2 := t.TempDir()
	err = DownloadCACerts(ctx, store, localDir2, []byte("wrong"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "wrong passphrase")

	// Download with no passphrase should fail.
	localDir3 := t.TempDir()
	err = DownloadCACerts(ctx, store, localDir3, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no passphrase")

	// Node cert generation should work with the decrypted CA.
	require.NoError(t, GenerateNodeCert(localDir, []string{"myhost", "localhost"}))
	nodeKey, err := os.ReadFile(filepath.Join(localDir, "node.key"))
	require.NoError(t, err)
	require.Contains(t, string(nodeKey), "PRIVATE KEY")
}

func TestGenerateNodeCert(t *testing.T) {
	ctx := t.Context()
	store := remote.NewInMem()
	defer func() { _ = store.Close() }()

	require.NoError(t, GenerateAndUploadCACerts(ctx, store, nil))

	localDir := t.TempDir()
	require.NoError(t, DownloadCACerts(ctx, store, localDir, nil))
	require.NoError(t, GenerateNodeCert(localDir, []string{"myhost.example.com", "localhost"}))

	for _, name := range []string{"node.crt", "node.key"} {
		data, err := os.ReadFile(filepath.Join(localDir, name))
		require.NoError(t, err, "should be able to read %s", name)
		require.NotEmpty(t, data, "%s should be non-empty", name)
	}
}

func TestDownloadClientCerts(t *testing.T) {
	ctx := t.Context()
	store := remote.NewInMem()
	defer func() { _ = store.Close() }()

	require.NoError(t, GenerateAndUploadCACerts(ctx, store, nil))

	localDir := t.TempDir()
	require.NoError(t, DownloadClientCerts(ctx, store, localDir))

	for _, name := range []string{"ca.crt", "client.root.crt", "client.root.key"} {
		data, err := os.ReadFile(filepath.Join(localDir, name))
		require.NoError(t, err, "should be able to read %s", name)
		require.NotEmpty(t, data, "%s should be non-empty", name)
	}
	for _, name := range []string{"node.crt", "node.key", "ca.key"} {
		_, err := os.ReadFile(filepath.Join(localDir, name))
		require.Error(t, err, "%s should not be present", name)
	}
}
