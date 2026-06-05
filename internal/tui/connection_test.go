package tui

import (
	"errors"
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

func TestServerConnectionSubmitEndpointWithoutAnonymousSupportRequiresCredentials(t *testing.T) {
	connection := NewServerConnection("opc.tcp://localhost:4840")
	connection.ApplyDiscovery([]opcua.Endpoint{usernameEndpoint("None", "None")}, nil)

	requests := connection.Submit()
	view := connection.View()

	if len(requests) != 0 {
		t.Fatalf("expected no requests, got %#v", requests)
	}
	if view.Status != ServerConnectionEndpointRequiresCredentials || view.Connecting {
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
