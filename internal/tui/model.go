package tui

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"termua/internal/app"
	"termua/internal/config"
	"termua/internal/opcua"
)

type Dependencies struct {
	Client opcua.Client
	Paths  config.Paths
	Launch app.LaunchOptions
}

type focus int

const (
	focusTree focus = iota
	focusDetails
	focusWatchlist
)

type endpointDiscoveryMsg struct {
	Endpoints []opcua.Endpoint
	Err       error
}

type endpointConnectionMsg struct {
	Request opcua.ConnectRequest
	Err     error
}

type Model struct {
	client opcua.Client
	paths  config.Paths
	launch app.LaunchOptions

	width            int
	height           int
	focus            focus
	showHelp         bool
	statusLine       string
	endpoints        []opcua.Endpoint
	selectedEndpoint int
	connecting       bool
	connected        bool
	details          []string
}

func NewModel(deps Dependencies) Model {
	status := "Read-Only Mode"
	details := []string{
		"Select a Variable Node to inspect its Live Value.",
		"Health, age, timestamps, Engineering Unit, and NodeId will appear here.",
		"",
		"Watchlist is available as a v1 drawer/tab target.",
	}

	if deps.Launch.Endpoint != "" {
		status = fmt.Sprintf("Read-Only Mode · endpoint %s", deps.Launch.Endpoint)
		details = []string{"Endpoint provided.", "Discovery will start automatically."}
	}
	if deps.Launch.ConnectionName != "" {
		status = fmt.Sprintf("Read-Only Mode · saved connection %s", deps.Launch.ConnectionName)
		details = []string{"Saved Connection launch requested.", "Saved Connection loading is not implemented yet."}
	}

	return Model{
		client:     deps.Client,
		paths:      deps.Paths,
		launch:     deps.Launch,
		width:      120,
		height:     30,
		focus:      focusTree,
		statusLine: status,
		details:    details,
	}
}

func (m Model) Init() tea.Cmd {
	if m.launch.Endpoint == "" || m.client == nil {
		return nil
	}
	return discoverEndpointsCmd(m.client, m.launch.Endpoint)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "?":
			m.showHelp = !m.showHelp
		case "esc":
			m.showHelp = false
		case "tab":
			m.focus = (m.focus + 1) % 3
		case "shift+tab":
			m.focus = (m.focus + 2) % 3
		case "up", "k":
			if m.hasEndpointSelection() {
				m.moveEndpointSelection(-1)
			}
		case "down", "j":
			if m.hasEndpointSelection() {
				m.moveEndpointSelection(1)
			}
		case "enter":
			if cmd := m.connectSelectedEndpoint(); cmd != nil {
				return m, cmd
			}
		}
	case endpointDiscoveryMsg:
		if msg.Err != nil {
			log.Printf("endpoint discovery failed: %v", msg.Err)
			m.statusLine = "Read-Only Mode · endpoint discovery failed"
			m.details = []string{"Endpoint discovery failed.", msg.Err.Error()}
			return m, nil
		}
		log.Printf("endpoint discovery succeeded: endpoints=%d", len(msg.Endpoints))
		m.endpoints = msg.Endpoints
		m.selectedEndpoint = defaultEndpointSelection(msg.Endpoints)
		m.connected = false
		m.connecting = false
		m.statusLine = fmt.Sprintf("Read-Only Mode · discovered %d endpoint(s)", len(msg.Endpoints))
		m.details = endpointDetailLines(msg.Endpoints, m.selectedEndpoint)
	case endpointConnectionMsg:
		m.connecting = false
		if msg.Err != nil {
			log.Printf("endpoint connection failed: %v", msg.Err)
			m.connected = false
			m.statusLine = "Read-Only Mode · connection failed"
			m.details = append(selectedEndpointLines(m.endpoints, m.selectedEndpoint), "", "Connection failed.", msg.Err.Error())
			return m, nil
		}
		log.Printf("endpoint connection succeeded: endpoint=%s securityPolicy=%s securityMode=%s authType=%s", msg.Request.Endpoint, msg.Request.SecurityPolicy, msg.Request.SecurityMode, msg.Request.AuthType)
		m.connected = true
		m.statusLine = fmt.Sprintf("Read-Only Mode · connected · %s · %s", compactSecurityMode(msg.Request.SecurityMode), msg.Request.SecurityPolicy)
		m.details = append(selectedEndpointLines(m.endpoints, m.selectedEndpoint), "", "Connected with Anonymous authentication.", "Address Space browsing starts next.")
	}
	return m, nil
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
	mainHeight := clamp(m.height-9, 12, 24)
	watchHeight := 6
	if mainHeight < 16 {
		watchHeight = 4
	}

	left := m.panel("Address Space", m.addressSpaceLines(), leftWidth, mainHeight, m.focus == focusTree)
	right := m.panel("Node Details", m.details, rightWidth, mainHeight, m.focus == focusDetails)
	watchlist := m.panel("Watchlist", []string{"No Variable Nodes added yet.", "Select a Variable Node and press w to keep its Live Value visible."}, innerWidth, watchHeight, m.focus == focusWatchlist)

	body := lipgloss.JoinVertical(lipgloss.Left,
		m.topBar(innerWidth),
		lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", gap), right),
		watchlist,
		m.footer(innerWidth),
	)
	return m.frame(body)
}

