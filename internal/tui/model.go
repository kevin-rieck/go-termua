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

type browseChildrenMsg struct {
	ParentNodeID string
	Children     []opcua.AddressNode
	Err          error
}

type watchSubscribedMsg struct {
	NodeID       string
	Updates      <-chan opcua.LiveValue
	Subscription opcua.ValueSubscription
	Err          error
}

type watchValueMsg struct {
	NodeID string
	Value  opcua.LiveValue
	Err    error
}

type watchItem struct {
	node         opcua.AddressNode
	updates      <-chan opcua.LiveValue
	subscription opcua.ValueSubscription
	value        opcua.LiveValue
	subscribing  bool
	err          error
	updateCount  int
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
	addressSpace     *AddressSpace
	selectedTree     int
	treeScroll       int
	watchlist        []watchItem
	selectedWatch    int
	watchScroll      int
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
		addressSpace: NewAddressSpace(),
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
			} else if m.focus == focusTree {
				m.moveTreeSelection(-1)
			} else if m.focus == focusWatchlist {
				m.moveWatchSelection(-1)
			}
		case "down", "j":
			if m.hasEndpointSelection() {
				m.moveEndpointSelection(1)
			} else if m.focus == focusTree {
				m.moveTreeSelection(1)
			} else if m.focus == focusWatchlist {
				m.moveWatchSelection(1)
			}
		case "enter", "right", "l":
			if cmd := m.connectSelectedEndpoint(); cmd != nil {
				return m, cmd
			}
			if cmd := m.expandSelectedNode(); cmd != nil {
				return m, cmd
			}
		case "w":
			if cmd := m.addSelectedNodeToWatchlist(); cmd != nil {
				return m, cmd
			}
		case "left", "h":
			m.collapseSelectedNode()
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
		m.details = append(selectedEndpointLines(m.endpoints, m.selectedEndpoint), "", "Connected with Anonymous authentication.", "Loading Objects Address Space…")
		return m, m.startBrowse("i=85")
	case browseChildrenMsg:
		m.applyBrowseResult(msg)
	case watchSubscribedMsg:
		return m.applyWatchSubscribed(msg)
	case watchValueMsg:
		return m.applyWatchValue(msg)
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
	mainHeight := m.mainPanelHeight()
	watchHeight := m.watchPanelHeight(mainHeight)

	left := m.panel("Address Space", m.addressSpaceLines(mainHeight), leftWidth, mainHeight, m.focus == focusTree)
	right := m.panel("Node Details", m.details, rightWidth, mainHeight, m.focus == focusDetails)
	watchlist := m.panel("Watchlist", m.watchlistLines(watchHeight), innerWidth, watchHeight, m.focus == focusWatchlist)

	body := lipgloss.JoinVertical(lipgloss.Left,
		m.topBar(innerWidth),
		lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", gap), right),
		watchlist,
		m.footer(innerWidth),
	)
	return m.frame(body)
}

