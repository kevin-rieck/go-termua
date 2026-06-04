package tui

import (
	"context"
	"fmt"
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

func TestConnectedEndpointBrowsesObjectsRoot(t *testing.T) {
	client := &fakeClient{children: map[string][]opcua.AddressNode{
		"i=85": {{NodeID: "i=2253", DisplayName: "Server", BrowseName: "Server", NodeClass: "Object"}},
	}}
	model := NewModel(Dependencies{Client: client})

	updated, cmd := model.Update(endpointConnectionMsg{Request: opcua.ConnectRequest{SecurityMode: "None", SecurityPolicy: "None"}})
	if cmd == nil {
		t.Fatal("expected browse command")
	}
	msg := cmd()
	browse, ok := msg.(browseChildrenMsg)
	if !ok {
		t.Fatalf("expected browseChildrenMsg, got %T", msg)
	}
	if browse.ParentNodeID != "i=85" {
		t.Fatalf("browsed node = %q", browse.ParentNodeID)
	}

	updated, _ = updated.(Model).Update(browse)
	view := updated.(Model).View()
	if !strings.Contains(view, "Server") {
		t.Fatalf("expected browsed child in view:\n%s", view)
	}
}

func TestAddressSpaceScrollsToSelectedNode(t *testing.T) {
	model := NewModel(Dependencies{})
	model.connected = true
	model.height = 18
	model.addressSpace = &AddressSpace{tree: []treeNode{{node: opcua.AddressNode{NodeID: "i=85", DisplayName: "Objects", NodeClass: "Object"}, expanded: true, childrenLoaded: true}}}
	for i := 1; i <= 20; i++ {
		model.addressSpace.tree = append(model.addressSpace.tree, treeNode{
			node:  opcua.AddressNode{NodeID: "i=" + string(rune('a'+i)), DisplayName: fmt.Sprintf("Node%02d", i), NodeClass: "Object"},
			depth: 1,
		})
	}

	updated := tea.Model(model)
	for i := 0; i < 12; i++ {
		updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	view := updated.(Model).View()

	if !strings.Contains(view, "Node12") {
		t.Fatalf("expected selected node to scroll into view:\n%s", view)
	}
	if strings.Contains(view, "Node01") {
		t.Fatalf("expected early nodes to scroll out of view:\n%s", view)
	}
}

func TestExpandSelectedNodeBrowsesLazily(t *testing.T) {
	client := &fakeClient{children: map[string][]opcua.AddressNode{
		"i=2253": {{NodeID: "i=2258", DisplayName: "ServerStatus", BrowseName: "ServerStatus", NodeClass: "Variable"}},
	}}
	model := NewModel(Dependencies{Client: client})
	model.connected = true
	model.addressSpace = &AddressSpace{tree: []treeNode{
		{node: opcua.AddressNode{NodeID: "i=85", DisplayName: "Objects", NodeClass: "Object"}, expanded: true, childrenLoaded: true},
		{node: opcua.AddressNode{NodeID: "i=2253", DisplayName: "Server", NodeClass: "Object"}, depth: 1},
	}}
	model.selectedTree = 1

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected lazy browse command")
	}
	msg := cmd().(browseChildrenMsg)
	updated, _ = updated.(Model).Update(msg)
	view := updated.(Model).View()
	if !strings.Contains(view, "ServerStatus") || !strings.Contains(view, "variable") {
		t.Fatalf("expected variable child in view:\n%s", view)
	}
}

func TestWatchlistSubscribesSelectedVariableNode(t *testing.T) {
	updates := make(chan opcua.LiveValue, 1)
	client := &fakeClient{subscriptions: map[string]chan opcua.LiveValue{"ns=2;s=Temperature": updates}}
	model := NewModel(Dependencies{Client: client})
	model.connected = true
	model.addressSpace = &AddressSpace{tree: []treeNode{
		{node: opcua.AddressNode{NodeID: "i=85", DisplayName: "Objects", NodeClass: "Object"}, expanded: true, childrenLoaded: true},
		{node: opcua.AddressNode{NodeID: "ns=2;s=Temperature", DisplayName: "Temperature", NodeClass: "Variable"}, depth: 1},
	}}
	model.selectedTree = 1

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	if cmd == nil {
		t.Fatal("expected subscribe command")
	}
	model = updated.(Model)
	if len(model.watchlist) != 1 || !model.watchlist[0].subscribing {
		t.Fatalf("watchlist = %#v", model.watchlist)
	}

	updated, cmd = model.Update(cmd())
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected wait-for-value command")
	}

	updates <- opcua.LiveValue{NodeID: "ns=2;s=Temperature", Value: "42.5", Status: "OK"}
	updated, _ = model.Update(cmd())
	view := updated.(Model).View()
	if !strings.Contains(view, "Temperature") || !strings.Contains(view, "42.5 · OK") {
		t.Fatalf("expected subscribed Live Value in watchlist:\n%s", view)
	}
}