func (m Model) addressSpaceLines() []string {
	objectsHint := "  ▸ Connect to start lazy browsing"
	if m.connecting {
		objectsHint = "  ▸ Connecting…"
	}
	if m.connected {
		objectsHint = "  ▸ Browse loading is not implemented yet"
	}
	lines := []string{
		branchStyle.Render("Objects"),
		mutedStyle.Render(objectsHint),
		"",
		labelStyle.Render("Search") + ": indexed Objects nodes",
	}
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
	body := wrapLines(lines, contentWidth, contentHeight-1)
	return style.Render(panelTitleStyle.Render(title) + "\n" + body)
}

func (m Model) helpView() string {
	return lipgloss.JoinVertical(lipgloss.Left,
		m.topBar(clamp(m.width-8, 88, 160)),
		helpPanelStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
			panelTitleStyle.Render("Help"),
			"↑/↓ or j/k  Move selection",
			"Enter       Expand/select",
			"/           Search indexed Objects nodes",
			"w           Add Variable Node to Watchlist",
			"s           Export Snapshot",
			"d           Export Diagnostics Bundle",
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

func (m *Model) moveEndpointSelection(delta int) {
	if len(m.endpoints) == 0 {
		return
	}
	m.selectedEndpoint = (m.selectedEndpoint + delta + len(m.endpoints)) % len(m.endpoints)
	m.details = endpointDetailLines(m.endpoints, m.selectedEndpoint)
}

func (m Model) hasEndpointSelection() bool {
	return len(m.endpoints) > 0 && !m.connecting && !m.connected
}

func (m *Model) connectSelectedEndpoint() tea.Cmd {
	if !m.hasEndpointSelection() || m.client == nil || m.launch.Endpoint == "" {
		return nil
	}

	endpoint := m.endpoints[m.selectedEndpoint]
	if !supportsAuth(endpoint, opcua.AuthAnonymous) {
		m.statusLine = "Read-Only Mode · endpoint requires credentials"
		m.details = append(selectedEndpointLines(m.endpoints, m.selectedEndpoint), "", "This endpoint does not advertise Anonymous authentication.", "Username/password selection is not implemented yet.")
		return nil
	}

	request := opcua.ConnectRequest{
		Endpoint:       m.launch.Endpoint,
		SecurityPolicy: endpoint.SecurityPolicy,
		SecurityMode:   endpoint.SecurityMode,
		AuthType:       opcua.AuthAnonymous,
	}
	m.connecting = true
	m.statusLine = fmt.Sprintf("Read-Only Mode · connecting · %s · %s", compactSecurityMode(endpoint.SecurityMode), endpoint.SecurityPolicy)
	m.details = append(selectedEndpointLines(m.endpoints, m.selectedEndpoint), "", "Connecting with Anonymous authentication…")
	return connectEndpointCmd(m.client, request)
}

func endpointDetailLines(endpoints []opcua.Endpoint, selected int) []string {
	if len(endpoints) == 0 {
		return []string{"No endpoints discovered."}
	}

	lines := []string{fmt.Sprintf("Discovered endpoints: %d", len(endpoints)), mutedStyle.Render("Select an endpoint and press Enter to connect."), ""}
	lines = append(lines, selectedEndpointLines(endpoints, selected)...)
	return lines
}

func selectedEndpointLines(endpoints []opcua.Endpoint, selected int) []string {
	lines := []string{}
	for i, endpoint := range endpoints {
		if i >= 6 {
			lines = append(lines, mutedStyle.Render(fmt.Sprintf("… %d more", len(endpoints)-i)))
			break
		}
		marker := " "
		if i == selected {
			marker = "›"
		}
		tokens := compactTokens(endpoint.UserTokenTypes)
		if tokens == "" {
			tokens = "unknown auth"
		}
		security := fmt.Sprintf("%s · %s", compactSecurityMode(endpoint.SecurityMode), endpoint.SecurityPolicy)
		lines = append(lines,
			fmt.Sprintf("%s %d. %s · %s", marker, i+1, security, tokens),
			"   "+labelStyle.Render("Auth")+": "+tokens,
		)
		if endpoint.SecurityLevel > 0 {
			lines = append(lines, "   "+labelStyle.Render("Level")+fmt.Sprintf(": %d", endpoint.SecurityLevel))
		}
		if endpoint.ServerThumbprint != "" {
			lines = append(lines, "   "+labelStyle.Render("Server cert")+": "+endpoint.ServerThumbprint)
		}
		if i != len(endpoints)-1 {
			lines = append(lines, "")
		}
	}
	return lines
}

func defaultEndpointSelection(endpoints []opcua.Endpoint) int {
	for i, endpoint := range endpoints {
		if compactSecurityMode(endpoint.SecurityMode) == "None" && endpoint.SecurityPolicy == "None" && supportsAuth(endpoint, opcua.AuthAnonymous) {
			return i
		}
	}
	for i, endpoint := range endpoints {
		if supportsAuth(endpoint, opcua.AuthAnonymous) {
			return i
		}
	}
	return 0
}

func supportsAuth(endpoint opcua.Endpoint, auth opcua.AuthType) bool {
	for _, token := range endpoint.UserTokenTypes {
		if strings.EqualFold(strings.TrimPrefix(strings.TrimSpace(token), "UserTokenType"), string(auth)) {
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
)
