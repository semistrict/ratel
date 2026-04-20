// Copyright 2026 The Ratel Authors.
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
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/cockroachdb/pebble/objstorage/remote"
	"github.com/stretchr/testify/require"
)

func TestGenerateAndUploadCertsPlaintext(t *testing.T) {
	ctx := t.Context()
	store := remote.NewInMem()
	defer func() { _ = store.Close() }()

	exists, err := CertsExist(ctx, store)
	require.NoError(t, err)
	require.False(t, exists)

	require.NoError(t, GenerateAndUploadCerts(ctx, store, nil))

	exists, err = CertsExist(ctx, store)
	require.NoError(t, err)
	require.True(t, exists)

	for _, name := range []string{
		"ca.crt", "ca.key",
		"node.crt", "node.key",
		"client.root.crt", "client.root.key",
	} {
		size, err := store.Size(name)
		require.NoError(t, err, "cert %s should exist", name)
		require.Greater(t, size, int64(0), "cert %s should be non-empty", name)
	}

	// With no passphrase, ca.key is stored as plaintext PEM.
	raw, err := ReadObject(ctx, store, "ca.key")
	require.NoError(t, err)
	require.False(t, bytes.HasPrefix(raw, encBundleMagic),
		"ca.key should be plaintext when no passphrase is set")
	block, _ := pem.Decode(raw)
	require.NotNil(t, block, "ca.key should be valid PEM")
	require.Equal(t, "RSA PRIVATE KEY", block.Type)
}

func TestGenerateAndUploadCertsEncrypted(t *testing.T) {
	ctx := t.Context()
	store := remote.NewInMem()
	defer func() { _ = store.Close() }()

	passphrase := []byte("correct horse battery staple")
	require.NoError(t, GenerateAndUploadCerts(ctx, store, passphrase))

	// Private keys are encrypted at rest.
	for _, name := range []string{"ca.key", "node.key", "client.root.key"} {
		raw, err := ReadObject(ctx, store, name)
		require.NoError(t, err)
		require.True(t, bytes.HasPrefix(raw, encBundleMagic),
			"%s should be encrypted with passphrase", name)
	}

	// Public certs are plaintext.
	for _, name := range []string{"ca.crt", "node.crt", "client.root.crt"} {
		raw, err := ReadObject(ctx, store, name)
		require.NoError(t, err)
		require.False(t, bytes.HasPrefix(raw, encBundleMagic),
			"%s should not be encrypted", name)
	}

	// Round-trip download decrypts correctly.
	localDir := t.TempDir()
	require.NoError(t, DownloadCerts(ctx, store, localDir, passphrase))
	keyPEM, err := os.ReadFile(filepath.Join(localDir, "ca.key"))
	require.NoError(t, err)
	block, _ := pem.Decode(keyPEM)
	require.NotNil(t, block)
	_, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	require.NoError(t, err, "decrypted ca.key should be a valid RSA key")
}

func TestDownloadCertsWrongPassphrase(t *testing.T) {
	ctx := t.Context()
	store := remote.NewInMem()
	defer func() { _ = store.Close() }()

	require.NoError(t, GenerateAndUploadCerts(ctx, store, []byte("right")))

	localDir := t.TempDir()
	err := DownloadCerts(ctx, store, localDir, []byte("wrong"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "decrypting")
}

func TestDownloadCertsMissingPassphrase(t *testing.T) {
	ctx := t.Context()
	store := remote.NewInMem()
	defer func() { _ = store.Close() }()

	require.NoError(t, GenerateAndUploadCerts(ctx, store, []byte("secret")))

	localDir := t.TempDir()
	err := DownloadCerts(ctx, store, localDir, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "passphrase")
}

func TestDownloadCerts(t *testing.T) {
	ctx := t.Context()
	store := remote.NewInMem()
	defer func() { _ = store.Close() }()

	require.NoError(t, GenerateAndUploadCerts(ctx, store, nil))

	localDir := t.TempDir()
	require.NoError(t, DownloadCerts(ctx, store, localDir, nil))

	for _, name := range []string{
		"ca.crt", "ca.key",
		"node.crt", "node.key",
		"client.root.crt", "client.root.key",
	} {
		data, err := os.ReadFile(filepath.Join(localDir, name))
		require.NoError(t, err, "should be able to read %s", name)
		require.NotEmpty(t, data, "%s should be non-empty", name)
	}
}

func TestDownloadClientCerts(t *testing.T) {
	ctx := t.Context()
	store := remote.NewInMem()
	defer func() { _ = store.Close() }()

	require.NoError(t, GenerateAndUploadCerts(ctx, store, nil))

	localDir := t.TempDir()
	require.NoError(t, DownloadClientCerts(ctx, store, localDir, nil))

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