func TestSelectedVariableNodeSubscribesLiveValueIntoDetails(t *testing.T) {
	updates := make(chan opcua.LiveValue, 1)
	client := &fakeClient{subscriptions: map[string]chan opcua.LiveValue{"ns=2;s=Pressure": updates}}
	model := NewModel(Dependencies{Client: client})
	model.connected = true
	model.addressSpace = &AddressSpace{tree: []treeNode{
		{node: opcua.AddressNode{NodeID: "i=85", DisplayName: "Objects", NodeClass: "Object"}, expanded: true, childrenLoaded: true},
		{node: opcua.AddressNode{NodeID: "ns=2;s=Pressure", DisplayName: "Pressure", NodeClass: "Variable"}, depth: 1},
	}}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	if cmd == nil {
		t.Fatal("expected selected-node subscribe command")
	}
	model = updated.(Model)
	if !model.selectedValue.subscribing || model.selectedValue.node.NodeID != "ns=2;s=Pressure" {
		t.Fatalf("selected value = %#v", model.selectedValue)
	}

	updated, cmd = model.Update(cmd())
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected wait-for-selected-value command")
	}

	updates <- opcua.LiveValue{NodeID: "ns=2;s=Pressure", Value: "12.3", Status: "StatusOK"}
	updated, _ = model.Update(cmd())
	view := updated.(Model).View()
	if !strings.Contains(view, "Pressure") || !strings.Contains(view, "Value") || !strings.Contains(view, "12.3") || !strings.Contains(view, "Health") {
		t.Fatalf("expected selected Live Value in details:\n%s", view)
	}
}

func TestSelectingNonVariableCancelsSelectedLiveValue(t *testing.T) {
	sub := &fakeSubscription{}
	model := NewModel(Dependencies{Client: &fakeClient{}})
	model.connected = true
	model.addressSpace = &AddressSpace{tree: []treeNode{
		{node: opcua.AddressNode{NodeID: "i=85", DisplayName: "Objects", NodeClass: "Object"}, expanded: true, childrenLoaded: true},
	}}
	model.selectedValue = selectedValueState{
		node:         opcua.AddressNode{NodeID: "ns=2;s=Flow", DisplayName: "Flow", NodeClass: "Variable"},
		subscription: sub,
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)
	if model.selectedValue.node.NodeID != "" {
		t.Fatalf("expected selected Live Value cleared, got %#v", model.selectedValue)
	}
	if cmd == nil {
		t.Fatal("expected cancel command")
	}
	cmd()
	if !sub.cancelled {
		t.Fatal("expected previous selected-node subscription cancelled")
	}
}

func TestWatchlistScrollsWhenFocused(t *testing.T) {
	model := NewModel(Dependencies{})
	model.focus = focusWatchlist
	for i := 0; i < 5; i++ {
		model.watchlist = append(model.watchlist, watchItem{node: opcua.AddressNode{NodeID: fmt.Sprintf("ns=2;s=Node%d", i), DisplayName: fmt.Sprintf("Node%d", i), NodeClass: "Variable"}})
	}

	updated := tea.Model(model)
	for i := 0; i < 3; i++ {
		updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	model = updated.(Model)
	lines := strings.Join(model.watchlistLines(9), "\n")

	if model.watchScroll == 0 {
		t.Fatal("expected Watchlist to scroll")
	}
	if !strings.Contains(lines, "Node3") || strings.Contains(lines, "Node0 =") {
		t.Fatalf("expected Watchlist window around selected item, got:\n%s", lines)
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
	discovered    string
	endpoints     []opcua.Endpoint
	connected     opcua.ConnectRequest
	children      map[string][]opcua.AddressNode
	subscriptions map[string]chan opcua.LiveValue
}

func (f *fakeClient) DiscoverEndpoints(ctx context.Context, endpoint string) ([]opcua.Endpoint, error) {
	f.discovered = endpoint
	return f.endpoints, nil
}

func (f *fakeClient) Connect(ctx context.Context, request opcua.ConnectRequest) error {
	f.connected = request
	return nil
}

func (f *fakeClient) BrowseChildren(ctx context.Context, nodeID string) ([]opcua.AddressNode, error) {
	return f.children[nodeID], nil
}

func (f *fakeClient) SubscribeValue(ctx context.Context, nodeID string) (<-chan opcua.LiveValue, opcua.ValueSubscription, error) {
	return f.subscriptions[nodeID], &fakeSubscription{}, nil
}

func (f *fakeClient) Close(ctx context.Context) error { return nil }

type fakeSubscription struct {
	cancelled bool
}

func (f *fakeSubscription) Cancel(ctx context.Context) error {
	f.cancelled = true
	return nil
}
