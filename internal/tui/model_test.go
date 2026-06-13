package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	toast "github.com/kevin-rieck/go-bubble-toast"

	"termua/internal/app"
	"termua/internal/config"
	"termua/internal/opcua"
)

func markModelConnected(model *Model) {
	model.connection.ApplyConnection(opcua.ConnectRequest{SecurityMode: "None", SecurityPolicy: "None", AuthType: opcua.AuthAnonymous}, nil)
}

func runCmds(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		messages := make([]tea.Msg, 0, len(batch))
		for _, batchedCmd := range batch {
			messages = append(messages, batchedCmd())
		}
		return messages
	}
	return []tea.Msg{msg}
}

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

func TestConnectionModalOpensOnStartupWithoutEndpoint(t *testing.T) {
	model := NewModel(Dependencies{})
	view := model.View()

	if !strings.Contains(view, "Connect to OPC UA Server") {
		t.Fatalf("expected Connection Modal on startup:\n%s", view)
	}
	if !strings.Contains(view, "Enter server URL") {
		t.Fatalf("expected server URL prompt in Connection Modal:\n%s", view)
	}
}

func TestWatchlistIsRightPaneTabInsteadOfBottomPanel(t *testing.T) {
	model := NewModel(Dependencies{})
	model.connectionModalOpen = false
	view := model.View()

	if !strings.Contains(view, "Node Details") || !strings.Contains(view, "Watchlist (0)") {
		t.Fatalf("expected right pane tabs in view:\n%s", view)
	}
	if strings.Contains(view, "No Variable Nodes added yet.") {
		t.Fatalf("expected Watchlist to be hidden until its tab is active, got:\n%s", view)
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyTab})
	view = updated.(Model).View()

	if !strings.Contains(view, "No Variable Nodes added yet.") {
		t.Fatalf("expected Watchlist content after tabbing to Watchlist:\n%s", view)
	}
}

func TestRightPanePreservesWatchlistTabWhenFocusReturnsToAddressSpace(t *testing.T) {
	model := NewModel(Dependencies{})
	model.connectionModalOpen = false
	model.inspections.Watch(opcua.AddressNode{NodeID: "ns=2;s=Temperature", DisplayName: "Temperature", NodeClass: "Variable"})

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyTab})
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyTab})
	view := updated.(Model).View()

	if !strings.Contains(view, "Temperature") || !strings.Contains(view, "subscribing") {
		t.Fatalf("expected Watchlist tab to remain visible when Address Space regains focus:\n%s", view)
	}
}

func TestAddressSpaceAndRightPaneRenderSameHeight(t *testing.T) {
	model := NewModel(Dependencies{})
	model.connectionModalOpen = false
	view := model.View()

	assertPanelsShareBottomBorder(t, view)
}

func TestAddressSpaceAndRightPaneStaySameHeightWithLongRightPaneContent(t *testing.T) {
	model := NewModel(Dependencies{})
	model.connectionModalOpen = false
	model.details = make([]string, 0, 60)
	for i := 0; i < 60; i++ {
		model.details = append(model.details, fmt.Sprintf("Detail %02d: this is a long Node Details line that should wrap but must not make the right pane taller", i))
	}

	view := model.View()
	assertPanelsShareBottomBorder(t, view)
}

func TestAddressSpaceAndRightPaneStaySameHeightWithLongWatchlist(t *testing.T) {
	model := NewModel(Dependencies{})
	model.connectionModalOpen = false
	model.focus = focusWatchlist
	model.rightPane = rightPaneWatchlist
	for i := 0; i < 30; i++ {
		model.inspections.Watch(opcua.AddressNode{NodeID: fmt.Sprintf("ns=2;s=VeryLongWatchlistNodeIdentifier%d", i), DisplayName: fmt.Sprintf("VeryLongWatchlistNodeDisplayName%d", i), NodeClass: "Variable"})
	}

	view := model.View()
	assertPanelsShareBottomBorder(t, view)
}

func assertPanelsShareBottomBorder(t *testing.T, view string) {
	t.Helper()
	for _, line := range strings.Split(view, "\n") {
		if strings.Count(line, "╰") == 2 && strings.Count(line, "╯") == 2 {
			return
		}
	}
	t.Fatalf("expected Address Space and right pane bottom borders on the same line:\n%s", view)
}

func TestArrowKeysDoNotControlAddressSpaceWhenNodeDetailsFocused(t *testing.T) {
	client := &fakeClient{
		children:      map[string][]opcua.AddressNode{"i=85": {{NodeID: "ns=2;s=Temperature", DisplayName: "Temperature", NodeClass: "Variable"}}},
		subscriptions: map[string]chan opcua.LiveValue{"ns=2;s=Temperature": make(chan opcua.LiveValue, 1)},
	}
	model := NewModel(Dependencies{Client: client})
	markModelConnected(&model)
	model.connectionModalOpen = false
	model.addressSpace = &AddressSpace{tree: []treeNode{
		{node: opcua.AddressNode{NodeID: "i=85", DisplayName: "Objects", NodeClass: "Object"}},
		{node: opcua.AddressNode{NodeID: "ns=2;s=Temperature", DisplayName: "Temperature", NodeClass: "Variable"}, depth: 1},
	}}
	model.focus = focusDetails
	model.rightPane = rightPaneDetails
	model.selectedTree = 0

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)
	if cmd != nil {
		t.Fatal("expected Down to issue no Address Space command while Node Details is focused")
	}
	if model.selectedTree != 0 {
		t.Fatalf("expected Down to leave Address Space selection put, selectedTree=%d", model.selectedTree)
	}

	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRight})
	model = updated.(Model)
	if cmd != nil {
		t.Fatal("expected Right to issue no Address Space command while Node Details is focused")
	}
	if model.addressSpace.tree[0].expanded {
		t.Fatal("expected Right to leave Address Space node collapsed while Node Details is focused")
	}
}

