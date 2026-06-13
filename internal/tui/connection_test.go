package tui

import (
	"errors"
	"strings"
	"testing"

	"termua/internal/opcua"
)

func anonymousEndpoint(policy, mode string) opcua.Endpoint {
	return opcua.Endpoint{SecurityPolicy: policy, SecurityMode: mode, UserTokenTypes: []string{"Anonymous"}}
}

func usernameEndpoint(policy, mode string) opcua.Endpoint {
	return opcua.Endpoint{SecurityPolicy: policy, SecurityMode: mode, UserTokenTypes: []string{"UserName"}}
}

func TestServerConnectionEmptySubmitNeedsEndpoint(t *testing.T) {
	connection := NewServerConnection("")

	requests := connection.Submit()
	view := connection.View()

	if len(requests) != 0 {
		t.Fatalf("expected no requests, got %#v", requests)
	}
	if view.Status != ServerConnectionNeedsEndpoint {
		t.Fatalf("status = %v", view.Status)
	}
}

func TestServerConnectionSubmitEndpointBeforeDiscoveryRequestsDiscovery(t *testing.T) {
	connection := NewServerConnection("opc.tcp://localhost:4840")

	requests := connection.Submit()
	view := connection.View()

	if len(requests) != 1 || requests[0].Kind != ServerConnectionRequestDiscoverEndpoints {
		t.Fatalf("requests = %#v", requests)
	}
	if requests[0].Endpoint != "opc.tcp://localhost:4840" {
		t.Fatalf("endpoint = %q", requests[0].Endpoint)
	}
	if view.Status != ServerConnectionDiscovering || !view.Discovering {
		t.Fatalf("view = %#v", view)
	}
}

func TestServerConnectionDiscoveryDefaultSkipsSecureEndpointWithoutCertificateAndKey(t *testing.T) {
	connection := NewServerConnection("opc.tcp://localhost:4840")
	endpoints := []opcua.Endpoint{
		anonymousEndpoint("Basic256Sha256", "Sign"),
		usernameEndpoint("None", "None"),
	}

	connection.ApplyDiscovery(endpoints, nil)

	if selected := connection.View().SelectedEndpoint; selected != 1 {
		t.Fatalf("selected endpoint = %d, expected username/password endpoint", selected)
	}
}

func TestServerConnectionDiscoveryDefaultCanSelectSecureEndpointWithCertificateAndKey(t *testing.T) {
	connection := NewServerConnection("opc.tcp://localhost:4840")
	connection.SetClientCertificatePaths("cert.pem", "key.pem")
	endpoints := []opcua.Endpoint{
		anonymousEndpoint("Basic256Sha256", "Sign"),
		usernameEndpoint("None", "None"),
	}

	connection.ApplyDiscovery(endpoints, nil)

	if selected := connection.View().SelectedEndpoint; selected != 0 {
		t.Fatalf("selected endpoint = %d, expected secure endpoint", selected)
	}
}

func TestServerConnectionDiscoveryStoresEndpointsAndDefaultsToNoneNoneAnonymous(t *testing.T) {
	connection := NewServerConnection("opc.tcp://localhost:4840")
	endpoints := []opcua.Endpoint{
		anonymousEndpoint("Basic256Sha256", "Sign"),
		usernameEndpoint("None", "None"),
		anonymousEndpoint("None", "None"),
	}

	connection.ApplyDiscovery(endpoints, nil)
	view := connection.View()

	if len(view.Endpoints) != len(endpoints) {
		t.Fatalf("endpoints = %#v", view.Endpoints)
	}
	if view.SelectedEndpoint != 2 {
		t.Fatalf("selected endpoint = %d", view.SelectedEndpoint)
	}
	if view.Status != ServerConnectionDiscovered || view.Discovering || view.Connected || view.Connecting {
		t.Fatalf("view = %#v", view)
	}
}

func TestServerConnectionEndpointSelectionMovementWraps(t *testing.T) {
	connection := NewServerConnection("opc.tcp://localhost:4840")
	connection.ApplyDiscovery([]opcua.Endpoint{
		anonymousEndpoint("Basic256Sha256", "Sign"),
		anonymousEndpoint("None", "None"),
	}, nil)

	connection.MoveEndpointSelection(1)
	if selected := connection.View().SelectedEndpoint; selected != 0 {
		t.Fatalf("selected after down = %d", selected)
	}
	connection.MoveEndpointSelection(-1)
	if selected := connection.View().SelectedEndpoint; selected != 1 {
		t.Fatalf("selected after up = %d", selected)
	}
}

