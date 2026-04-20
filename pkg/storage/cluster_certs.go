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
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/pem"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/cockroachdb/cockroach/pkg/security"
	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/errors"
	"github.com/cockroachdb/errors/oserror"
	"github.com/cockroachdb/pebble/objstorage/remote"
	"golang.org/x/crypto/scrypt"
)

const (
	caCertName     = "ca.crt"
	caKeyName      = "ca.key"
	nodeCertName   = "node.crt"
	nodeKeyName    = "node.key"
	clientRootCert = "client.root.crt"
	clientRootKey  = "client.root.key"
	certLifetime   = 10 * 365 * 24 * time.Hour
)

// Ratel uses a single shared cert set (CA, node, root client) for every node
// in the cluster. TLS is only wire-encryption — the client connects with
// `sslmode=verify-ca`, so hostname SANs are not validated. Keeping one cert
// set sidesteps all the hostname-discovery problems of per-node certs in
// dynamic environments (containers, serverless, etc.).

// encBundleMagic tags a blob encrypted with encryptBlob so we can tell
// ciphertext from plaintext PEM at download time.
var encBundleMagic = []byte("RATELAES")

// scrypt parameters tuned for interactive CLI use (~100ms on modern hardware).
const (
	scryptN      = 1 << 15 // 32768
	scryptR      = 8
	scryptP      = 1
	scryptKeyLen = 32
	saltLen      = 16
	nonceLen     = 12
)

// encryptBlob encrypts plaintext with AES-256-GCM, deriving the key from
// passphrase via scrypt. Layout: magic(8) | salt(16) | nonce(12) | ciphertext+tag.
func encryptBlob(plaintext, passphrase []byte) ([]byte, error) {
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, errors.Wrap(err, "generating salt")
	}
	key, err := scrypt.Key(passphrase, salt, scryptN, scryptR, scryptP, scryptKeyLen)
	if err != nil {
		return nil, errors.Wrap(err, "deriving key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.Wrap(err, "creating cipher")
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errors.Wrap(err, "creating GCM")
	}
	nonce := make([]byte, nonceLen)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, errors.Wrap(err, "generating nonce")
	}
	out := make([]byte, 0, len(encBundleMagic)+saltLen+nonceLen+len(plaintext)+gcm.Overhead())
	out = append(out, encBundleMagic...)
	out = append(out, salt...)
	out = append(out, nonce...)
	out = gcm.Seal(out, nonce, plaintext, nil)
	return out, nil
}