func TestOverlayCenteredPreservesUnderlyingPanelEdges(t *testing.T) {
	base := strings.Join([]string{
		"┌──────────────────┐",
		"│                  │",
		"│                  │",
		"└──────────────────┘",
	}, "\n")
	overlay := "╔════╗\n║ OK ║\n╚════╝"

	view := overlayCentered(base, overlay, 20, 1)
	lines := strings.Split(view, "\n")
	for _, lineIndex := range []int{1, 2} {
		if !strings.HasPrefix(lines[lineIndex], "│") || !strings.HasSuffix(lines[lineIndex], "│") {
			t.Fatalf("expected overlay to preserve panel edges on line %d:\n%s", lineIndex, view)
		}
	}
	if !strings.Contains(lines[1], "╔════╗") || !strings.Contains(lines[2], "║ OK ║") {
		t.Fatalf("expected overlay content centered over base:\n%s", view)
	}
}

func TestConnectionModalEndpointSelectionFitsAvailableHeight(t *testing.T) {
	model := NewModel(Dependencies{})
	endpoints := longSecureEndpointList()
	model.connection.ApplyDiscovery(endpoints, nil)
	for model.connection.View().SelectedEndpoint != 4 {
		model.connection.MoveEndpointSelection(1)
	}

	view := model.connectionModalView(120, 16)
	lines := strings.Split(view, "\n")
	if len(lines) > 16 {
		t.Fatalf("expected endpoint selection modal to fit available height, got %d lines:\n%s", len(lines), view)
	}
	if !strings.Contains(view, "↓") && !strings.Contains(view, "↑") {
		t.Fatalf("expected truncated endpoint list to show scroll affordance:\n%s", view)
	}
}

func TestConnectionModalEndpointSelectionWithErrorFitsAvailableHeight(t *testing.T) {
	model := NewModel(Dependencies{Launch: app.LaunchOptions{Endpoint: "opc.tcp://localhost:4840"}})
	endpoints := longSecureEndpointList()
	for i := range endpoints {
		endpoints[i].UserTokenTypes = []string{"Anonymous"}
	}
	model.connection.ApplyDiscovery(endpoints, nil)
	for model.connection.View().SelectedEndpoint != 4 {
		model.connection.MoveEndpointSelection(1)
	}
	model.connection.Submit()
	model.connection.SelectAuthType(0)

	view := model.connectionModalView(120, 16)
	lines := strings.Split(view, "\n")
	if len(lines) > 16 {
		t.Fatalf("expected endpoint selection modal with error to fit available height, got %d lines:\n%s", len(lines), view)
	}
	if strings.Contains(view, "secure endpoint requires client certificate") || strings.Contains(view, "Error:") {
		t.Fatalf("expected connection error to be omitted from modal body:\n%s", view)
	}
}

func longSecureEndpointList() []opcua.Endpoint {
	endpoints := make([]opcua.Endpoint, 0, 7)
	for i := 0; i < 7; i++ {
		endpoints = append(endpoints, opcua.Endpoint{
			SecurityMode:     "SignAndEncrypt",
			SecurityPolicy:   "Basic256Sha256",
			UserTokenTypes:   []string{"Anonymous", "UserName", "Certificate"},
			SecurityLevel:    uint8(100 + i),
			ServerThumbprint: "B0448DB3ABFBE3CA1E1C60D97ED4D35A7E1A7704",
		})
	}
	return endpoints
}

func TestConnectionModalDiscoversEnteredEndpoint(t *testing.T) {
	client := &fakeClient{endpoints: []opcua.Endpoint{{SecurityMode: "None", SecurityPolicy: "None", UserTokenTypes: []string{"Anonymous"}}}}
	model := NewModel(Dependencies{Client: client})

	updated := tea.Model(model)
	for _, r := range "opc.tcp://localhost:4840" {
		updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	updated, cmd := updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected endpoint discovery command")
	}

	msg := cmd()
	if _, ok := msg.(endpointDiscoveryMsg); !ok {
		t.Fatalf("expected endpointDiscoveryMsg, got %T", msg)
	}
	if client.discovered != "opc.tcp://localhost:4840" {
		t.Fatalf("discovered endpoint = %q", client.discovered)
	}
}

func TestConnectionModalConnectsEnteredEndpoint(t *testing.T) {
	client := &fakeClient{}
	model := NewModel(Dependencies{Client: client})
	model.connectionInput.SetValue("opc.tcp://entered:4840")
	model.connection.SetEndpointText("opc.tcp://entered:4840")
	updated, _ := model.Update(endpointDiscoveryMsg{Endpoints: []opcua.Endpoint{{
		SecurityMode:   "None",
		SecurityPolicy: "None",
		UserTokenTypes: []string{"Anonymous"},
	}}})

	_, cmd := updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected connect command")
	}
	msg := cmd()
	if _, ok := msg.(endpointConnectionMsg); !ok {
		t.Fatalf("expected endpointConnectionMsg, got %T", msg)
	}
	if client.connected.Endpoint != "opc.tcp://entered:4840" {
		t.Fatalf("connected endpoint = %q", client.connected.Endpoint)
	}
}

func TestConnectionModalCanDismissAndReopenWhenDisconnected(t *testing.T) {
	model := NewModel(Dependencies{})

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	view := updated.(Model).View()
	if strings.Contains(view, "Connect to OPC UA Server") {
		t.Fatalf("expected Connection Modal to close on Esc:\n%s", view)
	}
	if !strings.Contains(view, "Open Connection Modal to connect") {
		t.Fatalf("expected unconnected shell to explain how to connect:\n%s", view)
	}

	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	view = updated.(Model).View()
	if !strings.Contains(view, "Connect to OPC UA Server") {
		t.Fatalf("expected c to reopen Connection Modal when disconnected:\n%s", view)
	}
}

func TestHelpToggle(t *testing.T) {
	model := NewModel(Dependencies{})
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	view := updated.(Model).View()

	if !strings.Contains(view, "Export Diagnostics Bundle") || !strings.Contains(view, "Open exports folder") {
		t.Fatalf("expected help view:\n%s", view)
	}
}

