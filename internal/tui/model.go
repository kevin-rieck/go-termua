package tui

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	toast "github.com/kevin-rieck/go-bubble-toast"

	"termua/internal/app"
	"termua/internal/config"
	"termua/internal/opcua"
	"termua/internal/session"
)

type Dependencies struct {
	Client              opcua.Client
	Paths               config.Paths
	Launch              app.LaunchOptions
	DiagnosticsExporter DiagnosticsExporter
	DiagnosticLog       *DiagnosticLog
	ExportFolderOpener  ExportFolderOpener
}

type focus int

type rightPaneTab int

const (
	focusTree focus = iota
	focusDetails
	focusWatchlist
)

const (
	rightPaneDetails rightPaneTab = iota
	rightPaneWatchlist
)

type endpointDiscoveryMsg struct {
	Endpoints []opcua.Endpoint
	Err       error
}

type endpointConnectionMsg struct {
	Request opcua.ConnectRequest
	Err     error
}

type browseChildrenMsg struct {
	ParentNodeID string
	Children     []opcua.AddressNode
	Err          error
}

type selectedValueSubscribedMsg struct {
	NodeID       string
	Updates      <-chan opcua.LiveValue
	Subscription opcua.ValueSubscription
	Err          error
}

type selectedValueMsg struct {
	NodeID string
	Value  opcua.LiveValue
	Err    error
}

type selectedNodeDetailsMsg struct {
	NodeID  string
	Details opcua.NodeDetails
	Err     error
}

type Model struct {
	client              opcua.Client
	paths               config.Paths
	launch              app.LaunchOptions
	diagnosticsExporter DiagnosticsExporter
	diagnosticsLog      *DiagnosticLog
	exportFolderOpener  ExportFolderOpener

	width               int
	height              int
	focus               focus
	showHelp            bool
	statusLine          string
	connection          ServerConnection
	connectedRequest    opcua.ConnectRequest
	connectionModalOpen bool
	connectionInput     textinput.Model
	usernameInput       textinput.Model
	passwordInput       textinput.Model
	credentialFocus     int
	details             []string
	addressSpace        *AddressSpace
	selectedTree        int
	treeScroll          int
	inspections         *session.InspectionSet
	rightPane           rightPaneTab
	selectedWatch       int
	watchScroll         int
	toasts              toast.Model
}

func NewModel(deps Dependencies) Model {
	status := "Read-Only Mode"
	connectionInput := textinput.New()
	connectionInput.Placeholder = "opc.tcp://host:4840"
	connectionInput.Prompt = ""
	connectionInput.SetValue(deps.Launch.Endpoint)
	connectionInput.Focus()

	usernameInput := textinput.New()
	usernameInput.Placeholder = "Username"
	usernameInput.Prompt = ""

	passwordInput := textinput.New()
	passwordInput.Placeholder = "Password"
	passwordInput.Prompt = ""
	passwordInput.EchoMode = textinput.EchoPassword
	passwordInput.EchoCharacter = '•'

	details := defaultNodeDetailsLines()

	if deps.Launch.Endpoint != "" {
		status = fmt.Sprintf("Read-Only Mode · endpoint %s", deps.Launch.Endpoint)
	}
	if deps.Launch.ConnectionName != "" {
		status = fmt.Sprintf("Read-Only Mode · saved connection %s", deps.Launch.ConnectionName)
	}

	connection := NewServerConnection(deps.Launch.Endpoint)
	connection.SetClientCertificatePaths(deps.Launch.ClientCertificatePath, deps.Launch.ClientPrivateKeyPath)
	diagnosticsExporter := deps.DiagnosticsExporter
	if diagnosticsExporter == nil {
		diagnosticsExporter = newFilesystemDiagnosticsExporter(deps.Paths)
	}
	diagnosticsLog := deps.DiagnosticLog
	if diagnosticsLog == nil {
		diagnosticsLog = NewDiagnosticLog(200)
	}
	diagnosticsLog.Add("TermUA started in Read-Only Mode")
	exportFolderOpener := deps.ExportFolderOpener
	if exportFolderOpener == nil {
		exportFolderOpener = systemExportFolderOpener{}
	}

	return Model{
		client:              deps.Client,
		paths:               deps.Paths,
		launch:              deps.Launch,
		diagnosticsExporter: diagnosticsExporter,
		diagnosticsLog:      diagnosticsLog,
		exportFolderOpener:  exportFolderOpener,
		width:               120,
		height:              30,
		focus:               focusTree,
		statusLine:          status,
		details:             details,
		connection:          connection,
		connectionModalOpen: true,
		connectionInput:     connectionInput,
		usernameInput:       usernameInput,
		passwordInput:       passwordInput,
		addressSpace:        NewAddressSpace(),
		inspections:         session.NewInspectionSet(),
		rightPane:           rightPaneDetails,
		toasts:              toast.New(toast.WithPlacement(toast.TopRight), toast.WithWidth(48), toast.WithMaxVisible(3), toast.WithMaxHeight(6), toast.WithProgress(true)),
	}
}

func (m Model) Init() tea.Cmd {
	if m.launch.Endpoint == "" || m.client == nil {
		return nil
	}
	return discoverEndpointsCmd(m.client, m.launch.Endpoint)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var toastCmd tea.Cmd
	m.toasts, toastCmd = m.toasts.Update(msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		if m.connectionModalActive() {
			updated, cmd := m.updateConnectionModal(msg)
			return updated, tea.Batch(toastCmd, cmd)
		}
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "?":
			m.showHelp = !m.showHelp
		case "esc":
			m.showHelp = false
		case "c":
			if m.connection.View().Connected {
				m.statusLine = "Read-Only Mode · already connected"
			} else {
				m.connectionModalOpen = true
			}
		case "tab":
			m.focus = (m.focus + 1) % 3
			m.syncRightPaneWithFocus()
		case "shift+tab":
			m.focus = (m.focus + 2) % 3
			m.syncRightPaneWithFocus()
		case "up", "k":
			if m.hasEndpointSelection() {
				m.moveEndpointSelection(-1)
			} else if m.focus == focusTree {
				if cmd := m.moveTreeSelection(-1); cmd != nil {
					return m, cmd
				}
			} else if m.focus == focusWatchlist {
				m.moveWatchSelection(-1)
			}
		case "down", "j":
			if m.hasEndpointSelection() {
				m.moveEndpointSelection(1)
			} else if m.focus == focusTree {
				if cmd := m.moveTreeSelection(1); cmd != nil {
					return m, cmd
				}
			} else if m.focus == focusWatchlist {
				m.moveWatchSelection(1)
			}
		case "enter", "right", "l":
			if cmd := m.connectSelectedEndpoint(); cmd != nil {
				return m, cmd
			}
			if m.focus == focusTree {
				if cmd := m.expandSelectedNode(); cmd != nil {
					return m, cmd
				}
			}
		case "w":
			if cmd := m.addSelectedNodeToWatchlist(); cmd != nil {
				return m, cmd
			}
		case "s":
			return m, tea.Batch(toastCmd, m.exportSnapshot())
		case "d":
			return m, tea.Batch(toastCmd, m.exportDiagnosticsBundle())
		case "o":
			return m, tea.Batch(toastCmd, m.openExportsFolder())
		case "left", "h":
			if m.focus == focusTree {
				m.collapseSelectedNode()
			}
		}
	case endpointDiscoveryMsg:
		if msg.Err != nil {
			log.Printf("endpoint discovery failed: %v", msg.Err)
			m.diagnosticsLog.Add("Endpoint discovery failed: " + msg.Err.Error())
		} else {
			log.Printf("endpoint discovery succeeded: endpoints=%d", len(msg.Endpoints))
			m.diagnosticsLog.Add(fmt.Sprintf("Endpoint discovery succeeded: %d endpoint(s)", len(msg.Endpoints)))
		}
		m.connection.ApplyDiscovery(msg.Endpoints, msg.Err)
		view := m.connection.View()
		m.statusLine = connectionStatusLine(view)
		if msg.Err != nil {
			return m, tea.Batch(toastCmd, m.pushConnectionErrorToast(view.LastError, "Endpoint discovery failed"))
		}
	case endpointConnectionMsg:
		m.connection.ApplyConnection(msg.Request, msg.Err)
		view := m.connection.View()
		if msg.Err != nil {
			log.Printf("endpoint connection failed: %v", msg.Err)
			m.diagnosticsLog.Add("Server Connection failed: " + msg.Err.Error())
			m.statusLine = connectionStatusLine(view)
			return m, tea.Batch(toastCmd, m.pushConnectionErrorToast(view.LastError, "Connection failed"))
		}
		log.Printf("endpoint connection succeeded: endpoint=%s securityPolicy=%s securityMode=%s authType=%s", msg.Request.Endpoint, msg.Request.SecurityPolicy, msg.Request.SecurityMode, msg.Request.AuthType)
		m.diagnosticsLog.Add(fmt.Sprintf("Server Connection succeeded: endpoint=%s securityMode=%s securityPolicy=%s authType=%s", msg.Request.Endpoint, msg.Request.SecurityMode, msg.Request.SecurityPolicy, msg.Request.AuthType))
		m.connectedRequest = msg.Request
		m.connectionModalOpen = false
		m.focus = focusTree
		m.statusLine = connectionStatusLine(view)
		return m, tea.Batch(toastCmd, m.pushConnectionSuccessToast(msg.Request), m.startBrowse("i=85"))
	case browseChildrenMsg:
		m.applyBrowseResult(msg)
	case selectedValueSubscribedMsg:
		return m.applySelectedValueSubscribed(msg)
	case selectedValueMsg:
		return m.applySelectedValue(msg)
	case selectedNodeDetailsMsg:
		return m.applySelectedNodeDetails(msg)
	}
	return m, toastCmd
}

