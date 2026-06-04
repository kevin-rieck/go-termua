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
}

func (f *fakeClient) DiscoverEndpoints(ctx context.Context, endpoint string) ([]opcua.Endpoint, error) {
	f.discovered = endpoint
	return f.endpoints, nil
}

func (f *fakeClient) Connect(ctx context.Context, request opcua.ConnectRequest) error { return nil }
func (f *fakeClient) Close(ctx context.Context) error                                { return nil }