func TestSnapshotExportDoesNotCreateEmptySnapshot(t *testing.T) {
	exportDir := t.TempDir()
	model := NewModel(Dependencies{Paths: config.Paths{CacheDir: exportDir}})
	model.connectionModalOpen = false

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	view := updated.(Model).View()
	if !strings.Contains(view, "no watched Variable Nodes to export") {
		t.Fatalf("expected empty Watchlist export status:\n%s", view)
	}
	visibleToasts := updated.(Model).toasts.Visible()
	if len(visibleToasts) != 1 || visibleToasts[0].Kind != toast.KindError || !strings.Contains(visibleToasts[0].Message, "no watched Variable Nodes") {
		t.Fatalf("expected error toast for empty Snapshot export, got %#v", visibleToasts)
	}
	files, err := filepath.Glob(filepath.Join(exportDir, "exports", "snapshot-*.md"))
	if err != nil || len(files) != 0 {
		t.Fatalf("expected no Snapshot files, files=%#v err=%v", files, err)
	}
}

func TestSnapshotExportWritesCurrentWatchlist(t *testing.T) {
	exportDir := t.TempDir()
	model := NewModel(Dependencies{Paths: config.Paths{CacheDir: exportDir}})
	updated, _ := model.Update(endpointConnectionMsg{Request: opcua.ConnectRequest{Endpoint: "opc.tcp://server:4840", SecurityMode: "None", SecurityPolicy: "None", AuthType: opcua.AuthUsername, Username: "engineer", Password: "secret-password"}})
	model = updated.(Model)
	model.connectionModalOpen = false
	model.inspections.Watch(opcua.AddressNode{NodeID: "ns=2;s=Temperature", DisplayName: "Temperature", NodeClass: "Variable"})
	model.inspections.ApplyDetails("ns=2;s=Temperature", opcua.NodeDetails{NodeID: "ns=2;s=Temperature", EURange: &opcua.ValueRange{Low: 0, High: 100}}, nil)
	model.inspections.ApplyLiveValue("ns=2;s=Temperature", opcua.LiveValue{
		NodeID:          "ns=2;s=Temperature",
		Value:           "120",
		Status:          "StatusOK",
		SourceTimestamp: time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC),
		ServerTimestamp: time.Date(2026, 6, 6, 12, 0, 1, 0, time.UTC),
	}, nil)

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	view := updated.(Model).View()
	if !strings.Contains(view, "Snapshot exported") {
		t.Fatalf("expected successful Snapshot export status:\n%s", view)
	}
	visibleToasts := updated.(Model).toasts.Visible()
	if !hasToast(visibleToasts, toast.KindSuccess, "Press o to open the exports folder") {
		t.Fatalf("expected success toast with next action for Snapshot export, got %#v", visibleToasts)
	}

	files, err := filepath.Glob(filepath.Join(exportDir, "exports", "snapshot-*.md"))
	if err != nil || len(files) != 1 {
		t.Fatalf("expected one Snapshot file, files=%#v err=%v", files, err)
	}
	contentBytes, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	content := string(contentBytes)
	for _, expected := range []string{"# Watchlist Snapshot", "node names, process values, endpoints, and server metadata may be sensitive", "opc.tcp://server:4840", "None", "UserName", "Temperature", "ns=2;s=Temperature", "120", "StatusOK", "2026-06-06T12:00:00Z", "2026-06-06T12:00:01Z", "Out-of-Range: 120 is above 100"} {
		if !strings.Contains(content, expected) {
			t.Fatalf("expected %q in Snapshot:\n%s", expected, content)
		}
	}
	if strings.Contains(content, "secret-password") {
		t.Fatalf("Snapshot leaked password:\n%s", content)
	}
}

func TestDiagnosticsBundleExportWritesCurrentSessionState(t *testing.T) {
	exporter := &fakeDiagnosticsExporter{path: "diagnostics.md"}
	log := NewDiagnosticLog(10)
	log.now = func() time.Time { return time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC) }
	model := NewModel(Dependencies{
		Paths:               config.Paths{ConfigDir: "config-dir", CacheDir: "cache-dir"},
		DiagnosticsExporter: exporter,
		DiagnosticLog:       log,
	})
	model.connectionModalOpen = false
	request := opcua.ConnectRequest{Endpoint: "opc.tcp://server:4840", SecurityMode: "None", SecurityPolicy: "None", AuthType: opcua.AuthUsername, Username: "engineer", Password: "secret-password"}
	model.connection.ApplyConnection(request, nil)
	model.connectedRequest = request
	model.statusLine = "Read-Only Mode · selected Live Value updated · Temperature"
	model.diagnosticsLog.Add("Subscription restored for ns=2;s=Temperature")

	node := opcua.AddressNode{NodeID: "ns=2;s=Temperature", DisplayName: "Temperature", NodeClass: "Variable"}
	model.inspections.Select(node)
	model.inspections.Watch(node)
	model.inspections.ApplyLiveValue(node.NodeID, opcua.LiveValue{NodeID: node.NodeID, Value: "42", Status: "StatusOK"}, nil)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	view := updated.(Model).View()
	if !strings.Contains(view, "Diagnostics Bundle exported") {
		t.Fatalf("expected successful Diagnostics Bundle export status:\n%s", view)
	}
	visibleToasts := updated.(Model).toasts.Visible()
	if !hasToast(visibleToasts, toast.KindSuccess, "Press o to open the exports folder") {
		t.Fatalf("expected success toast with next action for Diagnostics Bundle export, got %#v", visibleToasts)
	}
	content := exporter.content
	for _, expected := range []string{"# Diagnostics Bundle", "endpoints, node names, process values", "opc.tcp://server:4840", "None", "UserName", "Connected", "Temperature", "ns=2;s=Temperature", "42", "StatusOK", "Read-Only Mode · selected Live Value updated", "config-dir", "cache-dir", "Subscription restored"} {
		if !strings.Contains(content, expected) {
			t.Fatalf("expected %q in Diagnostics Bundle:\n%s", expected, content)
		}
	}
	if strings.Contains(content, "secret-password") {
		t.Fatalf("Diagnostics Bundle leaked password:\n%s", content)
	}
}