func TestServerConnectionSubmitSecureSignEndpointIncludesConfiguredCertificateAndKey(t *testing.T) {
	connection := NewServerConnection("opc.tcp://localhost:4840")
	connection.SetClientCertificatePaths("cert.pem", "key.pem")
	connection.ApplyDiscovery([]opcua.Endpoint{anonymousEndpoint("Basic256Sha256", "Sign")}, nil)

	requests := connection.Submit()

	if len(requests) != 1 || requests[0].Kind != ServerConnectionRequestConnectEndpoint {
		t.Fatalf("requests = %#v", requests)
	}
	connect := requests[0].Connect
	if connect.ClientCertificatePath != "cert.pem" || connect.ClientPrivateKeyPath != "key.pem" {
		t.Fatalf("cert/key paths = %q / %q", connect.ClientCertificatePath, connect.ClientPrivateKeyPath)
	}
}

func TestServerConnectionSubmitSecureSignAndEncryptEndpointIncludesConfiguredCertificateAndKey(t *testing.T) {
	connection := NewServerConnection("opc.tcp://localhost:4840")
	connection.SetClientCertificatePaths("cert.pem", "key.pem")
	connection.ApplyDiscovery([]opcua.Endpoint{anonymousEndpoint("Basic256Sha256", "SignAndEncrypt")}, nil)

	requests := connection.Submit()

	if len(requests) != 1 || requests[0].Kind != ServerConnectionRequestConnectEndpoint {
		t.Fatalf("requests = %#v", requests)
	}
	connect := requests[0].Connect
	if connect.SecurityMode != "SignAndEncrypt" {
		t.Fatalf("security mode = %q", connect.SecurityMode)
	}
	if connect.ClientCertificatePath != "cert.pem" || connect.ClientPrivateKeyPath != "key.pem" {
		t.Fatalf("cert/key paths = %q / %q", connect.ClientCertificatePath, connect.ClientPrivateKeyPath)
	}
}

func TestServerConnectionSubmitSecureEndpointWithoutCertificateAndKeyFailsBeforeConnect(t *testing.T) {
	for _, securityMode := range []string{"Sign", "SignAndEncrypt"} {
		t.Run(securityMode, func(t *testing.T) {
			connection := NewServerConnection("opc.tcp://localhost:4840")
			connection.ApplyDiscovery([]opcua.Endpoint{anonymousEndpoint("Basic256Sha256", securityMode)}, nil)

			requests := connection.Submit()
			view := connection.View()

			if len(requests) != 0 {
				t.Fatalf("expected no connect request without cert/key, got %#v", requests)
			}
			if view.Status != ServerConnectionFailed {
				t.Fatalf("status = %v, expected Failed", view.Status)
			}
			if !strings.Contains(view.LastError, "client certificate and private key") {
				t.Fatalf("last error = %q", view.LastError)
			}
		})
	}
}

func TestServerConnectionSubmitDiscoveredAnonymousEndpointRequestsConnect(t *testing.T) {
	connection := NewServerConnection("opc.tcp://localhost:4840")
	connection.ApplyDiscovery([]opcua.Endpoint{anonymousEndpoint("None", "None")}, nil)

	requests := connection.Submit()
	view := connection.View()

	if len(requests) != 1 || requests[0].Kind != ServerConnectionRequestConnectEndpoint {
		t.Fatalf("requests = %#v", requests)
	}
	if requests[0].Connect.Endpoint != "opc.tcp://localhost:4840" || requests[0].Connect.AuthType != opcua.AuthAnonymous {
		t.Fatalf("connect request = %#v", requests[0].Connect)
	}
	if view.Status != ServerConnectionConnecting || !view.Connecting {
		t.Fatalf("view = %#v", view)
	}
}

func TestServerConnectionSubmitUsernameOnlyEndpointTransitionsToCredentialEntry(t *testing.T) {
	connection := NewServerConnection("opc.tcp://localhost:4840")
	connection.ApplyDiscovery([]opcua.Endpoint{usernameEndpoint("None", "None")}, nil)

	requests := connection.Submit()
	view := connection.View()

	if len(requests) != 0 {
		t.Fatalf("expected no requests before credentials, got %#v", requests)
	}
	if view.Status != ServerConnectionEnteringCredentials || view.Connecting {
		t.Fatalf("view = %#v", view)
	}
}