func (m Model) watchlistLines(panelHeight int) []string {
	if len(m.watchlist) == 0 {
		return []string{"No Variable Nodes added yet.", "Select a Variable Node and press w to subscribe its Live Value."}
	}

	pageSize := m.watchlistPageSize(panelHeight)
	start := clamp(m.watchScroll, 0, max(0, len(m.watchlist)-1))
	end := start + pageSize
	if end > len(m.watchlist) {
		end = len(m.watchlist)
	}

	lines := make([]string, 0, pageSize*2+1)
	if start > 0 {
		lines = append(lines, mutedStyle.Render(fmt.Sprintf("↑ %d earlier", start)))
	}
	for i, item := range m.watchlist[start:end] {
		watchIndex := start + i
		state := "subscribing…"
		if item.err != nil {
			state = "subscription failed: " + item.err.Error()
		} else if item.value.Value != "" {
			stamp := compactTimestamp(item.value.SourceTimestamp)
			if stamp == "" {
				stamp = compactTimestamp(item.value.ServerTimestamp)
			}
			state = fmt.Sprintf("%s · %s", ellipsize(item.value.Value, 48), compactStatus(item.value.Status))
			if stamp != "" {
				state += " · " + stamp
			}
		} else if !item.subscribing {
			state = "waiting for first Live Value…"
		}
		marker := "•"
		if m.focus == focusWatchlist && watchIndex == m.selectedWatch {
			marker = "›"
		}
		lines = append(lines, fmt.Sprintf("%s %s = %s", marker, item.node.DisplayName, state), "  "+mutedStyle.Render(ellipsize(item.node.NodeID, 72)))
	}
	if end < len(m.watchlist) {
		lines = append(lines, mutedStyle.Render(fmt.Sprintf("↓ %d more", len(m.watchlist)-end)))
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
	if !m.connected {
		lines = append(lines, mutedStyle.Render("  Connect to start lazy browsing"))
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
	} else if m.focus == focusTree && m.connected {
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
	body := wrapLines(lines, contentWidth, contentHeight-1)
	return style.Render(panelTitleStyle.Render(title) + "\n" + body)
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

func subscribeValueCmd(client opcua.Client, nodeID string) tea.Cmd {
	return func() tea.Msg {
		log.Printf("watchlist subscription started: nodeID=%s", nodeID)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		updates, subscription, err := client.SubscribeValue(ctx, nodeID)
		return watchSubscribedMsg{NodeID: nodeID, Updates: updates, Subscription: subscription, Err: err}
	}
}

func waitForWatchValueCmd(nodeID string, updates <-chan opcua.LiveValue) tea.Cmd {
	return func() tea.Msg {
		value, ok := <-updates
		if !ok {
			return watchValueMsg{NodeID: nodeID, Err: fmt.Errorf("subscription closed")}
		}
		return watchValueMsg{NodeID: nodeID, Value: value}
	}
}

func (m *Model) moveTreeSelection(delta int) {
	visible := m.addressSpace.View()
	if len(visible) == 0 {
		return
	}
	m.selectedTree = (m.selectedTree + delta + len(visible)) % len(visible)
	m.ensureSelectedTreeVisible(len(visible), m.addressTreePageSize(m.mainPanelHeight()))
	m.details = nodeDetailLines(visible[m.selectedTree].Node)
}

func (m *Model) moveWatchSelection(delta int) {
	if len(m.watchlist) == 0 {
		m.selectedWatch = 0
		m.watchScroll = 0
		return
	}
	m.selectedWatch = (m.selectedWatch + delta + len(m.watchlist)) % len(m.watchlist)
	m.ensureSelectedWatchVisible(m.watchlistPageSize(m.watchPanelHeight(m.mainPanelHeight())))
}

func (m *Model) expandSelectedNode() tea.Cmd {
	if !m.connected || m.client == nil {
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
	m.statusLine = "Read-Only Mode · browsing Address Space"
	return browseChildrenCmd(m.client, nodeID)
}

func (m *Model) collapseSelectedNode() {
	visible := m.addressSpace.View()
	if m.selectedTree >= 0 && m.selectedTree < len(visible) {
		m.addressSpace.Collapse(visible[m.selectedTree].Node.NodeID)
	}
}

func (m *Model) addSelectedNodeToWatchlist() tea.Cmd {
	if !m.connected || m.client == nil {
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
	if m.watchIndexByNodeID(node.NodeID) >= 0 {
		m.statusLine = "Read-Only Mode · Variable Node already on Watchlist"
		return nil
	}
	m.watchlist = append(m.watchlist, watchItem{node: node, subscribing: true})
	m.selectedWatch = len(m.watchlist) - 1
	m.ensureSelectedWatchVisible(m.watchlistPageSize(m.watchPanelHeight(m.mainPanelHeight())))
	m.statusLine = "Read-Only Mode · subscribing Watchlist node"
	return subscribeValueCmd(m.client, node.NodeID)
}

func (m Model) applyWatchSubscribed(msg watchSubscribedMsg) (tea.Model, tea.Cmd) {
	idx := m.watchIndexByNodeID(msg.NodeID)
	if idx < 0 {
		return m, nil
	}
	m.watchlist[idx].subscribing = false
	if msg.Err != nil {
		log.Printf("watchlist subscription failed: nodeID=%s error=%v", msg.NodeID, msg.Err)
		m.watchlist[idx].err = msg.Err
		m.statusLine = "Read-Only Mode · Watchlist subscription failed"
		return m, nil
	}
	m.watchlist[idx].updates = msg.Updates
	m.watchlist[idx].subscription = msg.Subscription
	m.statusLine = "Read-Only Mode · Watchlist subscription active"
	return m, waitForWatchValueCmd(msg.NodeID, msg.Updates)
}

func (m Model) applyWatchValue(msg watchValueMsg) (tea.Model, tea.Cmd) {
	idx := m.watchIndexByNodeID(msg.NodeID)
	if idx < 0 {
		return m, nil
	}
	if msg.Err != nil {
		m.watchlist[idx].err = msg.Err
		m.statusLine = "Read-Only Mode · Watchlist subscription closed"
		return m, nil
	}
	m.watchlist[idx].value = msg.Value
	m.watchlist[idx].err = nil
	m.watchlist[idx].updateCount++
	m.statusLine = fmt.Sprintf("Read-Only Mode · Watchlist updated · %s", m.watchlist[idx].node.DisplayName)
	return m, waitForWatchValueCmd(msg.NodeID, m.watchlist[idx].updates)
}

func (m Model) watchIndexByNodeID(nodeID string) int {
	for i, item := range m.watchlist {
		if item.node.NodeID == nodeID {
			return i
		}
	}
	return -1
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
	if len(m.watchlist) == 0 {
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
	if m.selectedWatch >= len(m.watchlist) {
		m.selectedWatch = len(m.watchlist) - 1
	}
	if m.selectedWatch < m.watchScroll {
		m.watchScroll = m.selectedWatch
	}
	if m.selectedWatch >= m.watchScroll+pageSize {
		m.watchScroll = m.selectedWatch - pageSize + 1
	}
	maxScroll := len(m.watchlist) - pageSize
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
	return clamp(m.height-12, 12, 24)
}

func (m Model) watchPanelHeight(mainHeight int) int {
	if mainHeight < 16 {
		return 6
	}
	return 9
}

func (m Model) addressTreePageSize(panelHeight int) int {
	bodyLines := panelHeight - panelStyle.GetVerticalFrameSize() - 1
	fixedLines := 2 // blank line + Search hint
	if m.launch.Endpoint != "" {
		fixedLines += 2
	}
	if !m.connected {
		fixedLines++
	}
	pageSize := bodyLines - fixedLines
	if pageSize < 1 {
		return 1
	}
	return pageSize
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
		lines = append(lines, "", "Live Value loading is next: value, health, timestamps, data type, and Engineering Unit.")
	}
	return lines
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