func TestDiagnosticsBundleExportFailureIsVisible(t *testing.T) {
	model := NewModel(Dependencies{DiagnosticsExporter: &fakeDiagnosticsExporter{err: fmt.Errorf("disk full")}})
	model.connectionModalOpen = false

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	view := updated.(Model).View()
	if !strings.Contains(view, "Diagnostics Bundle export failed: disk full") {
		t.Fatalf("expected failed Diagnostics Bundle export status:\n%s", view)
	}
	visibleToasts := updated.(Model).toasts.Visible()
	if !hasToast(visibleToasts, toast.KindError, "disk full") {
		t.Fatalf("expected error toast for Diagnostics Bundle export, got %#v", visibleToasts)
	}
}

func TestOpenExportsFolderShortcutOpensConfiguredExportsDirectory(t *testing.T) {
	opener := &fakeExportFolderOpener{}
	cacheDir := t.TempDir()
	model := NewModel(Dependencies{Paths: config.Paths{CacheDir: cacheDir}, ExportFolderOpener: opener})
	model.connectionModalOpen = false

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	view := updated.(Model).View()
	expectedPath := filepath.Join(cacheDir, "exports")
	if opener.path != expectedPath {
		t.Fatalf("opened path = %q, want %q", opener.path, expectedPath)
	}
	if !strings.Contains(view, "opened exports folder") {
		t.Fatalf("expected opened exports folder status:\n%s", view)
	}
	if _, err := os.Stat(expectedPath); err != nil {
		t.Fatalf("expected exports folder created: %v", err)
	}
	visibleToasts := updated.(Model).toasts.Visible()
	if !hasToast(visibleToasts, toast.KindSuccess, "Exports folder opened") {
		t.Fatalf("expected success toast for open exports folder, got %#v", visibleToasts)
	}
}

func TestOpenExportsFolderFailureIsVisible(t *testing.T) {
	model := NewModel(Dependencies{ExportFolderOpener: &fakeExportFolderOpener{err: fmt.Errorf("no opener")}})
	model.connectionModalOpen = false

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	view := updated.(Model).View()
	if !strings.Contains(view, "Open exports folder failed: no opener") {
		t.Fatalf("expected open exports folder failure status:\n%s", view)
	}
	visibleToasts := updated.(Model).toasts.Visible()
	if !hasToast(visibleToasts, toast.KindError, "no opener") {
		t.Fatalf("expected error toast for open exports folder, got %#v", visibleToasts)
	}
}

func hasToast(toasts []toast.Toast, kind toast.Kind, messageFragment string) bool {
	for _, item := range toasts {
		if item.Kind == kind && strings.Contains(item.Message, messageFragment) {
			return true
		}
	}
	return false
}

func TestEndpointDiscoveryLabelsUsernamePasswordRequirement(t *testing.T) {
	model := NewModel(Dependencies{})
	updated, _ := model.Update(endpointDiscoveryMsg{Endpoints: []opcua.Endpoint{{
		SecurityMode:   "None",
		SecurityPolicy: "None",
		UserTokenTypes: []string{"UserName"},
	}}})
	view := updated.(Model).View()

	if !strings.Contains(view, "username/password") {
		t.Fatalf("expected username/password capability label:\n%s", view)
	}
}

func TestEndpointDiscoveryLabelsSecureEndpointConnectableWhenCertificateConfigured(t *testing.T) {
	model := NewModel(Dependencies{Launch: app.LaunchOptions{ClientCertificatePath: "cert.pem", ClientPrivateKeyPath: "key.pem"}})
	updated, _ := model.Update(endpointDiscoveryMsg{Endpoints: []opcua.Endpoint{{
		SecurityMode:   "Sign",
		SecurityPolicy: "Basic256Sha256",
		UserTokenTypes: []string{"Anonymous"},
	}}})
	view := updated.(Model).View()

	if !strings.Contains(view, "connectable") {
		t.Fatalf("expected connectable capability label:\n%s", view)
	}
	if strings.Contains(view, "cert/key") {
		t.Fatalf("did not expect cert/key requirement when configured:\n%s", view)
	}
}

func TestEndpointDiscoveryLabelsSecureEndpointCertificateRequirement(t *testing.T) {
	model := NewModel(Dependencies{})
	updated, _ := model.Update(endpointDiscoveryMsg{Endpoints: []opcua.Endpoint{{
		SecurityMode:   "Sign",
		SecurityPolicy: "Basic256Sha256",
		UserTokenTypes: []string{"Anonymous"},
	}}})
	view := updated.(Model).View()

	if !strings.Contains(view, "cert/key") {
		t.Fatalf("expected client cert/key capability label:\n%s", view)
	}
}

func TestEndpointDiscoveryLabelsUnsupportedAuthentication(t *testing.T) {
	model := NewModel(Dependencies{})
	updated, _ := model.Update(endpointDiscoveryMsg{Endpoints: []opcua.Endpoint{{
		SecurityMode:   "None",
		SecurityPolicy: "None",
		UserTokenTypes: []string{"Certificate"},
	}}})
	view := updated.(Model).View()

	if !strings.Contains(view, "unsupported auth") {
		t.Fatalf("expected unsupported auth capability label:\n%s", view)
	}
}