func TestServerConnectionApplyConnectionSuccess(t *testing.T) {
	connection := NewServerConnection("opc.tcp://localhost:4840")
	request := opcua.ConnectRequest{Endpoint: "opc.tcp://localhost:4840", SecurityPolicy: "None", SecurityMode: "None", AuthType: opcua.AuthAnonymous}

	connection.ApplyConnection(request, nil)
	view := connection.View()

	if view.Status != ServerConnectionConnected || !view.Connected || view.Connecting {
		t.Fatalf("view = %#v", view)
	}
}

func TestServerConnectionApplyConnectionFailure(t *testing.T) {
	connection := NewServerConnection("opc.tcp://localhost:4840")
	request := opcua.ConnectRequest{Endpoint: "opc.tcp://localhost:4840", SecurityPolicy: "None", SecurityMode: "None", AuthType: opcua.AuthAnonymous}

	connection.ApplyConnection(request, errors.New("boom"))
	view := connection.View()

	if view.Status != ServerConnectionFailed || view.Connected || view.Connecting {
		t.Fatalf("view = %#v", view)
	}
}

func multiAuthEndpoint(policy, mode string) opcua.Endpoint {
	return opcua.Endpoint{SecurityPolicy: policy, SecurityMode: mode, UserTokenTypes: []string{"Anonymous", "UserName"}}
}

func TestServerConnectionSubmitMultiAuthEndpointTransitionsToSelectingAuthType(t *testing.T) {
	connection := NewServerConnection("opc.tcp://localhost:4840")
	connection.ApplyDiscovery([]opcua.Endpoint{multiAuthEndpoint("None", "None")}, nil)

	requests := connection.Submit()
	view := connection.View()

	if len(requests) != 0 {
		t.Fatalf("expected no requests before auth selection, got %#v", requests)
	}
	if view.Status != ServerConnectionSelectingAuthType {
		t.Fatalf("status = %v, expected SelectingAuthType", view.Status)
	}
	if len(view.AuthTypes) != 2 {
		t.Fatalf("auth types = %v", view.AuthTypes)
	}
}

func TestServerConnectionAuthTypeListMatchesEndpointTokens(t *testing.T) {
	connection := NewServerConnection("opc.tcp://localhost:4840")
	connection.ApplyDiscovery([]opcua.Endpoint{multiAuthEndpoint("None", "None")}, nil)

	connection.Submit()
	view := connection.View()

	if view.AuthTypes[0] != "Anonymous" || view.AuthTypes[1] != "UserName" {
		t.Fatalf("auth types = %v, expected [Anonymous UserName]", view.AuthTypes)
	}
	if !view.HasAuthTypeSelection {
		t.Fatal("expected HasAuthTypeSelection to be true")
	}
	if view.HasEndpointSelection {
		t.Fatal("expected HasEndpointSelection to be false during auth selection")
	}
}

func TestServerConnectionSelectAnonymousAuthConnectsDirectly(t *testing.T) {
	connection := NewServerConnection("opc.tcp://localhost:4840")
	connection.ApplyDiscovery([]opcua.Endpoint{multiAuthEndpoint("None", "None")}, nil)
	connection.Submit() // transitions to SelectingAuthType

	requests := connection.SelectAuthType(0) // select Anonymous
	view := connection.View()

	if len(requests) != 1 || requests[0].Kind != ServerConnectionRequestConnectEndpoint {
		t.Fatalf("requests = %#v", requests)
	}
	if requests[0].Connect.AuthType != opcua.AuthAnonymous {
		t.Fatalf("auth type = %v, expected Anonymous", requests[0].Connect.AuthType)
	}
	if view.Status != ServerConnectionConnecting || !view.Connecting {
		t.Fatalf("view = %#v", view)
	}
}

