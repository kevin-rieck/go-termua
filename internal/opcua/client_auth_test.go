package opcua

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestClientConnectRejectsSecureEndpointWithoutCertificateAndKey(t *testing.T) {
	client := &gopcuaClient{}

	err := client.Connect(context.Background(), ConnectRequest{
		Endpoint:       "opc.tcp://localhost:4840",
		SecurityPolicy: "Basic256Sha256",
		SecurityMode:   "Sign",
		AuthType:       AuthAnonymous,
	})

	if err == nil {
		t.Fatal("expected missing certificate/key error")
	}
	if !strings.Contains(err.Error(), "client certificate and private key") {
		t.Fatalf("error = %v", err)
	}
}

func TestClientConnectRejectsSecureEndpointWithMissingCertificateFile(t *testing.T) {
	client := &gopcuaClient{}

	err := client.Connect(context.Background(), ConnectRequest{
		Endpoint:              "opc.tcp://localhost:4840",
		SecurityPolicy:        "Basic256Sha256",
		SecurityMode:          "Sign",
		AuthType:              AuthAnonymous,
		ClientCertificatePath: filepath.Join(t.TempDir(), "missing-cert.pem"),
		ClientPrivateKeyPath:  filepath.Join(t.TempDir(), "missing-key.pem"),
	})

	if err == nil {
		t.Fatal("expected missing certificate file error")
	}
	if !strings.Contains(err.Error(), "client certificate file") || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error = %v", err)
	}
}

func TestClientConnectRejectsSecureEndpointWithMissingPrivateKeyFile(t *testing.T) {
	client := &gopcuaClient{}
	dir := t.TempDir()
	certPath := writeTestCertificate(t, dir)

	err := client.Connect(context.Background(), ConnectRequest{
		Endpoint:              "opc.tcp://localhost:4840",
		SecurityPolicy:        "Basic256Sha256",
		SecurityMode:          "Sign",
		AuthType:              AuthAnonymous,
		ClientCertificatePath: certPath,
		ClientPrivateKeyPath:  filepath.Join(dir, "missing-key.pem"),
	})

	if err == nil {
		t.Fatal("expected missing private key file error")
	}
	if !strings.Contains(err.Error(), "client private key file") || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error = %v", err)
	}
}

func TestClientConnectRejectsSecureEndpointWithInvalidPrivateKeyFile(t *testing.T) {
	client := &gopcuaClient{}
	dir := t.TempDir()
	certPath := writeTestCertificate(t, dir)
	keyPath := filepath.Join(dir, "key.pem")
	if err := os.WriteFile(keyPath, []byte("not a key"), 0600); err != nil {
		t.Fatal(err)
	}

	err := client.Connect(context.Background(), ConnectRequest{
		Endpoint:              "opc.tcp://localhost:4840",
		SecurityPolicy:        "Basic256Sha256",
		SecurityMode:          "Sign",
		AuthType:              AuthAnonymous,
		ClientCertificatePath: certPath,
		ClientPrivateKeyPath:  keyPath,
	})

	if err == nil {
		t.Fatal("expected invalid private key file error")
	}
	if !strings.Contains(err.Error(), "client private key file") || !strings.Contains(err.Error(), "PEM RSA private key") {
		t.Fatalf("error = %v", err)
	}
}

func TestSecureConnectErrorExplainsServerClosedSecureChannel(t *testing.T) {
	err := secureConnectError(ConnectRequest{
		SecurityMode:          "SignAndEncrypt",
		ClientCertificatePath: "certificates/client-cert.pem",
	}, io.EOF)

	if err == nil {
		t.Fatal("expected secure channel error")
	}
	if !strings.Contains(err.Error(), "trust the TermUA client application certificate") || !strings.Contains(err.Error(), "certificates/client-cert.pem") {
		t.Fatalf("error = %v", err)
	}
}

func TestClientConnectRejectsUnsupportedAuthType(t *testing.T) {
	for _, authType := range []AuthType{AuthType("Certificate"), AuthType("IssuedToken")} {
		t.Run(string(authType), func(t *testing.T) {
			client := &gopcuaClient{}

			err := client.Connect(context.Background(), ConnectRequest{
				Endpoint:       "opc.tcp://localhost:4840",
				SecurityPolicy: "None",
				SecurityMode:   "None",
				AuthType:       authType,
			})

			if err == nil {
				t.Fatal("expected unsupported authentication error")
			}
			if !strings.Contains(err.Error(), "unsupported authentication") || !strings.Contains(err.Error(), string(authType)) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func writeTestCertificate(t *testing.T, dir string) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "TermUA Test Client"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "cert.pem")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := pem.Encode(file, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		t.Fatal(err)
	}
	return path
}