func TestEndpointDiscoveryDoesNotReplaceNodeDetails(t *testing.T) {
	model := NewModel(Dependencies{})
	updated, _ := model.Update(endpointDiscoveryMsg{Endpoints: []opcua.Endpoint{{
		SecurityMode:   "None",
		SecurityPolicy: "None",
		UserTokenTypes: []string{"Anonymous"},
	}}})
	view := updated.(Model).View()
	details := strings.Join(updated.(Model).details, "\n")

	if !strings.Contains(view, "discovered 1 endpoint") {
		t.Fatalf("expected discovery status:\n%s", view)
	}
	if !strings.Contains(view, "None · None · Anonymous") {
		t.Fatalf("expected endpoint selection in connection modal:\n%s", view)
	}
	if strings.Contains(details, "endpoint") || strings.Contains(details, "Endpoint") {
		t.Fatalf("expected Node Details to stay node-only, got:\n%s", details)
	}
	if !strings.Contains(view, "Enter connect") {
		t.Fatalf("expected endpoint selection footer:\n%s", view)
	}
}

func TestEndpointSelectionMovesAndConnects(t *testing.T) {
	client := &fakeClient{}
	model := NewModel(Dependencies{Client: client, Launch: app.LaunchOptions{Endpoint: "opc.tcp://localhost:4840", ClientCertificatePath: "cert.pem", ClientPrivateKeyPath: "key.pem"}})
	updated, _ := model.Update(endpointDiscoveryMsg{Endpoints: []opcua.Endpoint{
		{SecurityMode: "Sign", SecurityPolicy: "Basic256Sha256", UserTokenTypes: []string{"Anonymous"}},
		{SecurityMode: "None", SecurityPolicy: "None", UserTokenTypes: []string{"Anonymous"}},
	}})

	selected := updated.(Model)
	if selected.connection.View().SelectedEndpoint != 1 {
		t.Fatalf("selected endpoint = %d", selected.connection.View().SelectedEndpoint)
	}

	updated, _ = selected.Update(tea.KeyMsg{Type: tea.KeyDown})
	selected = updated.(Model)
	if selected.connection.View().SelectedEndpoint != 0 {
		t.Fatalf("selected endpoint after down = %d", selected.connection.View().SelectedEndpoint)
	}

	updated, cmd := selected.Update(tea.KeyMsg{Type: tea.KeyEnter})
	selected = updated.(Model)
	if cmd == nil {
		t.Fatal("expected connect command")
	}
	if !selected.connection.View().Connecting {
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
	if client.connected.ClientCertificatePath != "cert.pem" || client.connected.ClientPrivateKeyPath != "key.pem" {
		t.Fatalf("cert/key paths = %q / %q", client.connected.ClientCertificatePath, client.connected.ClientPrivateKeyPath)
	}
}

func TestSuccessfulConnectionClosesConnectionModal(t *testing.T) {
	model := NewModel(Dependencies{Client: &fakeClient{}})

	updated, cmd := model.Update(endpointConnectionMsg{Request: opcua.ConnectRequest{SecurityMode: "None", SecurityPolicy: "None"}})
	if cmd == nil {
		t.Fatal("expected browse command after connection")
	}
	view := updated.(Model).View()
	if strings.Contains(view, "Connect to OPC UA Server") {
		t.Fatalf("expected Connection Modal to close after connection:\n%s", view)
	}
	if !strings.Contains(view, "Loading Objects Address Space") {
		t.Fatalf("expected Address Space browsing status after connection:\n%s", view)
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
	var browse browseChildrenMsg
	foundBrowse := false
	for _, msg := range runCmds(cmd) {
		if candidate, ok := msg.(browseChildrenMsg); ok {
			browse = candidate
			foundBrowse = true
			break
		}
	}
	if !foundBrowse {
		t.Fatalf("expected browseChildrenMsg")
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
	markModelConnected(&model)
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
	markModelConnected(&model)
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
	markModelConnected(&model)
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
	watched := model.inspections.Watched()
	if len(watched) != 1 || !watched[0].Subscribing {
		t.Fatalf("watchlist = %#v", watched)
	}

	var waitCmd tea.Cmd
	for _, msg := range runCmds(cmd) {
		var nextCmd tea.Cmd
		updated, nextCmd = model.Update(msg)
		model = updated.(Model)
		if nextCmd != nil {
			waitCmd = nextCmd
		}
	}
	if waitCmd == nil {
		t.Fatal("expected wait-for-value command")
	}

	updates <- opcua.LiveValue{NodeID: "ns=2;s=Temperature", Value: "42.5", Status: "OK"}
	updated, _ = model.Update(waitCmd())
	model = updated.(Model)
	view := model.View()
	if !strings.Contains(view, "Watchlist (1)") || strings.Contains(view, "42.5 · OK") {
		t.Fatalf("expected Watchlist count without switching tabs:\n%s", view)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyTab})
	view = updated.(Model).View()
	if !strings.Contains(view, "Temperature") || !strings.Contains(view, "42.5 · OK") {
		t.Fatalf("expected subscribed Live Value in Watchlist tab:\n%s", view)
	}
}

func TestSelectedVariableNodeSubscribesLiveValueIntoDetails(t *testing.T) {
	updates := make(chan opcua.LiveValue, 1)
	client := &fakeClient{subscriptions: map[string]chan opcua.LiveValue{"ns=2;s=Pressure": updates}}
	model := NewModel(Dependencies{Client: client})
	markModelConnected(&model)
	model.addressSpace = &AddressSpace{tree: []treeNode{
		{node: opcua.AddressNode{NodeID: "i=85", DisplayName: "Objects", NodeClass: "Object"}, expanded: true, childrenLoaded: true},
		{node: opcua.AddressNode{NodeID: "ns=2;s=Pressure", DisplayName: "Pressure", NodeClass: "Variable"}, depth: 1},
	}}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	if cmd == nil {
		t.Fatal("expected selected-node subscribe command")
	}
	model = updated.(Model)
	selected, ok := model.inspections.Selected()
	if !ok || !selected.Subscribing || selected.Node.NodeID != "ns=2;s=Pressure" {
		t.Fatalf("selected value = %#v, ok=%t", selected, ok)
	}

	var waitCmd tea.Cmd
	for _, msg := range runCmds(cmd) {
		updated, waitCmd = model.Update(msg)
		model = updated.(Model)
		if waitCmd != nil {
			break
		}
	}
	if waitCmd == nil {
		t.Fatal("expected wait-for-selected-value command")
	}

	updates <- opcua.LiveValue{NodeID: "ns=2;s=Pressure", Value: "12.3", Status: "StatusOK"}
	updated, _ = model.Update(waitCmd())
	view := updated.(Model).View()
	if !strings.Contains(view, "Pressure") || !strings.Contains(view, "Value") || !strings.Contains(view, "12.3") || !strings.Contains(view, "Health") {
		t.Fatalf("expected selected Live Value in details:\n%s", view)
	}
}

func TestSelectedVariableNodeShowsMetadataAndOutOfRange(t *testing.T) {
	updates := make(chan opcua.LiveValue, 1)
	client := &fakeClient{
		subscriptions: map[string]chan opcua.LiveValue{"ns=2;s=Level": updates},
		details: map[string]opcua.NodeDetails{"ns=2;s=Level": {
			DataType:        "Double",
			AccessLevel:     "CurrentRead, CurrentWrite",
			Writable:        true,
			EngineeringUnit: "%",
			EURange:         &opcua.ValueRange{Low: 0, High: 100},
		}},
	}
	model := NewModel(Dependencies{Client: client})
	markModelConnected(&model)
	model.addressSpace = &AddressSpace{tree: []treeNode{
		{node: opcua.AddressNode{NodeID: "i=85", DisplayName: "Objects", NodeClass: "Object"}, expanded: true, childrenLoaded: true},
		{node: opcua.AddressNode{NodeID: "ns=2;s=Level", DisplayName: "Level", NodeClass: "Variable"}, depth: 1},
	}}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)
	var waitCmd tea.Cmd
	for _, msg := range runCmds(cmd) {
		var nextCmd tea.Cmd
		updated, nextCmd = model.Update(msg)
		model = updated.(Model)
		if nextCmd != nil {
			waitCmd = nextCmd
		}
	}
	if waitCmd == nil {
		t.Fatal("expected wait-for-selected-value command")
	}
	updates <- opcua.LiveValue{NodeID: "ns=2;s=Level", Value: "120", Status: "StatusOK"}
	updated, _ = model.Update(waitCmd())
	details := strings.Join(updated.(Model).details, "\n")

	for _, expected := range []string{"Data Type", "Double", "Engineering Unit", "%", "EURange", "0…100", "Out-of-Range", "120 is above 100", "Read-Only Mode prevents writes"} {
		if !strings.Contains(details, expected) {
			t.Fatalf("expected %q in selected node details:\n%s", expected, details)
		}
	}
}

func TestSelectingNonVariableCancelsSelectedLiveValue(t *testing.T) {
	sub := &fakeSubscription{}
	model := NewModel(Dependencies{Client: &fakeClient{}})
	markModelConnected(&model)
	model.addressSpace = &AddressSpace{tree: []treeNode{
		{node: opcua.AddressNode{NodeID: "i=85", DisplayName: "Objects", NodeClass: "Object"}, expanded: true, childrenLoaded: true},
	}}
	model.inspections.Select(opcua.AddressNode{NodeID: "ns=2;s=Flow", DisplayName: "Flow", NodeClass: "Variable"})
	model.inspections.ApplySubscription("ns=2;s=Flow", make(chan opcua.LiveValue), sub, nil)

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)
	if selected, ok := model.inspections.Selected(); ok || selected.Node.NodeID != "" {
		t.Fatalf("expected selected Live Value cleared, got %#v", selected)
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
	model.connectionModalOpen = false
	model.height = 12
	model.focus = focusWatchlist
	model.rightPane = rightPaneWatchlist
	for i := 0; i < 9; i++ {
		model.inspections.Watch(opcua.AddressNode{NodeID: fmt.Sprintf("ns=2;s=Node%d", i), DisplayName: fmt.Sprintf("Node%d", i), NodeClass: "Variable"})
	}

	updated := tea.Model(model)
	for i := 0; i < 7; i++ {
		updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	model = updated.(Model)
	lines := strings.Join(model.watchlistLines(model.mainPanelHeight()), "\n")

	if model.watchScroll == 0 {
		t.Fatal("expected Watchlist to scroll")
	}
	if !strings.Contains(lines, "Node7") || strings.Contains(lines, "Node0 =") {
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

type fakeExportFolderOpener struct {
	path string
	err  error
}

func (f *fakeExportFolderOpener) OpenExportFolder(path string) error {
	f.path = path
	return f.err
}

type fakeDiagnosticsExporter struct {
	path    string
	content string
	err     error
}

func (f *fakeDiagnosticsExporter) ExportDiagnostics(markdown string) (string, error) {
	f.content = markdown
	if f.err != nil {
		return "", f.err
	}
	return f.path, nil
}

type fakeClient struct {
	discovered    string
	endpoints     []opcua.Endpoint
	connected     opcua.ConnectRequest
	children      map[string][]opcua.AddressNode
	details       map[string]opcua.NodeDetails
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

func (f *fakeClient) ReadNodeDetails(ctx context.Context, nodeID string) (opcua.NodeDetails, error) {
	return f.details[nodeID], nil
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

func TestWizardShowsAuthTypeSelectionForMultiAuthEndpoint(t *testing.T) {
	client := &fakeClient{}
	model := NewModel(Dependencies{Client: client})
	model.connectionInput.SetValue("opc.tcp://localhost:4840")
	model.connection.SetEndpointText("opc.tcp://localhost:4840")
	updated, _ := model.Update(endpointDiscoveryMsg{Endpoints: []opcua.Endpoint{{
		SecurityMode:   "None",
		SecurityPolicy: "None",
		UserTokenTypes: []string{"Anonymous", "UserName"},
	}}})

	// Press Enter to select the endpoint
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	view := updated.(Model).View()

	if !strings.Contains(view, "Authentication") {
		t.Fatalf("expected auth selection step in view:\n%s", view)
	}
	if !strings.Contains(view, "Anonymous") || !strings.Contains(view, "UserName") {
		t.Fatalf("expected auth options in view:\n%s", view)
	}
}

func TestWizardConnectsAnonymouslyThroughFullFlow(t *testing.T) {
	client := &fakeClient{}
	model := NewModel(Dependencies{Client: client})
	model.connectionInput.SetValue("opc.tcp://localhost:4840")
	model.connection.SetEndpointText("opc.tcp://localhost:4840")
	updated, _ := model.Update(endpointDiscoveryMsg{Endpoints: []opcua.Endpoint{{
		SecurityMode:   "None",
		SecurityPolicy: "None",
		UserTokenTypes: []string{"Anonymous", "UserName"},
	}}})

	// Select endpoint → auth selection
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	// Select Anonymous (index 0) → connect
	_, cmd := updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected connect command after selecting Anonymous")
	}
	msg := cmd()
	if _, ok := msg.(endpointConnectionMsg); !ok {
		t.Fatalf("expected endpointConnectionMsg, got %T", msg)
	}
	if client.connected.AuthType != opcua.AuthAnonymous {
		t.Fatalf("auth type = %v, expected Anonymous", client.connected.AuthType)
	}
}

func TestWizardShowsCredentialFormForUserNameAuth(t *testing.T) {
	client := &fakeClient{}
	model := NewModel(Dependencies{Client: client})
	model.connectionInput.SetValue("opc.tcp://localhost:4840")
	model.connection.SetEndpointText("opc.tcp://localhost:4840")
	updated, _ := model.Update(endpointDiscoveryMsg{Endpoints: []opcua.Endpoint{{
		SecurityMode:   "None",
		SecurityPolicy: "None",
		UserTokenTypes: []string{"Anonymous", "UserName"},
	}}})

	// Select endpoint → auth selection
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	// Move down to UserName
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyDown})
	// Select UserName → credential form
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	view := updated.(Model).View()

	if !strings.Contains(view, "Username") {
		t.Fatalf("expected credential form in view:\n%s", view)
	}
}

func TestWizardRejectsUnsupportedCertificateAuth(t *testing.T) {
	client := &fakeClient{}
	model := NewModel(Dependencies{Client: client})
	model.connectionInput.SetValue("opc.tcp://localhost:4840")
	model.connection.SetEndpointText("opc.tcp://localhost:4840")
	updated, _ := model.Update(endpointDiscoveryMsg{Endpoints: []opcua.Endpoint{{
		SecurityMode:   "None",
		SecurityPolicy: "None",
		UserTokenTypes: []string{"Anonymous", "Certificate"},
	}}})

	// Select endpoint → auth selection
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	// Move to Certificate, select it
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyDown})
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})

	view := updated.(Model).View()
	if !strings.Contains(view, "unsupported authentication") || !strings.Contains(view, "Certificate") {
		t.Fatalf("expected unsupported auth error in view:\n%s", view)
	}
	if !strings.Contains(view, "Authentication Options") {
		t.Fatalf("expected auth options to remain visible after unsupported auth:\n%s", view)
	}
	state := updated.(Model).connection.View()
	if !state.HasAuthTypeSelection {
		t.Fatalf("expected auth selection to remain active after unsupported auth, view=%#v", state)
	}

	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyUp})
	state = updated.(Model).connection.View()
	if state.SelectedAuthType != 0 {
		t.Fatalf("selected auth type = %d, expected arrow key to move back to Anonymous", state.SelectedAuthType)
	}
}