func TestServerConnectionSelectUserNameAuthTransitionsToEnteringCredentials(t *testing.T) {
	connection := NewServerConnection("opc.tcp://localhost:4840")
	connection.ApplyDiscovery([]opcua.Endpoint{multiAuthEndpoint("None", "None")}, nil)
	connection.Submit() // transitions to SelectingAuthType

	requests := connection.SelectAuthType(1) // select UserName
	view := connection.View()

	if len(requests) != 0 {
		t.Fatalf("expected no requests before credentials, got %#v", requests)
	}
	if view.Status != ServerConnectionEnteringCredentials {
		t.Fatalf("status = %v, expected EnteringCredentials", view.Status)
	}
}

func TestServerConnectionCertificateOnlyEndpointWithoutCertificateFailsBeforeConnect(t *testing.T) {
	connection := NewServerConnection("opc.tcp://localhost:4840")
	connection.ApplyDiscovery([]opcua.Endpoint{{SecurityPolicy: "None", SecurityMode: "None", UserTokenTypes: []string{"Certificate"}}}, nil)

	requests := connection.Submit()
	view := connection.View()

	if len(requests) != 0 {
		t.Fatalf("expected no connect request without certificate/key, got %#v", requests)
	}
	if view.Status != ServerConnectionFailed {
		t.Fatalf("status = %v, expected Failed", view.Status)
	}
	if !strings.Contains(view.LastError, "client certificate and private key") {
		t.Fatalf("last error = %q", view.LastError)
	}
}

func TestServerConnectionCertificateOnlyEndpointWithCertificateConnects(t *testing.T) {
	connection := NewServerConnection("opc.tcp://localhost:4840")
	connection.SetClientCertificatePaths("cert.pem", "key.pem")
	connection.ApplyDiscovery([]opcua.Endpoint{{SecurityPolicy: "None", SecurityMode: "None", UserTokenTypes: []string{"Certificate"}}}, nil)

	requests := connection.Submit()
	view := connection.View()

	if len(requests) != 1 || requests[0].Connect.AuthType != opcua.AuthCertificate {
		t.Fatalf("requests = %#v", requests)
	}
	if requests[0].Connect.ClientCertificatePath != "cert.pem" || requests[0].Connect.ClientPrivateKeyPath != "key.pem" {
		t.Fatalf("cert/key paths = %q / %q", requests[0].Connect.ClientCertificatePath, requests[0].Connect.ClientPrivateKeyPath)
	}
	if view.Status != ServerConnectionConnecting {
		t.Fatalf("status = %v, expected Connecting", view.Status)
	}
}

func TestServerConnectionSelectCertificateAuthConnects(t *testing.T) {
	connection := NewServerConnection("opc.tcp://localhost:4840")
	connection.SetClientCertificatePaths("cert.pem", "key.pem")
	connection.ApplyDiscovery([]opcua.Endpoint{{SecurityPolicy: "None", SecurityMode: "None", UserTokenTypes: []string{"Anonymous", "Certificate"}}}, nil)
	connection.Submit() // transitions to SelectingAuthType

	requests := connection.SelectAuthType(1) // select Certificate
	view := connection.View()

	if len(requests) != 1 || requests[0].Connect.AuthType != opcua.AuthCertificate {
		t.Fatalf("requests = %#v", requests)
	}
	if view.Status != ServerConnectionConnecting {
		t.Fatalf("status = %v, expected Connecting", view.Status)
	}
}

func TestServerConnectionSelectIssuedTokenAuthFailsWithoutConnectRequest(t *testing.T) {
	connection := NewServerConnection("opc.tcp://localhost:4840")
	connection.ApplyDiscovery([]opcua.Endpoint{{SecurityPolicy: "None", SecurityMode: "None", UserTokenTypes: []string{"Anonymous", "IssuedToken"}}}, nil)
	connection.Submit() // transitions to SelectingAuthType

	requests := connection.SelectAuthType(1) // select IssuedToken
	view := connection.View()

	if len(requests) != 0 {
		t.Fatalf("expected no connect request for unsupported auth, got %#v", requests)
	}
	if view.Status != ServerConnectionSelectingAuthType {
		t.Fatalf("status = %v, expected SelectingAuthType", view.Status)
	}
	if !view.HasAuthTypeSelection {
		t.Fatalf("expected auth selection to remain available after unsupported auth")
	}
	if !strings.Contains(view.LastError, "unsupported authentication") || !strings.Contains(view.LastError, "IssuedToken") {
		t.Fatalf("last error = %q", view.LastError)
	}
}

