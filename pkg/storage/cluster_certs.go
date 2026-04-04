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

	"github.com/cockroachdb/cockroach/pkg/security"
	"github.com/cockroachdb/errors"
	"github.com/cockroachdb/pebble/objstorage/remote"
)

const (
	// Cert object names in the certs/ storage.
	caCertName       = "ca.crt"
	caKeyName        = "ca.key"
	nodeCertName     = "node.crt"
	nodeKeyName      = "node.key"
	clientRootCert   = "client.root.crt"
	clientRootKey    = "client.root.key"
	certLifetime     = 10 * 365 * 24 * time.Hour // 10 years
	certNodeHostname = "localhost"
)

// GenerateAndUploadCerts generates CA, node, and root client TLS certificates
// and uploads them to the certs/ storage.
func GenerateAndUploadCerts(ctx context.Context, store remote.Storage) error {
	// Generate CA cert and key.
	caCertPEM, caKeyPEM, err := security.CreateCACertAndKey(ctx, nil, certLifetime, "Cockroach CA")
	if err != nil {
		return errors.Wrap(err, "generating CA cert")
	}

	// Generate node cert and key signed by the CA.
	nodeCertPEM, nodeKeyPEM, err := security.CreateServiceCertAndKey(
		ctx, nil, certLifetime, security.NodeUser,
		[]string{certNodeHostname},
		caCertPEM, caKeyPEM,
		true, // serviceCertIsAlsoValidAsClient — node certs are used for both
	)
	if err != nil {
		return errors.Wrap(err, "generating node cert")
	}

	// Generate root client cert and key signed by the CA.
	clientCertPEM, clientKeyPEM, err := security.CreateServiceCertAndKey(
		ctx, nil, certLifetime, security.RootUser,
		nil, // no hostnames for client cert
		caCertPEM, caKeyPEM,
		true, // client cert
	)
	if err != nil {
		return errors.Wrap(err, "generating client root cert")
	}

	// Upload all certs to storage.
	uploads := []struct {
		name string
		data []byte
	}{
		{caCertName, pem.EncodeToMemory(caCertPEM)},
		{caKeyName, pem.EncodeToMemory(caKeyPEM)},
		{nodeCertName, pem.EncodeToMemory(nodeCertPEM)},
		{nodeKeyName, pem.EncodeToMemory(nodeKeyPEM)},
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

// DownloadCerts downloads all TLS certificates from the certs/ storage to a
// local directory, creating the directory if needed.
func DownloadCerts(ctx context.Context, store remote.Storage, localDir string) error {
	if err := os.MkdirAll(localDir, 0700); err != nil {
		return errors.Wrapf(err, "creating certs dir %s", localDir)
	}

	files := []struct {
		name string
		mode os.FileMode
	}{
		{caCertName, 0644},
		{caKeyName, 0600},
		{nodeCertName, 0644},
		{nodeKeyName, 0600},
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

// CertsExist checks whether certificates have already been uploaded to the
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