func (m Model) View() string {
	if m.showHelp {
		return m.frame(m.helpView())
	}

	innerWidth := clamp(m.width-8, 88, 160)
	gap := 2
	leftWidth := clamp((innerWidth-gap)*48/100, 40, 68)
	rightWidth := innerWidth - gap - leftWidth
	if rightWidth < 38 {
		rightWidth = 38
		leftWidth = innerWidth - gap - rightWidth
	}
	mainHeight := m.mainPanelHeight()

	left := m.panel(panelTitleStyle.Render("Address Space"), m.addressSpaceLines(mainHeight), leftWidth, mainHeight, m.focus == focusTree)
	right := m.panel(m.rightPaneTitle(), m.rightPaneLines(mainHeight), rightWidth, mainHeight, m.focus == focusDetails || m.focus == focusWatchlist)

	body := lipgloss.JoinVertical(lipgloss.Left,
		m.topBar(innerWidth),
		lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", gap), right),
		m.footer(innerWidth),
	)
	if m.connectionModalActive() {
		modalTop := 6
		modalMaxHeight := len(strings.Split(body, "\n")) - modalTop - 1
		body = overlayCentered(body, m.connectionModalView(innerWidth, modalMaxHeight), innerWidth, modalTop)
	}
	return m.toasts.Overlay(m.frame(body))
}

func overlayCentered(base string, overlay string, width int, top int) string {
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")
	for i, overlayLine := range overlayLines {
		lineIndex := top + i
		if lineIndex < 0 || lineIndex >= len(baseLines) {
			continue
		}

		overlayWidth := lipgloss.Width(overlayLine)
		leftPad := (width - overlayWidth) / 2
		if leftPad < 0 {
			leftPad = 0
		}

		baseLine := baseLines[lineIndex]
		prefix := ansi.Cut(baseLine, 0, leftPad)
		if prefixWidth := ansi.StringWidth(prefix); prefixWidth < leftPad {
			prefix += strings.Repeat(" ", leftPad-prefixWidth)
		}
		suffix := ansi.Cut(baseLine, leftPad+overlayWidth, ansi.StringWidth(baseLine))
		baseLines[lineIndex] = prefix + overlayLine + suffix
	}
	return strings.Join(baseLines, "\n")
}

func (m Model) connectionModalActive() bool {
	return m.connectionModalOpen && !m.connection.View().Connected
}

