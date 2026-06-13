package config

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureClientCertificateCreatesReusableCertificateAndKey(t *testing.T) {
	paths := Paths{ConfigDir: t.TempDir()}

	certPath, keyPath, err := EnsureClientCertificate(paths)
	if err != nil {
		t.Fatal(err)
	}
	if certPath != filepath.Join(paths.ConfigDir, "certificates", "client-cert.pem") {
		t.Fatalf("cert path = %q", certPath)
	}
	if keyPath != filepath.Join(paths.ConfigDir, "certificates", "client-key.pem") {
		t.Fatalf("key path = %q", keyPath)
	}

	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatal(err)
	}
	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil || certBlock.Type != "CERTIFICATE" {
		t.Fatalf("certificate was not PEM encoded")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if cert.Subject.CommonName != "TermUA Client" {
		t.Fatalf("common name = %q", cert.Subject.CommonName)
	}
	if len(cert.URIs) == 0 {
		t.Fatal("expected application URI in certificate")
	}
	if cert.KeyUsage&x509.KeyUsageContentCommitment == 0 {
		t.Fatal("expected certificate key usage to include content commitment/non-repudiation")
	}

	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil || keyBlock.Type != "RSA PRIVATE KEY" {
		t.Fatalf("private key was not PKCS#1 PEM encoded")
	}
	if _, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes); err != nil {
		t.Fatal(err)
	}

	originalKey, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = EnsureClientCertificate(paths)
	if err != nil {
		t.Fatal(err)
	}
	reusedKey, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(originalKey) != string(reusedKey) {
		t.Fatal("expected existing key to be reused")
	}
}
