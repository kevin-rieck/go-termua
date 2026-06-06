package opcua

import (
	"context"
	"strings"
	"testing"
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