func (m Model) connectionModalView(width int, maxHeight int) string {
	view := m.connection.View()
	modalWidth := clamp(width*2/3, 56, 88)
	style := modalStyle.Width(modalWidth)
	contentWidth := modalWidth - style.GetHorizontalFrameSize()
	input := m.connectionInput
	input.Width = clamp(width/2, 40, 64)
	lines := []string{
		panelTitleStyle.Render("Connect to OPC UA Server"),
		labelStyle.Render("Server URL"),
		ansi.Truncate(input.View(), contentWidth, "…"),
	}
	if len(view.Endpoints) == 0 {
		lines = append(lines, mutedStyle.Render("Enter server URL"))
	} else {
		reservedFooterLines := 2
		if view.Connecting {
			reservedFooterLines += 2
		}
		contentHeight := maxHeight - style.GetVerticalFrameSize()
		availableEndpointLines := contentHeight - len(lines) - 1 - reservedFooterLines
		lines = append(lines, "")
		if view.Status == ServerConnectionSelectingAuthType {
			lines = append(lines, authSelectionModalLines(view.AuthTypes, view.SelectedAuthType, availableEndpointLines, contentWidth)...)
		} else if view.Status == ServerConnectionEnteringCredentials {
			lines = append(lines, credentialEntryModalLines(m.usernameInput, m.passwordInput, m.credentialFocus, availableEndpointLines, contentWidth)...)
		} else {
			lines = append(lines, endpointSelectionModalLines(view.Endpoints, view.SelectedEndpoint, view.HasClientCertificateAndKey, availableEndpointLines, contentWidth)...)
		}
	}
	if view.Connecting {
		lines = append(lines, "", "Connecting…")
	}
	lines = append(lines, "", footerStyle.Render(m.statusLine))
	return style.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func endpointSelectionModalLines(endpoints []opcua.Endpoint, selected int, hasClientCertificateAndKey bool, maxLines int, width int) []string {
	if len(endpoints) == 0 || maxLines < 1 {
		return nil
	}
	if maxLines < 4 {
		return []string{ansi.Truncate(fmt.Sprintf("Discovered endpoints: %d", len(endpoints)), width, "…")}
	}

	lines := []string{
		ansi.Truncate(fmt.Sprintf("Discovered endpoints: %d", len(endpoints)), width, "…"),
		ansi.Truncate(mutedStyle.Render("Select an endpoint and press Enter to connect."), width, "…"),
		"",
	}
	remaining := maxLines - len(lines)
	selectedDetailLines := endpointSelectedDetailLines(endpoints, selected, width)
	listLines := remaining - len(selectedDetailLines)
	if listLines < 1 {
		listLines = 1
		selectedDetailLines = nil
	}

	start := selected - listLines/2
	if start < 0 {
		start = 0
	}
	if start+listLines > len(endpoints) {
		start = len(endpoints) - listLines
		if start < 0 {
			start = 0
		}
	}
	end := start + listLines
	if end > len(endpoints) {
		end = len(endpoints)
	}
	if start > 0 {
		lines = append(lines, mutedStyle.Render(fmt.Sprintf("↑ %d earlier", start)))
	}
	for i := start; i < end && len(lines) < maxLines-len(selectedDetailLines); i++ {
		endpoint := endpoints[i]
		marker := " "
		if i == selected {
			marker = "›"
		}
		tokens := compactTokens(endpoint.UserTokenTypes)
		if tokens == "" {
			tokens = "unknown auth"
		}
		security := fmt.Sprintf("%s · %s", compactSecurityMode(endpoint.SecurityMode), endpoint.SecurityPolicy)
		capability := endpointCapabilityLabel(endpoint, hasClientCertificateAndKey)
		line := fmt.Sprintf("%s %d. %s · %s", marker, i+1, security, tokens)
		if capability != "" {
			line += " · " + capability
		}
		lines = append(lines, ansi.Truncate(line, width, "…"))
	}
	if end < len(endpoints) && len(lines) < maxLines-len(selectedDetailLines) {
		lines = append(lines, mutedStyle.Render(fmt.Sprintf("↓ %d more", len(endpoints)-end)))
	}
	if len(selectedDetailLines) > 0 && len(lines)+len(selectedDetailLines) <= maxLines {
		lines = append(lines, selectedDetailLines...)
	}
	return lines
}

func endpointSelectedDetailLines(endpoints []opcua.Endpoint, selected int, width int) []string {
	if selected < 0 || selected >= len(endpoints) {
		return nil
	}
	endpoint := endpoints[selected]
	lines := []string{""}
	tokens := compactTokens(endpoint.UserTokenTypes)
	if tokens != "" {
		lines = append(lines, ansi.Truncate(labelStyle.Render("Auth")+": "+tokens, width, "…"))
	}
	if endpoint.SecurityLevel > 0 {
		lines = append(lines, ansi.Truncate(labelStyle.Render("Level")+fmt.Sprintf(": %d", endpoint.SecurityLevel), width, "…"))
	}
	if endpoint.ServerThumbprint != "" {
		lines = append(lines, ansi.Truncate(labelStyle.Render("Server cert")+": "+endpoint.ServerThumbprint, width, "…"))
	}
	return lines
}

func authSelectionModalLines(authTypes []string, selected int, maxLines int, width int) []string {
	lines := []string{
		ansi.Truncate("Authentication Options:", width, "…"),
		ansi.Truncate(mutedStyle.Render("Select an authentication method and press Enter."), width, "…"),
		"",
	}
	for i, authType := range authTypes {
		marker := " "
		if i == selected {
			marker = "›"
		}
		line := fmt.Sprintf("%s %d. %s", marker, i+1, authType)
		lines = append(lines, ansi.Truncate(line, width, "…"))
	}
	return lines
}

func credentialEntryModalLines(usernameInput, passwordInput textinput.Model, focus int, maxLines int, width int) []string {
	usernameInput.Width = width
	passwordInput.Width = width
	if focus == 0 {
		usernameInput.Focus()
		passwordInput.Blur()
	} else {
		usernameInput.Blur()
		passwordInput.Focus()
	}
	lines := []string{
		ansi.Truncate("Authentication Credentials:", width, "…"),
		ansi.Truncate(mutedStyle.Render("Enter your credentials and press Enter to connect."), width, "…"),
		"",
		labelStyle.Render("Username"),
		ansi.Truncate(usernameInput.View(), width, "…"),
		"",
		labelStyle.Render("Password"),
		ansi.Truncate(passwordInput.View(), width, "…"),
	}
	return lines
}

func (m Model) updateConnectionModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	view := m.connection.View()

	switch msg.String() {
	case "ctrl+c", "q":
		if view.Status == ServerConnectionEnteringCredentials && msg.String() == "q" {
			break
		}
		return m, tea.Quit
	case "?":
		if view.Status == ServerConnectionEnteringCredentials {
			break
		}
		m.showHelp = !m.showHelp
		return m, nil
	case "esc":
		if view.Status == ServerConnectionSelectingAuthType || view.Status == ServerConnectionEnteringCredentials {
			m.connection.Back()
			return m, nil
		}
		if !view.Connecting {
			m.connectionModalOpen = false
			m.statusLine = "Read-Only Mode · not connected"
		}
		return m, nil
	case "tab", "shift+tab":
		if view.Status == ServerConnectionEnteringCredentials {
			m.credentialFocus = (m.credentialFocus + 1) % 2
			if m.credentialFocus == 0 {
				m.usernameInput.Focus()
				m.passwordInput.Blur()
			} else {
				m.passwordInput.Focus()
				m.usernameInput.Blur()
			}
			return m, nil
		}
	case "up", "k":
		if view.Status == ServerConnectionEnteringCredentials {
			if msg.String() == "up" {
				m.credentialFocus = (m.credentialFocus + 1) % 2
				if m.credentialFocus == 0 {
					m.usernameInput.Focus()
					m.passwordInput.Blur()
				} else {
					m.passwordInput.Focus()
					m.usernameInput.Blur()
				}
			}
			if msg.String() == "up" || msg.String() == "k" {
				// if it's 'k' we treat it as typing in the input, handled below
				if msg.String() == "up" {
					return m, nil
				}
			}
		} else if view.HasAuthTypeSelection {
			m.connection.MoveAuthTypeSelection(-1)
			return m, nil
		} else if m.hasEndpointSelection() {
			m.moveEndpointSelection(-1)
			return m, nil
		}
	case "down", "j":
		if view.Status == ServerConnectionEnteringCredentials {
			if msg.String() == "down" {
				m.credentialFocus = (m.credentialFocus + 1) % 2
				if m.credentialFocus == 0 {
					m.usernameInput.Focus()
					m.passwordInput.Blur()
				} else {
					m.passwordInput.Focus()
					m.usernameInput.Blur()
				}
			}
			if msg.String() == "down" || msg.String() == "j" {
				if msg.String() == "down" {
					return m, nil
				}
			}
		} else if view.HasAuthTypeSelection {
			m.connection.MoveAuthTypeSelection(1)
			return m, nil
		} else if m.hasEndpointSelection() {
			m.moveEndpointSelection(1)
			return m, nil
		}
	case "enter":
		if view.Status == ServerConnectionEnteringCredentials {
			m.connection.SetCredentials(m.usernameInput.Value(), m.passwordInput.Value())
			requests := m.connection.SubmitCredentials()
			return m, m.processConnectionRequests(requests)
		}
		if view.HasAuthTypeSelection {
			requests := m.connection.SelectAuthType(view.SelectedAuthType)
			if m.connection.View().Status == ServerConnectionEnteringCredentials {
				m.usernameInput.Focus()
				m.passwordInput.Blur()
				return m, nil
			}
			return m, m.processConnectionRequests(requests)
		}
		if cmd := m.connectSelectedEndpoint(); cmd != nil {
			return m, cmd
		}
		return m, m.discoverConnectionEndpoint()
	}

	if view.Status == ServerConnectionEnteringCredentials {
		var cmd tea.Cmd
		if m.credentialFocus == 0 {
			m.usernameInput, cmd = m.usernameInput.Update(msg)
		} else {
			m.passwordInput, cmd = m.passwordInput.Update(msg)
		}
		return m, cmd
	}

	if m.hasEndpointSelection() || view.HasAuthTypeSelection {
		return m, nil
	}

	var cmd tea.Cmd
	m.connectionInput, cmd = m.connectionInput.Update(msg)
	m.connection.SetEndpointText(m.connectionInput.Value())
	return m, cmd
}

func (m *Model) processConnectionRequests(requests []ServerConnectionRequest) tea.Cmd {
	view := m.connection.View()
	m.statusLine = connectionStatusLine(view)
	cmd := m.commandsFromConnectionRequests(requests)
	if view.LastError != "" {
		cmd = tea.Batch(cmd, m.pushConnectionErrorToast(view.LastError, "Connection failed"))
	}
	return cmd
}

func (m *Model) discoverConnectionEndpoint() tea.Cmd {
	m.connection.SetEndpointText(m.connectionInput.Value())
	requests := m.connection.Submit()
	return m.processConnectionRequests(requests)
}

func (m *Model) pushConnectionErrorToast(message, title string) tea.Cmd {
	if strings.TrimSpace(message) == "" {
		return nil
	}
	var cmd tea.Cmd
	m.toasts, _, cmd = m.toasts.Push(toast.Error(message, toast.WithTitle(title), toast.WithID("connection-error")))
	return cmd
}

func (m *Model) pushConnectionSuccessToast(request opcua.ConnectRequest) tea.Cmd {
	message := strings.TrimSpace(request.Endpoint)
	if message == "" {
		message = "Connected to OPC UA server"
	}
	var cmd tea.Cmd
	m.toasts, _, cmd = m.toasts.Push(toast.Success(message, toast.WithTitle("Connected"), toast.WithID("connection-success")))
	return cmd
}

func (m Model) rightPaneTitle() string {
	detailsTab := mutedStyle.Render("Node Details")
	watchlistTab := mutedStyle.Render(fmt.Sprintf("Watchlist (%d)", len(m.inspections.Watched())))
	if m.rightPane == rightPaneWatchlist {
		watchlistTab = panelTitleStyle.Render(fmt.Sprintf("Watchlist (%d)", len(m.inspections.Watched())))
	} else {
		detailsTab = panelTitleStyle.Render("Node Details")
	}
	return detailsTab + mutedStyle.Render(" | ") + watchlistTab
}

func (m Model) rightPaneLines(panelHeight int) []string {
	if m.rightPane == rightPaneWatchlist {
		return m.watchlistLines(panelHeight)
	}
	return m.details
}

func (m *Model) syncRightPaneWithFocus() {
	if m.focus == focusDetails {
		m.rightPane = rightPaneDetails
	} else if m.focus == focusWatchlist {
		m.rightPane = rightPaneWatchlist
	}
}