func TestWizardConnectsWithCredentialsForUserNameOnlyEndpoint(t *testing.T) {
	client := &fakeClient{}
	model := NewModel(Dependencies{Client: client})
	model.connectionInput.SetValue("opc.tcp://localhost:4840")
	model.connection.SetEndpointText("opc.tcp://localhost:4840")
	updated, _ := model.Update(endpointDiscoveryMsg{Endpoints: []opcua.Endpoint{{
		SecurityMode:   "None",
		SecurityPolicy: "None",
		UserTokenTypes: []string{"UserName"},
	}}})

	// Select username-only endpoint → credential form
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !strings.Contains(updated.(Model).View(), "Username") {
		t.Fatalf("expected credential form in view:\n%s", updated.(Model).View())
	}

	for _, r := range "operator" {
		updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyTab})
	for _, r := range "secret" {
		updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	_, cmd := updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected connect command after submitting credentials")
	}
	msg := cmd()
	if _, ok := msg.(endpointConnectionMsg); !ok {
		t.Fatalf("expected endpointConnectionMsg, got %T", msg)
	}
	if client.connected.AuthType != opcua.AuthUsername {
		t.Fatalf("auth type = %v, expected UserName", client.connected.AuthType)
	}
	if client.connected.Username != "operator" || client.connected.Password != "secret" {
		t.Fatalf("credentials = %q / %q", client.connected.Username, client.connected.Password)
	}
}

