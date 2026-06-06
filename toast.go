package toast

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	defaultDuration   = 5 * time.Second
	defaultMaxVisible = 3
	defaultMaxQueued  = 20
	defaultWidth      = 40
	defaultMaxHeight  = 4
	defaultGap        = 1
	defaultMargin     = 1
)

type ID string

type Kind string

const (
	KindNone    Kind = ""
	KindInfo    Kind = "info"
	KindSuccess Kind = "success"
	KindWarning Kind = "warning"
	KindError   Kind = "error"
)

type Placement int

const (
	TopLeft Placement = iota
	TopRight
	TopCenter
	BottomLeft
	BottomRight
	BottomCenter
)

type Toast struct {
	ID         ID
	Kind       Kind
	Title      string
	Message    string
	Content    string
	Duration   time.Duration
	Persistent bool
}

type ShowMsg struct{ Toast Toast }
type DismissMsg struct{ ID ID }
type DismissAllMsg struct{}

type expirationMsg struct {
	id         ID
	generation uint64
}

type RenderContext struct {
	Width     int
	MaxHeight int
	Style     lipgloss.Style
	Placement Placement
	Index     int
	Total     int
}

type Renderer func(Toast, RenderContext) string

type Theme struct {
	None    lipgloss.Style
	Info    lipgloss.Style
	Success lipgloss.Style
	Warning lipgloss.Style
	Error   lipgloss.Style
}

type Margin struct{ Top, Right, Bottom, Left int }

type entry struct {
	toast      Toast
	generation uint64
}

type Model struct {
	defaultDuration time.Duration
	maxVisible      int
	maxQueued       int
	placement       Placement
	width           int
	maxHeight       int
	gap             int
	margin          Margin
	theme           Theme
	renderer        Renderer

	visible []entry
	queued  []entry
	nextID  uint64
	nextGen uint64
	winW    int
	winH    int
}

type Option func(*Model)
type ToastOption func(*Toast)

func New(options ...Option) Model {
	m := Model{
		defaultDuration: defaultDuration,
		maxVisible:      defaultMaxVisible,
		maxQueued:       defaultMaxQueued,
		placement:       TopRight,
		width:           defaultWidth,
		maxHeight:       defaultMaxHeight,
		gap:             defaultGap,
		margin:          Margin{defaultMargin, defaultMargin, defaultMargin, defaultMargin},
		theme:           DefaultTheme(),
		nextID:          1,
		nextGen:         1,
	}
	for _, opt := range options {
		opt(&m)
	}
	m.normalize()
	return m
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowMsg:
		updated, _, cmd := m.Push(msg.Toast)
		return updated, cmd
	case DismissMsg:
		return m.Dismiss(string(msg.ID))
	case DismissAllMsg:
		return m.DismissAll()
	case expirationMsg:
		return m.expire(msg)
	case tea.WindowSizeMsg:
		m.winW, m.winH = msg.Width, msg.Height
		return m, nil
	default:
		return m, nil
	}
}

func Show(t Toast) tea.Cmd      { return func() tea.Msg { return ShowMsg{Toast: t} } }
func Dismiss(id string) tea.Cmd { return func() tea.Msg { return DismissMsg{ID: ID(id)} } }
func DismissAll() tea.Cmd       { return func() tea.Msg { return DismissAllMsg{} } }

func NewToast(message string, options ...ToastOption) Toast {
	return buildToast(message, KindNone, options...)
}
func Info(message string, options ...ToastOption) Toast {
	return buildToast(message, KindInfo, options...)
}
func Success(message string, options ...ToastOption) Toast {
	return buildToast(message, KindSuccess, options...)
}
func Warning(message string, options ...ToastOption) Toast {
	return buildToast(message, KindWarning, options...)
}
func Error(message string, options ...ToastOption) Toast {
	return buildToast(message, KindError, options...)
}

func buildToast(message string, kind Kind, options ...ToastOption) Toast {
	t := Toast{Kind: kind, Message: message}
	for _, opt := range options {
		opt(&t)
	}
	return t
}

func (m Model) Push(t Toast) (Model, ID, tea.Cmd) {
	if t.ID == "" {
		t.ID = m.generateID()
	}
	if i := indexOf(m.visible, t.ID); i >= 0 {
		m.nextGen++
		m.visible[i] = entry{toast: t, generation: m.nextGen}
		return m, t.ID, m.timer(m.visible[i])
	}
	if i := indexOf(m.queued, t.ID); i >= 0 {
		m.nextGen++
		m.queued[i] = entry{toast: t, generation: m.nextGen}
		return m, t.ID, nil
	}
	m.nextGen++
	e := entry{toast: t, generation: m.nextGen}
	if len(m.visible) < m.maxVisible {
		m.visible = append(m.visible, e)
		return m, t.ID, m.timer(e)
	}
	if m.maxQueued == 0 {
		return m, t.ID, nil
	}
	if len(m.queued) >= m.maxQueued {
		m.queued = m.queued[1:]
	}
	m.queued = append(m.queued, e)
	return m, t.ID, nil
}

