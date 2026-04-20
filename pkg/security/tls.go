// Copyright 2014 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package security

import (
	"crypto/tls"
	"crypto/x509"

	"github.com/cockroachdb/errors"
)

// EmbeddedTenantIDs lists the tenants we embed certs for.
// See 'securitytest/test_certs/regenerate.sh'.
var EmbeddedTenantIDs = func() []uint64 { return []uint64{10, 11, 20, 2} }

// newServerTLSConfig creates a server TLSConfig from the supplied byte strings containing
// - the certificate of this node (should be signed by the CA),
// - the private key of this node.
// - the certificate of the cluster CA, used to verify other server certificates
// - the certificate of the client CA, used to verify client certificates
//
// caPEM and caClientPEMs can be equal to caPEM (shared CA) or nil (use system CA
// pool).
func newServerTLSConfig(
	settings TLSSettings, certPEM, keyPEM, caPEM []byte, caClientPEMs ...[]byte,
) (*tls.Config, error) {
	cfg, err := newBaseTLSConfigWithCertificate(settings, certPEM, keyPEM, caPEM)
	if err != nil {
		return nil, err
	}
	cfg.ClientAuth = tls.VerifyClientCertIfGiven

	if len(caClientPEMs) != 0 {
		certPool := x509.NewCertPool()
		for _, pem := range caClientPEMs {
			if !certPool.AppendCertsFromPEM(pem) {
				return nil, errors.Errorf("failed to parse client CA PEM data to pool")
			}
		}
		cfg.ClientCAs = certPool
	}

	if settings.oldCipherSuitesEnabled() {
		cfg.CipherSuites = append(
			cfg.CipherSuites,
			OldCipherSuites()...,
		)
	}

	// Should we disable session resumption? This may break forward secrecy.
	// cfg.SessionTicketsDisabled = true
	return cfg, nil
}

// newUIServerTLSConfig creates a server TLSConfig for the Admin UI. It does not
// use client authentication or a CA.
// It needs:
// - the server certificate (should be signed by the CA used by HTTP clients to the admin UI)
// - the private key for the certificate
func newUIServerTLSConfig(settings TLSSettings, certPEM, keyPEM []byte) (*tls.Config, error) {
	cfg, err := newBaseTLSConfigWithCertificate(settings, certPEM, keyPEM, nil)
	if err != nil {
		return nil, err
	}

	if settings.oldCipherSuitesEnabled() {
		cfg.CipherSuites = append(
			cfg.CipherSuites,
			OldCipherSuites()...,
		)
	}

	// Should we disable session resumption? This may break forward secrecy.
	// cfg.SessionTicketsDisabled = true
	return cfg, nil
}

// newClientTLSConfig creates a client TLSConfig from the supplied byte strings containing:
// - the certificate of this client (should be signed by the CA),
// - the private key of this client.
// - the certificate of the cluster CA (use system cert pool if nil)
func newClientTLSConfig(settings TLSSettings, certPEM, keyPEM, caPEM []byte) (*tls.Config, error) {
	return newBaseTLSConfigWithCertificate(settings, certPEM, keyPEM, caPEM)
}

// newUIClientTLSConfig creates a client TLSConfig to talk to the Admin UI.
// It does not include client certificates and takes an optional CA certificate.
func newUIClientTLSConfig(settings TLSSettings, caPEM []byte) (*tls.Config, error) {
	return newBaseTLSConfig(settings, caPEM)
}

// newBaseTLSConfigWithCertificate returns a tls.Config initialized with the
// passed-in certificate and optional CA certificate.
func newBaseTLSConfigWithCertificate(
	settings TLSSettings, certPEM, keyPEM, caPEM []byte,
) (*tls.Config, error) {
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}

	cfg, err := newBaseTLSConfig(settings, caPEM)
	if err != nil {
		return nil, err
	}

	cfg.Certificates = []tls.Certificate{cert}
	return cfg, nil
}

// newBaseTLSConfig returns a tls.Config. If caPEM != nil, it is set in RootCAs.
func newBaseTLSConfig(settings TLSSettings, caPEM []byte) (*tls.Config, error) {
	var certPool *x509.CertPool
	if caPEM != nil {
		certPool = x509.NewCertPool()

		if !certPool.AppendCertsFromPEM(caPEM) {
			return nil, errors.Errorf("failed to parse PEM data to pool")
		}
	}

	cfg := &tls.Config{
		RootCAs: certPool,

		VerifyPeerCertificate: makeOCSPVerifier(settings),

		CipherSuites: RecommendedCipherSuites(),

		MinVersion: tls.VersionTLS12,
	}

	if settings.skipHostnameVerification() {
		// Ratel's shared-cert model: verify that the peer cert chains to our
		// CA, but do not verify the hostname / IP SAN. Setting
		// InsecureSkipVerify bypasses all of Go's built-in verification, so
		// we install a VerifyPeerCertificate that re-runs the chain build
		// (without DNSName) and then delegates to the OCSP verifier.
		ocspVerifier := makeOCSPVerifier(settings)
		cfg.InsecureSkipVerify = true
		cfg.VerifyPeerCertificate = func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return errors.New("tls: peer presented no certificate")
			}
			certs := make([]*x509.Certificate, 0, len(rawCerts))
			for _, raw := range rawCerts {
				cert, err := x509.ParseCertificate(raw)
				if err != nil {
					return errors.Wrap(err, "tls: parsing peer certificate")
				}
				certs = append(certs, cert)
			}
			intermediates := x509.NewCertPool()
			for _, c := range certs[1:] {
				intermediates.AddCert(c)
			}
			if _, err := certs[0].Verify(x509.VerifyOptions{
				Roots:         certPool,
				Intermediates: intermediates,
				KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
			}); err != nil {
				return errors.Wrap(err, "tls: chain verification failed")
			}
			return ocspVerifier(rawCerts, [][]*x509.Certificate{certs})
		}
	}

	return cfg, nil
}
