// Copyright 2021 The Cockroach Authors.
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

package security_test

import (
	"context"
	"crypto/x509"
	"testing"
	"time"

	"github.com/semistrict/ratel/pkg/security"
	"github.com/semistrict/ratel/pkg/util/leaktest"
	"github.com/stretchr/testify/require"
)

// TestDummyCreateCACertAndKey is a placeholder for actual testing functions
// TODO(aaron-crl): [tests] write unit tests
func TestDummyCreateCACertAndKey(t *testing.T) {
	defer leaktest.AfterTest(t)()
	_, _, err := security.CreateCACertAndKey(context.Background(), nil, /* loggerFn */
		time.Hour, "test CA cert generation")
	if err != nil {
		t.Fatalf("expected err=nil, got: %s", err)
	}
}

// TestDummyCreateServiceCertAndKey is a placeholder for actual testing functions
// TODO(aaron-crl): [tests] write unit tests
func TestDummyCreateServiceCertAndKey(t *testing.T) {
	defer leaktest.AfterTest(t)()
	caCert, caKey, err := security.CreateCACertAndKey(context.Background(), nil, /* loggerFn */
		time.Hour, "test CA cert generation")
	if err != nil {
		t.Fatalf("expected err=nil, got: %s", err)
	}

	_, _, err = security.CreateServiceCertAndKey(
		context.Background(), nil, /* loggerFn */
		time.Minute,
		"dummy-common-name",
		[]string{"localhost", "127.0.0.1"},
		caCert,
		caKey,
		false, /* serviceCertIsAlsoValidAsClient */
	)
	if err != nil {
		t.Fatalf("expected err=nil, got: %s", err)
	}
}

// TestServiceCertContainsAllSANs verifies that all hostnames passed to
// CreateServiceCertAndKey appear in the generated certificate's SANs.
// This catches the bug where the loop overwrote DNSNames/IPAddresses
// on each iteration instead of appending.
func TestServiceCertContainsAllSANs(t *testing.T) {
	defer leaktest.AfterTest(t)()

	caCert, caKey, err := security.CreateCACertAndKey(
		context.Background(), nil, time.Hour, "test CA")
	require.NoError(t, err)

	hostnames := []string{
		"abc123.vm.my-app.internal",
		"localhost",
		"192.168.1.1",
		"fd00::1",
	}

	certPEM, _, err := security.CreateServiceCertAndKey(
		context.Background(), nil,
		time.Minute,
		"node",
		hostnames,
		caCert, caKey,
		true,
	)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(certPEM.Bytes)
	require.NoError(t, err)

	// Verify DNS names.
	require.Contains(t, cert.DNSNames, "abc123.vm.my-app.internal",
		"cert missing .internal DNS name; DNSNames=%v", cert.DNSNames)
	require.Contains(t, cert.DNSNames, "localhost",
		"cert missing localhost; DNSNames=%v", cert.DNSNames)

	// Verify IP addresses.
	var ipStrings []string
	for _, ip := range cert.IPAddresses {
		ipStrings = append(ipStrings, ip.String())
	}
	require.Contains(t, ipStrings, "192.168.1.1",
		"cert missing IPv4; IPs=%v", ipStrings)
	require.Contains(t, ipStrings, "fd00::1",
		"cert missing IPv6; IPs=%v", ipStrings)
}

// TestServiceCertSingleHostname verifies the simple case works too.
func TestServiceCertSingleHostname(t *testing.T) {
	defer leaktest.AfterTest(t)()

	caCert, caKey, err := security.CreateCACertAndKey(
		context.Background(), nil, time.Hour, "test CA")
	require.NoError(t, err)

	certPEM, _, err := security.CreateServiceCertAndKey(
		context.Background(), nil,
		time.Minute, "node",
		[]string{"myhost.example.com"},
		caCert, caKey, false,
	)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(certPEM.Bytes)
	require.NoError(t, err)

	require.Equal(t, []string{"myhost.example.com"}, cert.DNSNames)
	require.Empty(t, cert.IPAddresses)
}
