package config

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

const (
	clientCertificateDirName  = "certificates"
	clientCertificateFileName = "client-cert.pem"
	clientPrivateKeyFileName  = "client-key.pem"
)

// ClientCertificateDir returns the folder used for TermUA's generated client
// application certificate and private key.
func ClientCertificateDir(paths Paths) string {
	return filepath.Join(paths.ConfigDir, clientCertificateDirName)
}

// EnsureClientCertificate returns a reusable client application certificate/key
// pair for secure OPC UA message modes. Existing files are reused; missing files
// are created under the app config directory.
func EnsureClientCertificate(paths Paths) (certificatePath string, privateKeyPath string, err error) {
	certDir := ClientCertificateDir(paths)
	certPath := filepath.Join(certDir, clientCertificateFileName)
	keyPath := filepath.Join(certDir, clientPrivateKeyFileName)

	certReusable := reusableClientCertificateExists(certPath)
	keyExists := regularFileExists(keyPath)
	if certReusable && keyExists {
		return certPath, keyPath, nil
	}

	if err := os.MkdirAll(certDir, 0700); err != nil {
		return "", "", err
	}
	if err := generateClientCertificate(certPath, keyPath); err != nil {
		return "", "", err
	}
	return certPath, keyPath, nil
}

func regularFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func reusableClientCertificateExists(path string) bool {
	if !regularFileExists(path) {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		return false
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false
	}
	return cert.KeyUsage&x509.KeyUsageContentCommitment != 0
}

func generateClientCertificate(certPath, keyPath string) error {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return err
	}

	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "localhost"
	}
	applicationURI := &url.URL{Scheme: "urn", Opaque: "termua:client:" + hostname}

	template := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "TermUA Client"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageContentCommitment | x509.KeyUsageKeyEncipherment | x509.KeyUsageDataEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		URIs:         []*url.URL{applicationURI},
		DNSNames:     []string{hostname, "localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}

	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return err
	}

	certFile, err := os.OpenFile(certPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		_ = certFile.Close()
		return err
	}
	if err := certFile.Close(); err != nil {
		return err
	}

	keyFile, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	if err := pem.Encode(keyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}); err != nil {
		_ = keyFile.Close()
		return err
	}
	return keyFile.Close()
}