func (m Model) Dismiss(id string) (Model, tea.Cmd) {
	m.visible = removeID(m.visible, ID(id))
	m.queued = removeID(m.queued, ID(id))
	return m.drain()
}

func (m Model) DismissAll() (Model, tea.Cmd) {
	m.visible = nil
	m.queued = nil
	return m, nil
}

func (m Model) Visible() []Toast {
	entries := m.renderEntries()
	out := make([]Toast, len(entries))
	for i, e := range entries {
		out[i] = e.toast
	}
	return out
}

func (m Model) Queued() []Toast {
	out := make([]Toast, len(m.queued))
	for i, e := range m.queued {
		out[i] = e.toast
	}
	return out
}

func (m Model) Len() int { return len(m.visible) + len(m.queued) }

func (m Model) expire(msg expirationMsg) (Model, tea.Cmd) {
	for _, e := range m.visible {
		if e.toast.ID == msg.id && e.generation != msg.generation {
			return m, nil
		}
	}
	m.visible = removeMatching(m.visible, msg.id, msg.generation)
	return m.drain()
}

func (m Model) drain() (Model, tea.Cmd) {
	var cmds []tea.Cmd
	for len(m.visible) < m.maxVisible && len(m.queued) > 0 {
		e := m.queued[0]
		m.queued = m.queued[1:]
		m.visible = append(m.visible, e)
		if cmd := m.timer(e); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return m, tea.Batch(cmds...)
}

func (m Model) timer(e entry) tea.Cmd {
	if e.toast.Persistent {
		return nil
	}
	d := e.toast.Duration
	if d == 0 {
		d = m.defaultDuration
	}
	return tea.Tick(d, func(time.Time) tea.Msg { return expirationMsg{id: e.toast.ID, generation: e.generation} })
}

func (m *Model) generateID() ID {
	id := ID("toast-" + strconv64(m.nextID))
	m.nextID++
	return id
}

func indexOf(entries []entry, id ID) int {
	for i, e := range entries {
		if e.toast.ID == id {
			return i
		}
	}
	return -1
}

func removeID(entries []entry, id ID) []entry {
	out := entries[:0]
	for _, e := range entries {
		if e.toast.ID != id {
			out = append(out, e)
		}
	}
	return out
}

func removeMatching(entries []entry, id ID, gen uint64) []entry {
	out := entries[:0]
	for _, e := range entries {
		if !(e.toast.ID == id && e.generation == gen) {
			out = append(out, e)
		}
	}
	return out
}

func (m *Model) normalize() {
	if m.defaultDuration <= 0 {
		m.defaultDuration = defaultDuration
	}
	if m.maxVisible <= 0 {
		m.maxVisible = defaultMaxVisible
	}
	if m.maxQueued < 0 {
		m.maxQueued = defaultMaxQueued
	}
	if m.width <= 0 {
		m.width = defaultWidth
	}
	if m.maxHeight < 0 {
		m.maxHeight = defaultMaxHeight
	}
	if m.gap < 0 {
		m.gap = defaultGap
	}
	if m.margin.Top < 0 {
		m.margin.Top = 0
	}
	if m.margin.Right < 0 {
		m.margin.Right = 0
	}
	if m.margin.Bottom < 0 {
		m.margin.Bottom = 0
	}
	if m.margin.Left < 0 {
		m.margin.Left = 0
	}
}

func WithDefaultDuration(d time.Duration) Option { return func(m *Model) { m.defaultDuration = d } }
func WithMaxVisible(n int) Option                { return func(m *Model) { m.maxVisible = n } }
func WithMaxQueued(n int) Option                 { return func(m *Model) { m.maxQueued = n } }
func WithPlacement(p Placement) Option           { return func(m *Model) { m.placement = p } }
func WithWidth(w int) Option                     { return func(m *Model) { m.width = w } }
func WithMaxHeight(h int) Option                 { return func(m *Model) { m.maxHeight = h } }
func WithGap(g int) Option                       { return func(m *Model) { m.gap = g } }
func WithOverlayMargin(top, right, bottom, left int) Option {
	return func(m *Model) { m.margin = Margin{top, right, bottom, left} }
}
func WithTheme(t Theme) Option { return func(m *Model) { m.theme = t } }
func WithStyle(kind Kind, style lipgloss.Style) Option {
	return func(m *Model) { setStyle(&m.theme, kind, style) }
}
func WithRenderer(r Renderer) Option { return func(m *Model) { m.renderer = r } }

func WithID(id string) ToastOption             { return func(t *Toast) { t.ID = ID(id) } }
func WithTitle(title string) ToastOption       { return func(t *Toast) { t.Title = title } }
func WithKind(kind Kind) ToastOption           { return func(t *Toast) { t.Kind = kind } }
func WithDuration(d time.Duration) ToastOption { return func(t *Toast) { t.Duration = d } }
func WithPersistent() ToastOption              { return func(t *Toast) { t.Persistent = true } }
func WithContent(content string) ToastOption   { return func(t *Toast) { t.Content = content } }

func strconv64(n uint64) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
