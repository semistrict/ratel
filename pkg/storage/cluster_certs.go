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
	"context"
	"encoding/pem"
	"os"
	"path/filepath"
	"time"

	"github.com/cockroachdb/cockroach/pkg/security"
	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/errors"
	"github.com/cockroachdb/errors/oserror"
	"github.com/cockroachdb/pebble/objstorage/remote"
)

const (
	caCertName       = "ca.crt"
	caKeyName        = "ca.key"
	nodeCertName     = "node.crt"
	nodeKeyName      = "node.key"
	clientRootCert   = "client.root.crt"
	clientRootKey    = "client.root.key"
	certLifetime     = 10 * 365 * 24 * time.Hour
	certNodeHostname = "localhost"
)

// GenerateAndUploadCerts generates a CA, node, and root client TLS certificate
// set and uploads them to the given certs/ remote.Storage.
func GenerateAndUploadCerts(ctx context.Context, store remote.Storage) error {
	caCertPEM, caKeyPEM, err := security.CreateCACertAndKey(ctx, nil, certLifetime, "Ratel CA")
	if err != nil {
		return errors.Wrap(err, "generating CA cert")
	}

	nodeCertPEM, nodeKeyPEM, err := security.CreateServiceCertAndKey(
		ctx, nil, certLifetime, username.NodeUser,
		[]string{certNodeHostname},
		caCertPEM, caKeyPEM,
		true, // node cert also valid as client
	)
	if err != nil {
		return errors.Wrap(err, "generating node cert")
	}

	clientCertPEM, clientKeyPEM, err := security.CreateServiceCertAndKey(
		ctx, nil, certLifetime, username.RootUser,
		nil, // no hostnames for client cert
		caCertPEM, caKeyPEM,
		true, // client cert
	)
	if err != nil {
		return errors.Wrap(err, "generating client root cert")
	}

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

// DownloadCerts downloads the full cert set (CA, node, root client) from the
// given certs/ storage into a local directory.
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
	return writeCertFiles(ctx, store, localDir, files)
}

// DownloadClientCerts downloads only the client cert set (CA, root client
// cert/key) for SQL shell connections.
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
	return writeCertFiles(ctx, store, localDir, files)
}

func writeCertFiles(
	ctx context.Context,
	store remote.Storage,
	localDir string,
	files []struct {
		name string
		mode os.FileMode
	},
) error {
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