func (m Model) watchlistLines(panelHeight int) []string {
	watched := m.inspections.Watched()
	if len(watched) == 0 {
		return []string{"No Variable Nodes added yet.", "Select a Variable Node and press w to subscribe its Live Value."}
	}

	pageSize := m.watchlistPageSize(panelHeight)
	start := clamp(m.watchScroll, 0, max(0, len(watched)-1))
	end := start + pageSize
	if end > len(watched) {
		end = len(watched)
	}

	lines := make([]string, 0, pageSize*2+1)
	if start > 0 {
		lines = append(lines, mutedStyle.Render(fmt.Sprintf("↑ %d earlier", start)))
	}
	for i, item := range watched[start:end] {
		watchIndex := start + i
		state := "subscribing…"
		if item.Err != nil {
			state = "subscription failed: " + item.Err.Error()
			if item.Stale {
				state = "Stale Value: " + item.Err.Error()
			}
		} else if item.Value.Value != "" {
			stamp := compactTimestamp(item.Value.SourceTimestamp)
			if stamp == "" {
				stamp = compactTimestamp(item.Value.ServerTimestamp)
			}
			state = fmt.Sprintf("%s · %s", ellipsize(item.Value.Value, 48), compactStatus(item.Value.Status))
			if stamp != "" {
				state += " · " + stamp
			}
		} else if !item.Subscribing {
			state = "waiting for first Live Value…"
		}
		marker := "•"
		if m.focus == focusWatchlist && watchIndex == m.selectedWatch {
			marker = "›"
		}
		lines = append(lines, fmt.Sprintf("%s %s = %s", marker, item.Node.DisplayName, state), "  "+mutedStyle.Render(ellipsize(item.Node.NodeID, 72)))
	}
	if end < len(watched) {
		lines = append(lines, mutedStyle.Render(fmt.Sprintf("↓ %d more", len(watched)-end)))
	}
	return lines
}

func (m Model) watchlistPageSize(panelHeight int) int {
	bodyLines := panelHeight - panelStyle.GetVerticalFrameSize() - 1
	// Each Watchlist item takes two lines; keep room for scroll affordances.
	pageSize := bodyLines / 2
	if pageSize < 1 {
		return 1
	}
	return pageSize
}

func compactTimestamp(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.Local().Format("15:04:05")
}

func compactStatus(status string) string {
	if strings.Contains(status, "StatusOK") || strings.Contains(status, "StatusGood") || status == "0" {
		return "OK"
	}
	return status
}

func ellipsize(value string, maxLength int) string {
	if maxLength < 1 || len(value) <= maxLength {
		return value
	}
	return value[:maxLength-1] + "…"
}

func (m Model) addressSpaceLines(panelHeight int) []string {
	visible := m.addressSpace.View()
	lines := make([]string, 0, len(visible)+4)
	treeLines := m.visibleTreeWindow(visible, m.addressTreePageSize(panelHeight))
	for i, item := range treeLines {
		visibleIndex := m.treeScroll + i
		indent := strings.Repeat("  ", item.Depth)
		marker := " "
		if visibleIndex == m.selectedTree && m.focus == focusTree {
			marker = "›"
		}
		expander := " "
		if item.IsLoading {
			expander = "…"
		} else if !item.ChildrenLoaded {
			expander = "▸"
		} else if item.IsExpanded {
			expander = "▾"
		} else {
			expander = "▸"
		}
		name := item.Node.DisplayName
		if item.Node.NodeClass == "Variable" {
			name = name + " " + mutedStyle.Render("variable")
		}
		line := fmt.Sprintf("%s %s%s%s", marker, indent, expander, name)
		if item.Err != nil {
			line += " " + mutedStyle.Render("browse failed")
		}
		lines = append(lines, line)
	}
	if !m.connection.View().Connected {
		lines = append(lines, mutedStyle.Render("  Open Connection Modal to connect"))
	}
	lines = append(lines, "", labelStyle.Render("Search")+": indexed Objects nodes")
	if m.launch.Endpoint != "" {
		lines = append(lines, "", labelStyle.Render("Endpoint")+": "+m.launch.Endpoint)
	}
	return lines
}

func (m Model) topBar(width int) string {
	title := titleBarStyle.Render("TermUA") + " " + mutedStyle.Render("OPC UA Client TUI")
	badge := readOnlyBadgeStyle.Render("READ-ONLY")
	space := width - lipgloss.Width(title) - lipgloss.Width(badge)
	if space < 1 {
		space = 1
	}
	return title + strings.Repeat(" ", space) + badge
}

func (m Model) footer(width int) string {
	left := footerStyle.Render(m.statusLine)
	rightHint := "Tab focus · ? help · q quit"
	if m.hasEndpointSelection() {
		rightHint = "↑/↓ endpoint · Enter connect · ? help · q quit"
	} else if m.focus == focusTree && m.connection.View().Connected {
		rightHint = "↑/↓ node · Enter expand · w watch · ← collapse · ? help · q quit"
	} else if m.focus == focusWatchlist {
		rightHint = "↑/↓ scroll Watchlist · Tab focus · ? help · q quit"
	}
	right := footerStyle.Render(rightHint)
	space := width - lipgloss.Width(left) - lipgloss.Width(right)
	if space < 1 {
		space = 1
	}
	return left + strings.Repeat(" ", space) + right
}

func (m Model) frame(body string) string {
	return appFrameStyle.Width(clamp(m.width, 88, 180)).Render(body)
}

func (m Model) panel(title string, lines []string, width int, height int, focused bool) string {
	style := panelStyle
	if focused {
		style = focusedPanelStyle
	}

	contentWidth := width - style.GetHorizontalFrameSize()
	if contentWidth < 10 {
		contentWidth = 10
	}
	contentHeight := height - style.GetVerticalFrameSize()
	if contentHeight < 3 {
		contentHeight = 3
	}

	style = style.Width(contentWidth).Height(contentHeight)
	bodyLines := contentHeight - panelStyle.GetVerticalPadding() - 1
	if bodyLines < 1 {
		bodyLines = 1
	}
	body := wrapLines(lines, contentWidth, bodyLines)
	return style.Render(title + "\n" + body)
}

func (m Model) helpView() string {
	return lipgloss.JoinVertical(lipgloss.Left,
		m.topBar(clamp(m.width-8, 88, 160)),
		helpPanelStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
			panelTitleStyle.Render("Help"),
			"↑/↓ or j/k  Move selection",
			"Enter       Connect/expand Address Space node",
			"/           Search indexed Objects nodes",
			"w           Add Variable Node to Watchlist",
			"s           Export Snapshot",
			"d           Export Diagnostics Bundle",
			"o           Open exports folder",
			"Tab         Move focus",
			"Esc or ?    Close help",
			"q           Quit",
		)),
		footerStyle.Render("Read-Only Mode"),
	)
}

func discoverEndpointsCmd(client opcua.Client, endpoint string) tea.Cmd {
	return func() tea.Msg {
		log.Printf("endpoint discovery started: endpoint=%s", endpoint)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		endpoints, err := client.DiscoverEndpoints(ctx, endpoint)
		return endpointDiscoveryMsg{Endpoints: endpoints, Err: err}
	}
}

func connectEndpointCmd(client opcua.Client, request opcua.ConnectRequest) tea.Cmd {
	return func() tea.Msg {
		log.Printf("endpoint connection started: endpoint=%s securityPolicy=%s securityMode=%s authType=%s", request.Endpoint, request.SecurityPolicy, request.SecurityMode, request.AuthType)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		err := client.Connect(ctx, request)
		return endpointConnectionMsg{Request: request, Err: err}
	}
}

func browseChildrenCmd(client opcua.Client, parentNodeID string) tea.Cmd {
	return func() tea.Msg {
		log.Printf("address space browse started: parentNodeID=%s", parentNodeID)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		children, err := client.BrowseChildren(ctx, parentNodeID)
		return browseChildrenMsg{ParentNodeID: parentNodeID, Children: children, Err: err}
	}
}

func subscribeSelectedValueCmd(client opcua.Client, nodeID string) tea.Cmd {
	return func() tea.Msg {
		log.Printf("selected-node subscription started: nodeID=%s", nodeID)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		updates, subscription, err := client.SubscribeValue(ctx, nodeID)
		return selectedValueSubscribedMsg{NodeID: nodeID, Updates: updates, Subscription: subscription, Err: err}
	}
}

func readSelectedNodeDetailsCmd(client opcua.Client, nodeID string) tea.Cmd {
	return func() tea.Msg {
		log.Printf("selected-node details started: nodeID=%s", nodeID)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		details, err := client.ReadNodeDetails(ctx, nodeID)
		return selectedNodeDetailsMsg{NodeID: nodeID, Details: details, Err: err}
	}
}