func TestServerConnectionSubmitCredentialsProducesConnectRequest(t *testing.T) {
	connection := NewServerConnection("opc.tcp://localhost:4840")
	connection.ApplyDiscovery([]opcua.Endpoint{multiAuthEndpoint("None", "None")}, nil)
	connection.Submit()
	connection.SelectAuthType(1) // UserName → EnteringCredentials

	connection.SetCredentials("admin", "secret")
	requests := connection.SubmitCredentials()
	view := connection.View()

	if len(requests) != 1 || requests[0].Kind != ServerConnectionRequestConnectEndpoint {
		t.Fatalf("requests = %#v", requests)
	}
	if requests[0].Connect.AuthType != opcua.AuthUsername {
		t.Fatalf("auth type = %v", requests[0].Connect.AuthType)
	}
	if requests[0].Connect.Username != "admin" || requests[0].Connect.Password != "secret" {
		t.Fatalf("credentials = %q / %q", requests[0].Connect.Username, requests[0].Connect.Password)
	}
	if view.Status != ServerConnectionConnecting || !view.Connecting {
		t.Fatalf("view = %#v", view)
	}
}

func TestServerConnectionUsernameOnlyEndpointCredentialsProduceUserNameConnectRequest(t *testing.T) {
	connection := NewServerConnection("opc.tcp://localhost:4840")
	connection.ApplyDiscovery([]opcua.Endpoint{usernameEndpoint("None", "None")}, nil)
	connection.Submit()

	connection.SetCredentials("operator", "secret")
	requests := connection.SubmitCredentials()

	if len(requests) != 1 || requests[0].Kind != ServerConnectionRequestConnectEndpoint {
		t.Fatalf("requests = %#v", requests)
	}
	connect := requests[0].Connect
	if connect.Endpoint != "opc.tcp://localhost:4840" || connect.SecurityPolicy != "None" || connect.SecurityMode != "None" {
		t.Fatalf("connect request endpoint/security = %#v", connect)
	}
	if connect.AuthType != opcua.AuthUsername {
		t.Fatalf("auth type = %v, expected UserName", connect.AuthType)
	}
	if connect.Username != "operator" || connect.Password != "secret" {
		t.Fatalf("credentials = %q / %q", connect.Username, connect.Password)
	}
}

func TestServerConnectionBackFromAuthTypeReturnsToEndpointSelection(t *testing.T) {
	connection := NewServerConnection("opc.tcp://localhost:4840")
	connection.ApplyDiscovery([]opcua.Endpoint{multiAuthEndpoint("None", "None")}, nil)
	connection.Submit() // SelectingAuthType

	connection.Back()
	view := connection.View()

	if view.Status != ServerConnectionDiscovered {
		t.Fatalf("status = %v, expected Discovered", view.Status)
	}
	if !view.HasEndpointSelection {
		t.Fatal("expected HasEndpointSelection after going back")
	}
	if view.HasAuthTypeSelection {
		t.Fatal("expected HasAuthTypeSelection to be false after going back")
	}
}

func TestServerConnectionBackFromCredentialsReturnsToAuthType(t *testing.T) {
	connection := NewServerConnection("opc.tcp://localhost:4840")
	connection.ApplyDiscovery([]opcua.Endpoint{multiAuthEndpoint("None", "None")}, nil)
	connection.Submit()
	connection.SelectAuthType(1) // EnteringCredentials

	connection.Back()
	view := connection.View()

	if view.Status != ServerConnectionSelectingAuthType {
		t.Fatalf("status = %v, expected SelectingAuthType", view.Status)
	}
	if !view.HasAuthTypeSelection {
		t.Fatal("expected HasAuthTypeSelection after going back to auth selection")
	}
}

func TestServerConnectionBackFromUsernameOnlyCredentialsReturnsToEndpointSelection(t *testing.T) {
	connection := NewServerConnection("opc.tcp://localhost:4840")
	connection.ApplyDiscovery([]opcua.Endpoint{usernameEndpoint("None", "None")}, nil)
	connection.Submit() // EnteringCredentials

	connection.Back()
	view := connection.View()

	if view.Status != ServerConnectionDiscovered {
		t.Fatalf("status = %v, expected Discovered", view.Status)
	}
	if !view.HasEndpointSelection {
		t.Fatal("expected HasEndpointSelection after going back")
	}
}
