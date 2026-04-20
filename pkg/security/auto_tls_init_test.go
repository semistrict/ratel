// Copyright 2021 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package security_test

import (
	"context"
	"crypto/x509"
	"testing"
	"time"

	"github.com/cockroachdb/cockroach/pkg/security"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
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

// TestServiceCertContainsAllSANs verifies that every hostname passed to
// CreateServiceCertAndKey ends up in the generated certificate's SANs. This
// is a regression test for the bug where the hostname loop overwrote
// DNSNames/IPAddresses on each iteration instead of appending.
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

	require.Contains(t, cert.DNSNames, "abc123.vm.my-app.internal",
		"cert missing .internal DNS name; DNSNames=%v", cert.DNSNames)
	require.Contains(t, cert.DNSNames, "localhost",
		"cert missing localhost; DNSNames=%v", cert.DNSNames)

	var ipStrings []string
	for _, ip := range cert.IPAddresses {
		ipStrings = append(ipStrings, ip.String())
	}
	require.Contains(t, ipStrings, "192.168.1.1",
		"cert missing IPv4; IPs=%v", ipStrings)
	require.Contains(t, ipStrings, "fd00::1",
		"cert missing IPv6; IPs=%v", ipStrings)
}

// TestServiceCertSingleHostname verifies the simple single-hostname case
// still works after the SAN fix.
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
}