// decryptBlob reverses encryptBlob. If the input does not start with the
// magic bytes, it is returned unchanged (backward-compat with unencrypted
// key uploads).
func decryptBlob(blob, passphrase []byte) ([]byte, error) {
	if !bytes.HasPrefix(blob, encBundleMagic) {
		return blob, nil
	}
	body := blob[len(encBundleMagic):]
	if len(body) < saltLen+nonceLen {
		return nil, errors.New("encrypted blob truncated")
	}
	salt := body[:saltLen]
	nonce := body[saltLen : saltLen+nonceLen]
	ciphertext := body[saltLen+nonceLen:]
	if len(passphrase) == 0 {
		return nil, errors.New("key is passphrase-encrypted but no passphrase was provided")
	}
	key, err := scrypt.Key(passphrase, salt, scryptN, scryptR, scryptP, scryptKeyLen)
	if err != nil {
		return nil, errors.Wrap(err, "deriving key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.Wrap(err, "creating cipher")
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errors.Wrap(err, "creating GCM")
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, errors.Wrap(err, "decrypting (wrong passphrase?)")
	}
	return plaintext, nil
}

type certUpload struct {
	name      string
	data      []byte
	encrypted bool // true for private keys when passphrase is set
}

// GenerateAndUploadCerts generates a CA, node, and root client TLS cert set
// and uploads them to the given certs/ remote.Storage. If passphrase is
// non-empty, the three private keys (ca.key, node.key, client.root.key) are
// encrypted at rest with AES-256-GCM + scrypt.
func GenerateAndUploadCerts(ctx context.Context, store remote.Storage, passphrase []byte) error {
	caCertPEM, caKeyPEM, err := security.CreateCACertAndKey(ctx, nil, certLifetime, "Ratel CA")
	if err != nil {
		return errors.Wrap(err, "generating CA cert")
	}

	nodeCertPEM, nodeKeyPEM, err := security.CreateServiceCertAndKey(
		ctx, nil, certLifetime, username.NodeUser,
		nil, // no hostname SANs — clients use sslmode=verify-ca
		caCertPEM, caKeyPEM,
		true, // also valid as client
	)
	if err != nil {
		return errors.Wrap(err, "generating node cert")
	}

	clientCertPEM, clientKeyPEM, err := security.CreateServiceCertAndKey(
		ctx, nil, certLifetime, username.RootUser,
		nil,
		caCertPEM, caKeyPEM,
		true,
	)
	if err != nil {
		return errors.Wrap(err, "generating client root cert")
	}

	uploads := []certUpload{
		{name: caCertName, data: pem.EncodeToMemory(caCertPEM)},
		{name: caKeyName, data: pem.EncodeToMemory(caKeyPEM), encrypted: true},
		{name: nodeCertName, data: pem.EncodeToMemory(nodeCertPEM)},
		{name: nodeKeyName, data: pem.EncodeToMemory(nodeKeyPEM), encrypted: true},
		{name: clientRootCert, data: pem.EncodeToMemory(clientCertPEM)},
		{name: clientRootKey, data: pem.EncodeToMemory(clientKeyPEM), encrypted: true},
	}
	for _, u := range uploads {
		payload := u.data
		if u.encrypted && len(passphrase) > 0 {
			payload, err = encryptBlob(u.data, passphrase)
			if err != nil {
				return errors.Wrapf(err, "encrypting %s", u.name)
			}
		}
		if err := WriteObject(store, u.name, payload); err != nil {
			return errors.Wrapf(err, "uploading %s", u.name)
		}
	}
	return nil
}

// DownloadCerts downloads the full cert set (CA, node, root client) from the
// given certs/ storage into a local directory. Private keys are decrypted
// with passphrase if they were encrypted at upload time.
func DownloadCerts(
	ctx context.Context, store remote.Storage, localDir string, passphrase []byte,
) error {
	return downloadCertFiles(ctx, store, localDir, passphrase, []certFile{
		{caCertName, 0644, false},
		{caKeyName, 0600, true},
		{nodeCertName, 0644, false},
		{nodeKeyName, 0600, true},
		{clientRootCert, 0644, false},
		{clientRootKey, 0600, true},
	})
}

// DownloadClientCerts downloads only the client cert set (CA, root client
// cert/key) for SQL shell connections.
func DownloadClientCerts(
	ctx context.Context, store remote.Storage, localDir string, passphrase []byte,
) error {
	return downloadCertFiles(ctx, store, localDir, passphrase, []certFile{
		{caCertName, 0644, false},
		{clientRootCert, 0644, false},
		{clientRootKey, 0600, true},
	})
}

type certFile struct {
	name      string
	mode      os.FileMode
	encrypted bool
}

func downloadCertFiles(
	ctx context.Context,
	store remote.Storage,
	localDir string,
	passphrase []byte,
	files []certFile,
) error {
	if err := os.MkdirAll(localDir, 0700); err != nil {
		return errors.Wrapf(err, "creating certs dir %s", localDir)
	}
	for _, f := range files {
		data, err := ReadObject(ctx, store, f.name)
		if err != nil {
			return errors.Wrapf(err, "downloading %s", f.name)
		}
		if f.encrypted {
			data, err = decryptBlob(data, passphrase)
			if err != nil {
				return errors.Wrapf(err, "decrypting %s", f.name)
			}
		}
		path := filepath.Join(localDir, f.name)
		if err := os.WriteFile(path, data, f.mode); err != nil {
			return errors.Wrapf(err, "writing %s", path)
		}
	}
	return nil
}

// CertsExist returns true if the CA cert has already been uploaded.
func CertsExist(ctx context.Context, store remote.Storage) (bool, error) {
	_, err := store.Size(caCertName)
	if err != nil {
		if store.IsNotExistError(err) || oserror.IsNotExist(err) {
			return false, nil
		}
		return false, errors.Wrap(err, "checking for CA cert")
	}
	return true, nil
}