func waitForSelectedValueCmd(nodeID string, updates <-chan opcua.LiveValue) tea.Cmd {
	return func() tea.Msg {
		value, ok := <-updates
		if !ok {
			return selectedValueMsg{NodeID: nodeID, Err: fmt.Errorf("subscription closed")}
		}
		return selectedValueMsg{NodeID: nodeID, Value: value}
	}
}

func cancelSubscriptionCmd(subscription opcua.ValueSubscription) tea.Cmd {
	return func() tea.Msg {
		if subscription != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = subscription.Cancel(ctx)
		}
		return nil
	}
}

func (m Model) commandsFromConnectionRequests(requests []ServerConnectionRequest) tea.Cmd {
	if len(requests) == 0 {
		return nil
	}
	cmds := make([]tea.Cmd, 0, len(requests))
	for _, request := range requests {
		switch request.Kind {
		case ServerConnectionRequestDiscoverEndpoints:
			if m.client != nil {
				cmds = append(cmds, discoverEndpointsCmd(m.client, request.Endpoint))
			}
		case ServerConnectionRequestConnectEndpoint:
			if m.client != nil {
				cmds = append(cmds, connectEndpointCmd(m.client, request.Connect))
			}
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m Model) commandsFromRequests(requests []session.Request) tea.Cmd {
	if len(requests) == 0 {
		return nil
	}
	cmds := make([]tea.Cmd, 0, len(requests))
	for _, request := range requests {
		switch request.Kind {
		case session.RequestSubscribeValue:
			if m.client != nil {
				cmds = append(cmds, subscribeSelectedValueCmd(m.client, request.NodeID))
			}
		case session.RequestReadDetails:
			if m.client != nil {
				cmds = append(cmds, readSelectedNodeDetailsCmd(m.client, request.NodeID))
			}
		case session.RequestWaitValue:
			cmds = append(cmds, waitForSelectedValueCmd(request.NodeID, request.Updates))
		case session.RequestCancelSubscription:
			cmds = append(cmds, cancelSubscriptionCmd(request.Subscription))
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) moveTreeSelection(delta int) tea.Cmd {
	visible := m.addressSpace.View()
	if len(visible) == 0 {
		return nil
	}
	m.selectedTree = (m.selectedTree + delta + len(visible)) % len(visible)
	m.ensureSelectedTreeVisible(len(visible), m.addressTreePageSize(m.mainPanelHeight()))
	return m.selectNode(visible[m.selectedTree].Node)
}

func (m *Model) moveWatchSelection(delta int) {
	watched := m.inspections.Watched()
	if len(watched) == 0 {
		m.selectedWatch = 0
		m.watchScroll = 0
		return
	}
	m.selectedWatch = (m.selectedWatch + delta + len(watched)) % len(watched)
	m.ensureSelectedWatchVisible(m.watchlistPageSize(m.mainPanelHeight()))
}

func (m *Model) expandSelectedNode() tea.Cmd {
	if !m.connection.View().Connected || m.client == nil {
		return nil
	}
	visible := m.addressSpace.View()
	if m.selectedTree < 0 || m.selectedTree >= len(visible) {
		return nil
	}
	item := visible[m.selectedTree]
	needsFetch := m.addressSpace.Toggle(item.Node.NodeID)
	if needsFetch {
		m.statusLine = "Read-Only Mode · browsing Address Space"
		return browseChildrenCmd(m.client, item.Node.NodeID)
	}
	return nil
}

func (m *Model) startBrowse(nodeID string) tea.Cmd {
	m.addressSpace.MarkLoading(nodeID)
	if nodeID == "i=85" {
		m.statusLine = "Read-Only Mode · Loading Objects Address Space…"
	} else {
		m.statusLine = "Read-Only Mode · browsing Address Space"
	}
	return browseChildrenCmd(m.client, nodeID)
}

func (m *Model) collapseSelectedNode() {
	visible := m.addressSpace.View()
	if m.selectedTree >= 0 && m.selectedTree < len(visible) {
		m.addressSpace.Collapse(visible[m.selectedTree].Node.NodeID)
	}
}

func (m *Model) addSelectedNodeToWatchlist() tea.Cmd {
	if !m.connection.View().Connected || m.client == nil {
		return nil
	}
	visible := m.addressSpace.View()
	if m.selectedTree < 0 || m.selectedTree >= len(visible) {
		return nil
	}
	node := visible[m.selectedTree].Node
	if node.NodeClass != "Variable" {
		m.statusLine = "Read-Only Mode · select a Variable Node to watch"
		return nil
	}
	if m.inspections.IsWatched(node.NodeID) {
		m.statusLine = "Read-Only Mode · Variable Node already on Watchlist"
		return nil
	}
	requests := m.inspections.Watch(node)
	m.selectedWatch = len(m.inspections.Watched()) - 1
	m.ensureSelectedWatchVisible(m.watchlistPageSize(m.mainPanelHeight()))
	m.statusLine = "Read-Only Mode · subscribing Watchlist node"
	return m.commandsFromRequests(requests)
}

func (m *Model) selectNode(node opcua.AddressNode) tea.Cmd {
	if node.NodeClass != "Variable" || !m.connection.View().Connected || m.client == nil {
		m.details = nodeDetailLines(node)
		return m.commandsFromRequests(m.inspections.Select(node))
	}
	requests := m.inspections.Select(node)
	if selected, ok := m.inspections.Selected(); ok {
		m.details = liveValueDetailLines(selected)
	}
	m.statusLine = "Read-Only Mode · loading selected Live Value"
	return m.commandsFromRequests(requests)
}

func (m Model) applySelectedValueSubscribed(msg selectedValueSubscribedMsg) (tea.Model, tea.Cmd) {
	requests := m.inspections.ApplySubscription(msg.NodeID, msg.Updates, msg.Subscription, msg.Err)
	if msg.Err != nil {
		log.Printf("selected-node subscription failed: nodeID=%s error=%v", msg.NodeID, msg.Err)
		m.statusLine = "Read-Only Mode · selected Live Value failed"
	} else {
		m.statusLine = "Read-Only Mode · selected Live Value active"
	}
	if selected, ok := m.inspections.Selected(); ok {
		m.details = liveValueDetailLines(selected)
	}
	return m, m.commandsFromRequests(requests)
}

func (m Model) applySelectedValue(msg selectedValueMsg) (tea.Model, tea.Cmd) {
	requests := m.inspections.ApplyLiveValue(msg.NodeID, msg.Value, msg.Err)
	if selected, ok := m.inspections.Selected(); ok {
		m.details = liveValueDetailLines(selected)
		if msg.Err != nil && selected.Node.NodeID == msg.NodeID {
			m.statusLine = "Read-Only Mode · selected Live Value stale"
		} else if selected.Node.NodeID == msg.NodeID {
			m.statusLine = fmt.Sprintf("Read-Only Mode · selected Live Value updated · %s", selected.Node.DisplayName)
		}
	}
	return m, m.commandsFromRequests(requests)
}

func (m Model) applySelectedNodeDetails(msg selectedNodeDetailsMsg) (tea.Model, tea.Cmd) {
	m.inspections.ApplyDetails(msg.NodeID, msg.Details, msg.Err)
	if selected, ok := m.inspections.Selected(); ok {
		m.details = liveValueDetailLines(selected)
	}
	return m, nil
}

func (m *Model) exportSnapshot() tea.Cmd {
	watched := m.inspections.Watched()
	if len(watched) == 0 {
		message := "no watched Variable Nodes to export"
		m.statusLine = "Read-Only Mode · " + message
		return m.pushSnapshotErrorToast(message)
	}
	baseDir := m.paths.CacheDir
	if baseDir == "" {
		baseDir = "."
	}
	exportDir := filepath.Join(baseDir, "exports")
	if err := os.MkdirAll(exportDir, 0o755); err != nil {
		message := "Snapshot export failed: " + err.Error()
		m.statusLine = "Read-Only Mode · " + message
		return m.pushSnapshotErrorToast(message)
	}
	path := filepath.Join(exportDir, "snapshot-"+time.Now().Format("20060102-150405")+".md")
	if err := os.WriteFile(path, []byte(m.snapshotMarkdown(watched)), 0o600); err != nil {
		message := "Snapshot export failed: " + err.Error()
		m.statusLine = "Read-Only Mode · " + message
		return m.pushSnapshotErrorToast(message)
	}
	m.statusLine = "Read-Only Mode · Snapshot exported: " + path
	return m.pushSnapshotSuccessToast(path)
}

func (m *Model) pushSnapshotErrorToast(message string) tea.Cmd {
	var cmd tea.Cmd
	m.toasts, _, cmd = m.toasts.Push(toast.Error(message, toast.WithTitle("Snapshot export failed"), toast.WithID("snapshot-export")))
	return cmd
}

func (m *Model) pushSnapshotSuccessToast(path string) tea.Cmd {
	_ = path
	var cmd tea.Cmd
	m.toasts, _, cmd = m.toasts.Push(toast.Success("Press o to open the exports folder.", toast.WithTitle("Snapshot exported"), toast.WithID("snapshot-export")))
	return cmd
}

func (m *Model) exportDiagnosticsBundle() tea.Cmd {
	path, err := m.diagnosticsExporter.ExportDiagnostics(m.diagnosticsMarkdown())
	if err != nil {
		message := "Diagnostics Bundle export failed: " + err.Error()
		m.statusLine = "Read-Only Mode · " + message
		m.diagnosticsLog.Add(message)
		return m.pushDiagnosticsErrorToast(message)
	}
	m.statusLine = "Read-Only Mode · Diagnostics Bundle exported: " + path
	m.diagnosticsLog.Add("Diagnostics Bundle exported: " + path)
	return m.pushDiagnosticsSuccessToast(path)
}

func (m *Model) pushDiagnosticsErrorToast(message string) tea.Cmd {
	var cmd tea.Cmd
	m.toasts, _, cmd = m.toasts.Push(toast.Error(message, toast.WithTitle("Diagnostics Bundle export failed"), toast.WithID("diagnostics-export")))
	return cmd
}

func (m *Model) pushDiagnosticsSuccessToast(path string) tea.Cmd {
	_ = path
	var cmd tea.Cmd
	m.toasts, _, cmd = m.toasts.Push(toast.Success("Press o to open the exports folder.", toast.WithTitle("Diagnostics Bundle exported"), toast.WithID("diagnostics-export")))
	return cmd
}

func (m *Model) openExportsFolder() tea.Cmd {
	path := exportsDir(m.paths)
	if err := os.MkdirAll(path, 0o755); err != nil {
		message := "Open exports folder failed: " + err.Error()
		m.statusLine = "Read-Only Mode · " + message
		m.diagnosticsLog.Add(message)
		return m.pushOpenExportsFolderErrorToast(message)
	}
	if err := m.exportFolderOpener.OpenExportFolder(path); err != nil {
		message := "Open exports folder failed: " + err.Error()
		m.statusLine = "Read-Only Mode · " + message
		m.diagnosticsLog.Add(message)
		return m.pushOpenExportsFolderErrorToast(message)
	}
	m.statusLine = "Read-Only Mode · opened exports folder: " + path
	m.diagnosticsLog.Add("Opened exports folder: " + path)
	return m.pushOpenExportsFolderSuccessToast()
}

func (m *Model) pushOpenExportsFolderErrorToast(message string) tea.Cmd {
	var cmd tea.Cmd
	m.toasts, _, cmd = m.toasts.Push(toast.Error(message, toast.WithTitle("Open exports folder failed"), toast.WithID("open-exports-folder")))
	return cmd
}

func (m *Model) pushOpenExportsFolderSuccessToast() tea.Cmd {
	var cmd tea.Cmd
	m.toasts, _, cmd = m.toasts.Push(toast.Success("Exports folder opened.", toast.WithTitle("Exports folder"), toast.WithID("open-exports-folder")))
	return cmd
}

func (m Model) diagnosticsMarkdown() string {
	view := m.connection.View()
	watched := m.inspections.Watched()
	var b strings.Builder
	b.WriteString("# Diagnostics Bundle\n\n")
	b.WriteString("> Sensitivity warning: endpoints, node names, process values, and server metadata may be sensitive. Review before sharing. Secrets are excluded.\n\n")
	b.WriteString("## Server Connection\n\n")
	appendMarkdownField(&b, "Endpoint", diagnosticsEndpoint(m.connectedRequest.Endpoint, view.EndpointText))
	appendMarkdownField(&b, "Security Mode", compactSecurityMode(m.connectedRequest.SecurityMode))
	appendMarkdownField(&b, "Security Policy", m.connectedRequest.SecurityPolicy)
	appendMarkdownField(&b, "Authentication", string(m.connectedRequest.AuthType))
	appendMarkdownField(&b, "Connection State", serverConnectionStatusText(view.Status))
	appendMarkdownField(&b, "Connected", fmt.Sprintf("%t", view.Connected))
	appendMarkdownField(&b, "Last Error", view.LastError)
	appendMarkdownField(&b, "Last Visible Status", m.statusLine)

	b.WriteString("\n## Selected Variable Node\n\n")
	if selected, ok := m.inspections.Selected(); ok {
		appendDiagnosticsInspection(&b, selected, true)
	} else {
		b.WriteString("No Variable Node selected.\n")
	}

	b.WriteString("\n## Watchlist and Subscriptions\n\n")
	appendMarkdownField(&b, "Watched Variable Nodes", fmt.Sprintf("%d", len(watched)))
	appendMarkdownField(&b, "Subscription Count", fmt.Sprintf("%d", activeSubscriptionCount(watched)))
	for _, item := range watched {
		b.WriteString("\n### " + markdownText(item.Node.DisplayName) + "\n\n")
		appendDiagnosticsInspection(&b, item, false)
	}

	b.WriteString("\n## Local Paths\n\n")
	appendMarkdownField(&b, "Config Directory", m.paths.ConfigDir)
	appendMarkdownField(&b, "Cache Directory", m.paths.CacheDir)
	appendMarkdownField(&b, "Export Directory", exportsDir(m.paths))

	b.WriteString("\n## Recent Diagnostic Events\n\n")
	events := m.diagnosticsLog.Events()
	if len(events) == 0 {
		b.WriteString("No diagnostic events recorded.\n")
	} else {
		for _, event := range events {
			appendMarkdownField(&b, event.Timestamp.UTC().Format(time.RFC3339), event.Message)
		}
	}
	return b.String()
}

func (m Model) snapshotMarkdown(watched []session.VariableNodeInspection) string {
	var b strings.Builder
	b.WriteString("# Watchlist Snapshot\n\n")
	b.WriteString("> Sensitivity warning: node names, process values, endpoints, and server metadata may be sensitive. Review before sharing.\n\n")
	b.WriteString("## Server Connection\n\n")
	appendMarkdownField(&b, "Endpoint", m.connectedRequest.Endpoint)
	appendMarkdownField(&b, "Security Mode", compactSecurityMode(m.connectedRequest.SecurityMode))
	appendMarkdownField(&b, "Security Policy", m.connectedRequest.SecurityPolicy)
	appendMarkdownField(&b, "Authentication", string(m.connectedRequest.AuthType))
	b.WriteString("\n## Watchlist\n")
	for _, item := range watched {
		b.WriteString("\n### " + markdownText(item.Node.DisplayName) + "\n\n")
		appendMarkdownField(&b, "NodeId", item.Node.NodeID)
		appendMarkdownField(&b, "NodeClass", item.Node.NodeClass)
		if item.Value.Value == "" {
			b.WriteString("- Live Value: unavailable\n")
		} else {
			appendMarkdownField(&b, "Live Value", item.Value.Value)
			appendMarkdownField(&b, "OPC UA StatusCode", item.Value.Status)
		}
		if !item.Value.SourceTimestamp.IsZero() {
			appendMarkdownField(&b, "Source Timestamp", item.Value.SourceTimestamp.UTC().Format(time.RFC3339))
		}
		if !item.Value.ServerTimestamp.IsZero() {
			appendMarkdownField(&b, "Server Timestamp", item.Value.ServerTimestamp.UTC().Format(time.RFC3339))
		}
		appendMarkdownField(&b, "Stale Value", fmt.Sprintf("%t", item.Stale))
		if item.OutOfRange != "" {
			appendMarkdownField(&b, "Out-of-Range", item.OutOfRange)
		}
		if item.Err != nil {
			appendMarkdownField(&b, "Subscription", item.Err.Error())
		}
	}
	return b.String()
}

func appendDiagnosticsInspection(b *strings.Builder, item session.VariableNodeInspection, includeHeading bool) {
	if includeHeading {
		b.WriteString("### " + markdownText(item.Node.DisplayName) + "\n\n")
	}
	appendMarkdownField(b, "DisplayName", item.Node.DisplayName)
	appendMarkdownField(b, "NodeId", item.Node.NodeID)
	appendMarkdownField(b, "NodeClass", item.Node.NodeClass)
	appendMarkdownField(b, "Live Value", item.Value.Value)
	appendMarkdownField(b, "OPC UA StatusCode", item.Value.Status)
	appendMarkdownField(b, "Stale Value", fmt.Sprintf("%t", item.Stale))
	if item.OutOfRange != "" {
		appendMarkdownField(b, "Out-of-Range", item.OutOfRange)
	}
	if item.Subscription != nil {
		appendMarkdownField(b, "Subscription", "active")
	} else if item.Subscribing {
		appendMarkdownField(b, "Subscription", "subscribing")
	} else if item.Err != nil {
		appendMarkdownField(b, "Subscription", item.Err.Error())
	} else {
		appendMarkdownField(b, "Subscription", "not active")
	}
}

func activeSubscriptionCount(watched []session.VariableNodeInspection) int {
	count := 0
	for _, item := range watched {
		if item.Subscription != nil {
			count++
		}
	}
	return count
}

func diagnosticsEndpoint(connectedEndpoint, typedEndpoint string) string {
	if strings.TrimSpace(connectedEndpoint) != "" {
		return connectedEndpoint
	}
	return typedEndpoint
}

func pathOrDot(path string) string {
	if strings.TrimSpace(path) == "" {
		return "."
	}
	return path
}

func serverConnectionStatusText(status ServerConnectionStatus) string {
	switch status {
	case ServerConnectionIdle:
		return "Idle"
	case ServerConnectionNeedsEndpoint:
		return "Needs Endpoint"
	case ServerConnectionDiscovering:
		return "Discovering"
	case ServerConnectionDiscoveryFailed:
		return "Discovery Failed"
	case ServerConnectionDiscovered:
		return "Discovered"
	case ServerConnectionConnecting:
		return "Connecting"
	case ServerConnectionConnected:
		return "Connected"
	case ServerConnectionFailed:
		return "Failed"
	case ServerConnectionEndpointRequiresCredentials:
		return "Endpoint Requires Credentials"
	case ServerConnectionSelectingAuthType:
		return "Selecting Authentication Type"
	case ServerConnectionEnteringCredentials:
		return "Entering Credentials"
	default:
		return fmt.Sprintf("Unknown (%d)", status)
	}
}

func appendMarkdownField(b *strings.Builder, label, value string) {
	if strings.TrimSpace(value) == "" {
		value = "unknown"
	}
	b.WriteString(fmt.Sprintf("- %s: %s\n", label, markdownText(value)))
}

func markdownText(value string) string {
	return strings.ReplaceAll(value, "\n", " ")
}

func (m *Model) applyBrowseResult(msg browseChildrenMsg) {
	m.addressSpace.ApplyChildren(msg.ParentNodeID, msg.Children, msg.Err)
	if msg.Err != nil {
		log.Printf("address space browse failed: parentNodeID=%s error=%v", msg.ParentNodeID, msg.Err)
		m.statusLine = "Read-Only Mode · browse failed"
		if node, ok := m.addressSpace.Node(msg.ParentNodeID); ok {
			m.details = append(nodeDetailLines(node), "", "Browse failed.", msg.Err.Error())
		}
		return
	}

	m.ensureSelectedTreeVisible(len(m.addressSpace.View()), m.addressTreePageSize(m.mainPanelHeight()))
	m.statusLine = fmt.Sprintf("Read-Only Mode · browsed %d child node(s)", len(msg.Children))
	if node, ok := m.addressSpace.Node(msg.ParentNodeID); ok {
		m.details = append(nodeDetailLines(node), "", fmt.Sprintf("Children loaded: %d", len(msg.Children)))
	}
}

func (m *Model) ensureSelectedTreeVisible(total int, pageSize int) {
	if total <= 0 {
		m.treeScroll = 0
		return
	}
	if pageSize < 1 {
		pageSize = 1
	}
	if m.selectedTree < 0 {
		m.selectedTree = 0
	}
	if m.selectedTree >= total {
		m.selectedTree = total - 1
	}
	if m.selectedTree < m.treeScroll {
		m.treeScroll = m.selectedTree
	}
	if m.selectedTree >= m.treeScroll+pageSize {
		m.treeScroll = m.selectedTree - pageSize + 1
	}
	maxScroll := total - pageSize
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.treeScroll > maxScroll {
		m.treeScroll = maxScroll
	}
	if m.treeScroll < 0 {
		m.treeScroll = 0
	}
}

func (m *Model) ensureSelectedWatchVisible(pageSize int) {
	watched := m.inspections.Watched()
	if len(watched) == 0 {
		m.selectedWatch = 0
		m.watchScroll = 0
		return
	}
	if pageSize < 1 {
		pageSize = 1
	}
	if m.selectedWatch < 0 {
		m.selectedWatch = 0
	}
	if m.selectedWatch >= len(watched) {
		m.selectedWatch = len(watched) - 1
	}
	if m.selectedWatch < m.watchScroll {
		m.watchScroll = m.selectedWatch
	}
	if m.selectedWatch >= m.watchScroll+pageSize {
		m.watchScroll = m.selectedWatch - pageSize + 1
	}
	maxScroll := len(watched) - pageSize
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.watchScroll > maxScroll {
		m.watchScroll = maxScroll
	}
	if m.watchScroll < 0 {
		m.watchScroll = 0
	}
}

func (m Model) visibleTreeWindow(visible []ViewItem, pageSize int) []ViewItem {
	if pageSize < 1 {
		pageSize = 1
	}
	start := clamp(m.treeScroll, 0, max(0, len(visible)-1))
	end := start + pageSize
	if end > len(visible) {
		end = len(visible)
	}
	return visible[start:end]
}

func (m Model) mainPanelHeight() int {
	return clamp(m.height-8, 16, 30)
}

func (m Model) addressTreePageSize(panelHeight int) int {
	bodyLines := panelHeight - panelStyle.GetVerticalFrameSize() - 1
	fixedLines := 2 // blank line + Search hint
	if m.launch.Endpoint != "" {
		fixedLines += 2
	}
	if !m.connection.View().Connected {
		fixedLines++
	}
	pageSize := bodyLines - fixedLines
	if pageSize < 1 {
		return 1
	}
	return pageSize
}

func defaultNodeDetailsLines() []string {
	return []string{
		"Select a Variable Node to inspect its Live Value.",
		"Health, age, timestamps, Engineering Unit, and NodeId will appear here.",
		"",
		"Press Tab to switch between Node Details and Watchlist.",
	}
}

func nodeDetailLines(node opcua.AddressNode) []string {
	lines := []string{
		labelStyle.Render("DisplayName") + ": " + node.DisplayName,
		labelStyle.Render("NodeId") + ": " + node.NodeID,
		labelStyle.Render("NodeClass") + ": " + node.NodeClass,
	}
	if node.BrowseName != "" && node.BrowseName != node.DisplayName {
		lines = append(lines, labelStyle.Render("BrowseName")+": "+node.BrowseName)
	}
	if node.NodeClass == "Variable" {
		lines = append(lines, "", "Select this Variable Node to inspect its Live Value.")
	}
	return lines
}

func liveValueDetailLines(state session.VariableNodeInspection) []string {
	lines := nodeDetailLines(state.Node)
	lines = append(lines, "", labelStyle.Render("Live Value"))
	if state.Err != nil {
		status := "Subscription failed"
		if state.Stale {
			status = "Stale Value"
		}
		return append(lines, status+": "+state.Err.Error())
	}
	if state.Subscribing {
		lines = append(lines, "Subscribing to selected Variable Node…")
	} else if state.Value.Value == "" {
		lines = append(lines, "Waiting for first Live Value…")
	} else {
		lines = append(lines,
			labelStyle.Render("Value")+": "+state.Value.Value,
			labelStyle.Render("Health")+": "+compactStatus(state.Value.Status),
		)
		if state.OutOfRange != "" {
			lines = append(lines, labelStyle.Render("Out-of-Range")+": "+state.OutOfRange)
		}
		if !state.Value.SourceTimestamp.IsZero() {
			lines = append(lines, labelStyle.Render("Source Timestamp")+": "+state.Value.SourceTimestamp.Local().Format(time.RFC3339))
			lines = append(lines, labelStyle.Render("Age")+": "+compactAge(time.Since(state.Value.SourceTimestamp)))
		}
		if !state.Value.ServerTimestamp.IsZero() {
			lines = append(lines, labelStyle.Render("Server Timestamp")+": "+state.Value.ServerTimestamp.Local().Format(time.RFC3339))
		}
		if state.Stale {
			lines = append(lines, "", "Stale Value: subscription is no longer active.")
		}
	}

	lines = append(lines, "", labelStyle.Render("Metadata"))
	if state.LoadingDetails {
		return append(lines, "Loading Variable Node metadata…")
	}
	if state.DetailsErr != nil {
		return append(lines, "Metadata unavailable: "+state.DetailsErr.Error())
	}
	return append(lines, nodeMetadataLines(state.Details)...)
}

func nodeMetadataLines(details opcua.NodeDetails) []string {
	var lines []string
	appendIfSet := func(label string, value string) {
		if value != "" {
			lines = append(lines, labelStyle.Render(label)+": "+value)
		}
	}
	appendIfSet("Data Type", details.DataType)
	appendIfSet("Access Level", details.AccessLevel)
	if details.AccessLevel != "" {
		lines = append(lines, labelStyle.Render("Writable")+": "+fmt.Sprintf("%t", details.Writable)+" (Read-Only Mode prevents writes)")
	}
	appendIfSet("Value Rank", details.ValueRank)
	appendIfSet("Array Dimensions", details.ArrayDimensions)
	appendIfSet("Engineering Unit", details.EngineeringUnit)
	if details.EURange != nil {
		lines = append(lines, labelStyle.Render("EURange")+": "+formatRange(details.EURange))
	}
	if details.InstrumentRange != nil {
		lines = append(lines, labelStyle.Render("InstrumentRange")+": "+formatRange(details.InstrumentRange))
	}
	appendIfSet("Description", details.Description)
	if len(lines) == 0 {
		return []string{"No Variable Node metadata exposed."}
	}
	return lines
}

func formatRange(valueRange *opcua.ValueRange) string {
	if valueRange == nil {
		return ""
	}
	return fmt.Sprintf("%g…%g", valueRange.Low, valueRange.High)
}

func compactAge(age time.Duration) string {
	if age < 0 {
		age = 0
	}
	if age < time.Second {
		return "<1s"
	}
	if age < time.Minute {
		return fmt.Sprintf("%ds", int(age.Seconds()))
	}
	if age < time.Hour {
		return fmt.Sprintf("%dm", int(age.Minutes()))
	}
	return fmt.Sprintf("%dh", int(age.Hours()))
}

func (m *Model) moveEndpointSelection(delta int) {
	m.connection.MoveEndpointSelection(delta)
}

func (m Model) hasEndpointSelection() bool {
	return m.connection.View().HasEndpointSelection
}

func (m *Model) connectSelectedEndpoint() tea.Cmd {
	if !m.hasEndpointSelection() || m.client == nil {
		return nil
	}
	requests := m.connection.Submit()
	view := m.connection.View()
	if view.Status == ServerConnectionEnteringCredentials {
		m.credentialFocus = 0
		m.usernameInput.Focus()
		m.passwordInput.Blur()
	}
	return m.processConnectionRequests(requests)
}

func connectionStatusLine(view ServerConnectionView) string {
	if view.LastError != "" {
		return "Read-Only Mode · connection failed"
	}
	switch view.Status {
	case ServerConnectionNeedsEndpoint:
		return "Read-Only Mode · enter an OPC UA Server URL"
	case ServerConnectionDiscovering:
		return "Read-Only Mode · discovering endpoints"
	case ServerConnectionDiscoveryFailed:
		return "Read-Only Mode · endpoint discovery failed"
	case ServerConnectionDiscovered:
		return fmt.Sprintf("Read-Only Mode · discovered %d endpoint(s)", len(view.Endpoints))
	case ServerConnectionConnecting:
		if view.SelectedEndpoint >= 0 && view.SelectedEndpoint < len(view.Endpoints) {
			endpoint := view.Endpoints[view.SelectedEndpoint]
			return fmt.Sprintf("Read-Only Mode · connecting · %s · %s", compactSecurityMode(endpoint.SecurityMode), endpoint.SecurityPolicy)
		}
		return "Read-Only Mode · connecting"
	case ServerConnectionConnected:
		if view.SelectedEndpoint >= 0 && view.SelectedEndpoint < len(view.Endpoints) {
			endpoint := view.Endpoints[view.SelectedEndpoint]
			return fmt.Sprintf("Read-Only Mode · connected · %s · %s", compactSecurityMode(endpoint.SecurityMode), endpoint.SecurityPolicy)
		}
		return "Read-Only Mode · connected"
	case ServerConnectionFailed:
		return "Read-Only Mode · connection failed"
	case ServerConnectionEndpointRequiresCredentials:
		return "Read-Only Mode · endpoint requires credentials"
	default:
		return "Read-Only Mode"
	}
}

func endpointCapabilityLabel(endpoint opcua.Endpoint, hasClientCertificateAndKey bool) string {
	securityMode := compactSecurityMode(endpoint.SecurityMode)
	if securityMode != "None" && !hasClientCertificateAndKey {
		return "cert/key"
	}
	if endpointHasUnsupportedAuth(endpoint) {
		return "unsupported auth"
	}
	if endpointSupportsOnlyAuth(endpoint, opcua.AuthUsername) {
		return "username/password"
	}
	return "connectable"
}

func endpointSupportsOnlyAuth(endpoint opcua.Endpoint, auth opcua.AuthType) bool {
	if len(endpoint.UserTokenTypes) != 1 {
		return false
	}
	return strings.EqualFold(strings.TrimPrefix(strings.TrimSpace(endpoint.UserTokenTypes[0]), "UserTokenType"), string(auth))
}

func endpointHasUnsupportedAuth(endpoint opcua.Endpoint) bool {
	for _, token := range endpoint.UserTokenTypes {
		normalized := strings.TrimPrefix(strings.TrimSpace(token), "UserTokenType")
		if normalized != string(opcua.AuthAnonymous) && normalized != string(opcua.AuthUsername) {
			return true
		}
	}
	return false
}

func compactSecurityMode(mode string) string {
	mode = strings.TrimPrefix(strings.TrimSpace(mode), "MessageSecurityMode")
	if mode == "" {
		return "Unknown"
	}
	return mode
}

func compactTokens(tokens []string) string {
	seen := map[string]bool{}
	unique := make([]string, 0, len(tokens))
	for _, token := range tokens {
		token = strings.TrimSpace(strings.TrimPrefix(token, "UserTokenType"))
		if token == "" || seen[token] {
			continue
		}
		seen[token] = true
		unique = append(unique, token)
	}
	return strings.Join(unique, ", ")
}

func wrapLines(lines []string, width int, maxLines int) string {
	wrapped := make([]string, 0, len(lines))
	wrapper := lipgloss.NewStyle().Width(width)
	truncated := false
	for _, line := range lines {
		if len(wrapped) >= maxLines {
			truncated = true
			break
		}
		if line == "" {
			wrapped = append(wrapped, "")
			continue
		}
		renderedLines := strings.Split(wrapper.Render(line), "\n")
		for i, renderedLine := range renderedLines {
			if len(wrapped) >= maxLines {
				truncated = true
				break
			}
			wrapped = append(wrapped, renderedLine)
			if i < len(renderedLines)-1 && len(wrapped) >= maxLines {
				truncated = true
			}
		}
		if truncated {
			break
		}
	}
	if truncated && len(wrapped) > 0 {
		wrapped[len(wrapped)-1] = mutedStyle.Render("… more")
	}
	return strings.Join(wrapped, "\n")
}

func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

var (
	appFrameStyle = lipgloss.NewStyle().Padding(1, 4)

	titleBarStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("219"))
	readOnlyBadgeStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("230")).
				Background(lipgloss.Color("58")).
				Padding(0, 1)
	footerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	panelTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252"))
	labelStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("110"))
	mutedStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	branchStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252"))

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(1, 1)

	focusedPanelStyle = panelStyle.BorderForeground(lipgloss.Color("63"))

	helpPanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(1, 2).
			Width(52)

	modalStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(1, 2)
)