func TestWizardConnectsWithCredentialsThroughFullFlow(t *testing.T) {
	client := &fakeClient{}
	model := NewModel(Dependencies{Client: client})
	model.connectionInput.SetValue("opc.tcp://localhost:4840")
	model.connection.SetEndpointText("opc.tcp://localhost:4840")
	updated, _ := model.Update(endpointDiscoveryMsg{Endpoints: []opcua.Endpoint{{
		SecurityMode:   "None",
		SecurityPolicy: "None",
		UserTokenTypes: []string{"Anonymous", "UserName"},
	}}})

	// Select endpoint → auth selection
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	// Move to UserName, select it
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyDown})
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Type username
	for _, r := range "admin" {
		updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	// Tab to password
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyTab})
	// Type password
	for _, r := range "secret" {
		updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	// Submit credentials
	_, cmd := updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected connect command after submitting credentials")
	}
	msg := cmd()
	if _, ok := msg.(endpointConnectionMsg); !ok {
		t.Fatalf("expected endpointConnectionMsg, got %T", msg)
	}
	if client.connected.AuthType != opcua.AuthUsername {
		t.Fatalf("auth type = %v, expected UserName", client.connected.AuthType)
	}
	if client.connected.Username != "admin" || client.connected.Password != "secret" {
		t.Fatalf("credentials = %q / %q", client.connected.Username, client.connected.Password)
	}
}

