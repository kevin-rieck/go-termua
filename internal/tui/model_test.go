package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"termua/internal/app"
	"termua/internal/config"
	"termua/internal/opcua"
)

func TestInitialViewShowsReadOnlyMode(t *testing.T) {
	model := NewModel(Dependencies{Paths: config.Paths{ConfigDir: "config", CacheDir: "cache"}})
	view := model.View()

	if !strings.Contains(view, "Read-Only Mode") {
		t.Fatalf("expected Read-Only Mode in view:\n%s", view)
	}
	if !strings.Contains(view, "Address Space") {
		t.Fatalf("expected Address Space panel in view:\n%s", view)
	}
}

func TestHelpToggle(t *testing.T) {
	model := NewModel(Dependencies{})
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	view := updated.(Model).View()

	if !strings.Contains(view, "Export Diagnostics Bundle") {
		t.Fatalf("expected help view:\n%s", view)
	}
}

func TestEndpointDiscoveryUpdatesDetails(t *testing.T) {
	model := NewModel(Dependencies{})
	updated, _ := model.Update(endpointDiscoveryMsg{Endpoints: []opcua.Endpoint{{
		SecurityMode:   "None",
		SecurityPolicy: "None",
		UserTokenTypes: []string{"Anonymous"},
	}}})
	view := updated.(Model).View()

	if !strings.Contains(view, "discovered 1 endpoint") {
		t.Fatalf("expected discovery status:\n%s", view)
	}
	if !strings.Contains(view, "None · None · Anonymous") {
		t.Fatalf("expected endpoint details:\n%s", view)
	}
	if !strings.Contains(view, "Enter connect") {
		t.Fatalf("expected endpoint selection footer:\n%s", view)
	}
}

func TestEndpointSelectionMovesAndConnects(t *testing.T) {
	client := &fakeClient{}
	model := NewModel(Dependencies{Client: client, Launch: app.LaunchOptions{Endpoint: "opc.tcp://localhost:4840"}})
	updated, _ := model.Update(endpointDiscoveryMsg{Endpoints: []opcua.Endpoint{
		{SecurityMode: "Sign", SecurityPolicy: "Basic256Sha256", UserTokenTypes: []string{"Anonymous"}},
		{SecurityMode: "None", SecurityPolicy: "None", UserTokenTypes: []string{"Anonymous"}},
	}})

	selected := updated.(Model)
	if selected.selectedEndpoint != 1 {
		t.Fatalf("selected endpoint = %d", selected.selectedEndpoint)
	}

	updated, _ = selected.Update(tea.KeyMsg{Type: tea.KeyDown})
	selected = updated.(Model)
	if selected.selectedEndpoint != 0 {
		t.Fatalf("selected endpoint after down = %d", selected.selectedEndpoint)
	}

	updated, cmd := selected.Update(tea.KeyMsg{Type: tea.KeyEnter})
	selected = updated.(Model)
	if cmd == nil {
		t.Fatal("expected connect command")
	}
	if !selected.connecting {
		t.Fatal("expected model to be connecting")
	}

	msg := cmd()
	connection, ok := msg.(endpointConnectionMsg)
	if !ok {
		t.Fatalf("expected endpointConnectionMsg, got %T", msg)
	}
	if connection.Err != nil {
		t.Fatalf("connect error = %v", connection.Err)
	}
	if client.connected.SecurityPolicy != "Basic256Sha256" || client.connected.SecurityMode != "Sign" {
		t.Fatalf("connected request = %#v", client.connected)
	}
}

func TestInitDiscoversEndpointWhenProvided(t *testing.T) {
	client := &fakeClient{endpoints: []opcua.Endpoint{{SecurityMode: "None", SecurityPolicy: "None"}}}
	model := NewModel(Dependencies{Client: client, Launch: app.LaunchOptions{Endpoint: "opc.tcp://localhost:4840"}})
	cmd := model.Init()
	if cmd == nil {
		t.Fatal("expected discovery command")
	}
	msg := cmd()
	if _, ok := msg.(endpointDiscoveryMsg); !ok {
		t.Fatalf("expected endpointDiscoveryMsg, got %T", msg)
	}
	if client.discovered != "opc.tcp://localhost:4840" {
		t.Fatalf("discovered endpoint = %q", client.discovered)
	}
}

type fakeClient struct {
	discovered string
	endpoints  []opcua.Endpoint
	connected  opcua.ConnectRequest
}

func (f *fakeClient) DiscoverEndpoints(ctx context.Context, endpoint string) ([]opcua.Endpoint, error) {
	f.discovered = endpoint
	return f.endpoints, nil
}

func (f *fakeClient) Connect(ctx context.Context, request opcua.ConnectRequest) error {
	f.connected = request
	return nil
}
func (f *fakeClient) Close(ctx context.Context) error { return nil }
