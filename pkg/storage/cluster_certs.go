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
	"encoding/pem"
	"os"
	"path/filepath"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/cockroachdb/pebble/objstorage/remote"
	"github.com/semistrict/ratel/pkg/security"
)

const (
	// Cert object names in the certs/ storage.
	caCertName     = "ca.crt"
	caKeyName      = "ca.key"
	clientRootCert = "client.root.crt"
	clientRootKey  = "client.root.key"
	certLifetime   = 10 * 365 * 24 * time.Hour // 10 years
)

// GenerateAndUploadCACerts generates the CA and root client certificates and
// uploads them to the certs/ storage. If passphrase is non-nil, the CA key is
// encrypted before upload. Node certificates are NOT generated here — each node
// generates its own via GenerateNodeCert after downloading the CA.
func GenerateAndUploadCACerts(ctx context.Context, store remote.Storage, passphrase []byte) error {
	caCertPEM, caKeyPEM, err := security.CreateCACertAndKey(ctx, nil, certLifetime, "Cockroach CA")
	if err != nil {
		return errors.Wrap(err, "generating CA cert")
	}

	clientCertPEM, clientKeyPEM, err := security.CreateServiceCertAndKey(
		ctx, nil, certLifetime, security.RootUser,
		nil,
		caCertPEM, caKeyPEM,
		true,
	)
	if err != nil {
		return errors.Wrap(err, "generating client root cert")
	}

	// Optionally encrypt the CA key.
	caKeyData := pem.EncodeToMemory(caKeyPEM)
	if len(passphrase) > 0 {
		caKeyData, err = EncryptWithPassphrase(caKeyData, passphrase)
		if err != nil {
			return errors.Wrap(err, "encrypting CA key")
		}
	}

	uploads := []struct {
		name string
		data []byte
	}{
		{caCertName, pem.EncodeToMemory(caCertPEM)},
		{caKeyName, caKeyData},
		{clientRootCert, pem.EncodeToMemory(clientCertPEM)},
		{clientRootKey, pem.EncodeToMemory(clientKeyPEM)},
	}
	for _, u := range uploads {
		if err := WriteObject(store, u.name, u.data); err != nil {
			return errors.Wrapf(err, "uploading %s", u.name)
		}
	}
	return nil
}

// DownloadCACerts downloads the CA cert and key from the certs/ storage to a
// local directory. If the CA key is encrypted, passphrase must be provided to
// decrypt it.
func DownloadCACerts(ctx context.Context, store remote.Storage, localDir string, passphrase []byte) error {
	if err := os.MkdirAll(localDir, 0700); err != nil {
		return errors.Wrapf(err, "creating certs dir %s", localDir)
	}
	files := []struct {
		name string
		mode os.FileMode
	}{
		{caCertName, 0644},
		{caKeyName, 0600},
		{clientRootCert, 0644},
		{clientRootKey, 0600},
	}
	for _, f := range files {
		data, err := ReadObject(ctx, store, f.name)
		if err != nil {
			return errors.Wrapf(err, "downloading %s", f.name)
		}
		// Decrypt the CA key if it's encrypted.
		if f.name == caKeyName && IsEncrypted(data) {
			if len(passphrase) == 0 {
				return errors.New("CA key is encrypted but no passphrase provided")
			}
			data, err = DecryptWithPassphrase(data, passphrase)
			if err != nil {
				return errors.Wrap(err, "decrypting CA key")
			}
		}
		path := filepath.Join(localDir, f.name)
		if err := os.WriteFile(path, data, f.mode); err != nil {
			return errors.Wrapf(err, "writing %s", path)
		}
	}
	return nil
}

// GenerateNodeCert generates a node certificate signed by the CA in localDir,
// with the given hostnames as SANs. The CA cert and key must already exist in
// localDir (downloaded via DownloadCACerts). The node cert and key are written
// to localDir as node.crt and node.key.
func GenerateNodeCert(localDir string, hostnames []string) error {
	caCertPath := filepath.Join(localDir, caCertName)
	caKeyPath := filepath.Join(localDir, caKeyName)

	caCertData, err := os.ReadFile(caCertPath)
	if err != nil {
		return errors.Wrap(err, "reading CA cert")
	}
	caKeyData, err := os.ReadFile(caKeyPath)
	if err != nil {
		return errors.Wrap(err, "reading CA key")
	}

	caCertBlock, _ := pem.Decode(caCertData)
	caKeyBlock, _ := pem.Decode(caKeyData)
	if caCertBlock == nil || caKeyBlock == nil {
		return errors.New("failed to decode CA PEM")
	}

	nodeCertPEM, nodeKeyPEM, err := security.CreateServiceCertAndKey(
		context.Background(), nil, certLifetime, security.NodeUser,
		hostnames,
		caCertBlock, caKeyBlock,
		true,
	)
	if err != nil {
		return errors.Wrap(err, "generating node cert")
	}

	if err := os.WriteFile(filepath.Join(localDir, "node.crt"), pem.EncodeToMemory(nodeCertPEM), 0644); err != nil {
		return errors.Wrap(err, "writing node cert")
	}
	if err := os.WriteFile(filepath.Join(localDir, "node.key"), pem.EncodeToMemory(nodeKeyPEM), 0600); err != nil {
		return errors.Wrap(err, "writing node key")
	}
	return nil
}

// DownloadClientCerts downloads only the client certificates needed for SQL
// connections (ca.crt, client.root.crt, client.root.key) to a local directory.
func DownloadClientCerts(ctx context.Context, store remote.Storage, localDir string) error {
	if err := os.MkdirAll(localDir, 0700); err != nil {
		return errors.Wrapf(err, "creating certs dir %s", localDir)
	}
	files := []struct {
		name string
		mode os.FileMode
	}{
		{caCertName, 0644},
		{clientRootCert, 0644},
		{clientRootKey, 0600},
	}
	for _, f := range files {
		data, err := ReadObject(ctx, store, f.name)
		if err != nil {
			return errors.Wrapf(err, "downloading %s", f.name)
		}
		path := filepath.Join(localDir, f.name)
		if err := os.WriteFile(path, data, f.mode); err != nil {
			return errors.Wrapf(err, "writing %s", path)
		}
	}
	return nil
}

// CertsExist checks whether the CA certificate has been uploaded to the
// certs/ storage.
func CertsExist(ctx context.Context, store remote.Storage) (bool, error) {
	_, _, err := store.ReadObject(ctx, caCertName)
	if err != nil {
		if store.IsNotExistError(err) {
			return false, nil
		}
		return false, errors.Wrap(err, "checking for CA cert")
	}
	return true, nil
}