func TestWizardEscapeFromAuthSelectionGoesBackToEndpoints(t *testing.T) {
	model := NewModel(Dependencies{})
	model.connectionInput.SetValue("opc.tcp://localhost:4840")
	model.connection.SetEndpointText("opc.tcp://localhost:4840")
	updated, _ := model.Update(endpointDiscoveryMsg{Endpoints: []opcua.Endpoint{{
		SecurityMode:   "None",
		SecurityPolicy: "None",
		UserTokenTypes: []string{"Anonymous", "UserName"},
	}}})

	// Select endpoint → auth selection
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	// Escape should go back to endpoint list, not close the modal
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEsc})
	view := updated.(Model).View()

	if !strings.Contains(view, "Connect to OPC UA Server") {
		t.Fatalf("expected wizard to stay open after Esc from auth step:\n%s", view)
	}
	if !strings.Contains(view, "None · None") {
		t.Fatalf("expected endpoint list after going back:\n%s", view)
	}
}

func TestWizardConnectionErrorDisplaysInView(t *testing.T) {
	client := &fakeClient{}
	model := NewModel(Dependencies{Client: client})
	model.connectionInput.SetValue("opc.tcp://localhost:4840")
	model.connection.SetEndpointText("opc.tcp://localhost:4840")
	updated, _ := model.Update(endpointDiscoveryMsg{Endpoints: []opcua.Endpoint{{
		SecurityMode:   "None",
		SecurityPolicy: "None",
		UserTokenTypes: []string{"Anonymous"},
	}}})

	// Connect directly (single auth)
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	// Simulate connection error
	updated, _ = updated.(Model).Update(endpointConnectionMsg{
		Request: opcua.ConnectRequest{Endpoint: "opc.tcp://localhost:4840"},
		Err:     fmt.Errorf("BadSecurityChecksFailed"),
	})
	view := updated.(Model).View()

	if !strings.Contains(view, "BadSecurityChecksFailed") {
		t.Fatalf("expected connection error in view:\n%s", view)
	}
}

func TestWizardConnectionErrorFromAuthSelectionKeepsEndpointArrowsUsable(t *testing.T) {
	client := &fakeClient{}
	model := NewModel(Dependencies{Client: client})
	model.connectionInput.SetValue("opc.tcp://localhost:4840")
	model.connection.SetEndpointText("opc.tcp://localhost:4840")
	updated, _ := model.Update(endpointDiscoveryMsg{Endpoints: []opcua.Endpoint{
		{SecurityMode: "None", SecurityPolicy: "None", UserTokenTypes: []string{"Anonymous", "UserName"}},
		{SecurityMode: "Sign", SecurityPolicy: "Basic256Sha256", UserTokenTypes: []string{"Anonymous"}},
	}})

	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter}) // endpoint → auth selection
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter}) // Anonymous → connecting
	updated, _ = updated.(Model).Update(endpointConnectionMsg{
		Request: opcua.ConnectRequest{Endpoint: "opc.tcp://localhost:4840"},
		Err:     fmt.Errorf("BadSecurityChecksFailed"),
	})
	failed := updated.(Model).connection.View()
	if failed.HasAuthTypeSelection {
		t.Fatalf("expected failed connection to leave auth selection, view=%#v", failed)
	}

	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyDown})
	selected := updated.(Model).connection.View().SelectedEndpoint
	if selected != 1 {
		t.Fatalf("selected endpoint = %d, expected arrow key to move endpoint selection after failure", selected)
	}
}

func TestWizardEndpointWithNoAuthDoesNotFreezeEndpointSelection(t *testing.T) {
	client := &fakeClient{}
	model := NewModel(Dependencies{Client: client})
	model.connectionInput.SetValue("opc.tcp://localhost:4840")
	model.connection.SetEndpointText("opc.tcp://localhost:4840")
	updated, _ := model.Update(endpointDiscoveryMsg{Endpoints: []opcua.Endpoint{
		{SecurityMode: "None", SecurityPolicy: "None", UserTokenTypes: nil},
		{SecurityMode: "None", SecurityPolicy: "None", UserTokenTypes: []string{"Anonymous"}},
	}})

	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyUp})
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(Model).connection.View()
	if !state.HasEndpointSelection {
		t.Fatalf("expected endpoint selection to remain available after no-auth endpoint error, view=%#v", state)
	}

	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyDown})
	selected := updated.(Model).connection.View().SelectedEndpoint
	if selected != 1 {
		t.Fatalf("selected endpoint = %d, expected arrow key to move after no-auth endpoint error", selected)
	}
}

func TestWizardConnectionErrorDoesNotReplaceNodeDetails(t *testing.T) {
	client := &fakeClient{}
	model := NewModel(Dependencies{Client: client})
	model.connectionInput.SetValue("opc.tcp://localhost:4840")
	model.connection.SetEndpointText("opc.tcp://localhost:4840")
	updated, _ := model.Update(endpointDiscoveryMsg{Endpoints: []opcua.Endpoint{{
		SecurityMode:   "None",
		SecurityPolicy: "None",
		UserTokenTypes: []string{"Anonymous"},
	}}})

	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated, _ = updated.(Model).Update(endpointConnectionMsg{
		Request: opcua.ConnectRequest{Endpoint: "opc.tcp://localhost:4840"},
		Err:     fmt.Errorf("BadSecurityChecksFailed"),
	})

	details := strings.Join(updated.(Model).details, "\n")
	if strings.Contains(details, "Discovered endpoints") || strings.Contains(details, "Connection failed") || strings.Contains(details, "BadSecurityChecksFailed") {
		t.Fatalf("expected Node Details to stay node-only after connection error, got:\n%s", details)
	}
	if !strings.Contains(details, "Select a Variable Node") {
		t.Fatalf("expected default node details, got:\n%s", details)
	}
}
