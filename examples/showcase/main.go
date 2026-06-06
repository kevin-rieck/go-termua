package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kevin-rieck/go-bubble-toast"
)

type model struct {
	toasts toast.Model
	stage  int
	label  string
}

func main() {
	m := model{}
	m.configureStage()
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func (m model) Init() tea.Cmd { return m.stageCmd() }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.toasts, cmd = m.toasts.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "n":
			m.stage = (m.stage + 1) % 8
			m.configureStage()
			return m, tea.Batch(cmd, m.stageCmd())
		case "d":
			var dismiss tea.Cmd
			m.toasts, dismiss = m.toasts.Dismiss("persistent")
			return m, tea.Batch(cmd, dismiss)
		}
	}
	return m, cmd
}

func (m model) View() string {
	base := lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Render("Bubble Toast feature showcase") + "\n\n" +
		lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Render(m.label) + "\n\n" +
		"n: next feature   d: dismiss persistent Toast   q: quit\n\n" +
		strings.Join([]string{
			"Features covered in this GIF:",
			"• message-based Show and explicit Update routing",
			"• direct model options: placement, width, height, gap, margins",
			"• info/success/warning/error/neutral Toast Kinds",
			"• title+message rendering and pre-rendered Content precedence",
			"• generated and caller-provided Toast IDs",
			"• replacement by Toast ID and explicit Dismissal",
			"• visible stack limit, queued overflow, queue draining",
			"• persistent Toasts and custom renderers",
		}, "\n") + strings.Repeat("\n", 12)
	return m.toasts.Overlay(base)
}

func (m *model) configureStage() {
	switch m.stage {
	case 0:
		m.label = "Kinds + generated IDs: neutral, info, success, warning, and error Toasts use default styles."
		m.toasts = toast.New(toast.WithPlacement(toast.TopRight), toast.WithMaxVisible(5), toast.WithWidth(36), toast.WithGap(0))
	case 1:
		m.label = "Content: title/message rendering, then pre-rendered Content replacing both."
		m.toasts = toast.New(toast.WithPlacement(toast.TopRight), toast.WithMaxVisible(2), toast.WithWidth(42), toast.WithMaxHeight(0))
	case 2:
		m.label = "Stable Toast ID: showing the same ID replaces content and restarts the visible timer."
		m.toasts = toast.New(toast.WithPlacement(toast.TopRight), toast.WithWidth(44))
	case 3:
		m.label = "Queueing: max visible is 2; overflow queues and fills slots after Dismissal."
		m.toasts = toast.New(toast.WithPlacement(toast.BottomRight), toast.WithMaxVisible(2), toast.WithMaxQueued(3), toast.WithWidth(34))
	case 4:
		m.label = "Placement + layout: bottom-center, margins, wrapping, max height, and ellipsis truncation."
		m.toasts = toast.New(toast.WithPlacement(toast.BottomCenter), toast.WithWidth(32), toast.WithMaxHeight(4), toast.WithOverlayMargin(1, 2, 2, 2))
	case 5:
		m.label = "Placement preset: top-center keeps newest Toasts closest to the top edge."
		m.toasts = toast.New(toast.WithPlacement(toast.TopCenter), toast.WithMaxVisible(2), toast.WithWidth(34), toast.WithGap(0), toast.WithOverlayMargin(14, 1, 1, 1))
	case 6:
		m.label = "Placement preset: bottom-left keeps newest Toasts closest to the bottom edge."
		m.toasts = toast.New(toast.WithPlacement(toast.BottomLeft), toast.WithMaxVisible(2), toast.WithWidth(34), toast.WithOverlayMargin(1, 1, 2, 4))
	case 7:
		m.label = "Custom renderer + persistent Toast: renderer owns presentation; press d to dismiss."
		m.toasts = toast.New(toast.WithPlacement(toast.TopLeft), toast.WithMaxVisible(2), toast.WithWidth(46), toast.WithOverlayMargin(15, 1, 1, 4), toast.WithRenderer(customRenderer))
	}
}

func (m model) stageCmd() tea.Cmd {
	switch m.stage {
	case 0:
		return tea.Sequence(
			toast.Show(toast.NewToast("Neutral Toast", toast.WithDuration(10*time.Second))),
			toast.Show(toast.Info("Info Toast", toast.WithDuration(10*time.Second))),
			toast.Show(toast.Success("Success Toast", toast.WithDuration(10*time.Second))),
			toast.Show(toast.Warning("Warning Toast", toast.WithDuration(10*time.Second))),
			toast.Show(toast.Error("Error Toast", toast.WithDuration(10*time.Second))),
		)
	case 1:
		content := lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Bold(true).Render("pre-rendered Lip Gloss Content wins")
		return tea.Sequence(
			toast.Show(toast.Info("body message", toast.WithTitle("Separate title"), toast.WithDuration(10*time.Second))),
			toast.Show(toast.Success("ignored message", toast.WithTitle("Ignored title"), toast.WithContent(content), toast.WithDuration(10*time.Second))),
		)
	case 2:
		return tea.Sequence(
			toast.Show(toast.Warning("Uploading… 10%", toast.WithID("upload"), toast.WithDuration(10*time.Second))),
			tea.Tick(700*time.Millisecond, func(time.Time) tea.Msg {
				return toast.ShowMsg{Toast: toast.Success("Uploading… 100%", toast.WithID("upload"), toast.WithTitle("Replaced same Toast ID"), toast.WithDuration(10*time.Second))}
			}),
		)
	case 3:
		return tea.Sequence(
			toast.Show(toast.Info("visible #1", toast.WithID("one"), toast.WithPersistent())),
			toast.Show(toast.Success("visible #2", toast.WithID("two"), toast.WithPersistent())),
			toast.Show(toast.Warning("queued #3", toast.WithID("three"), toast.WithPersistent())),
			toast.Show(toast.Error("queued #4", toast.WithID("four"), toast.WithPersistent())),
			tea.Tick(1400*time.Millisecond, func(time.Time) tea.Msg { return toast.DismissMsg{ID: "one"} }),
		)
	case 4:
		return toast.Show(toast.Info("A long Toast wraps to the configured width and is truncated to the configured maximum height with an ellipsis.", toast.WithTitle("Bottom center layout"), toast.WithDuration(10*time.Second)))
	case 5:
		return tea.Sequence(
			toast.Show(toast.Info("top-center older", toast.WithDuration(10*time.Second))),
			toast.Show(toast.Success("top-center newest", toast.WithDuration(10*time.Second))),
		)
	case 6:
		return tea.Sequence(
			toast.Show(toast.Warning("bottom-left older", toast.WithDuration(10*time.Second))),
			toast.Show(toast.Error("bottom-left newest", toast.WithDuration(10*time.Second))),
		)
	case 7:
		return tea.Sequence(
			toast.Show(toast.NewToast("persistent custom Toast", toast.WithID("persistent"), toast.WithPersistent())),
			toast.Show(toast.NewToast("secondary custom Toast", toast.WithDuration(10*time.Second))),
		)
	default:
		return nil
	}
}

func customRenderer(t toast.Toast, ctx toast.RenderContext) string {
	box := lipgloss.NewStyle().
		Width(ctx.Width-2).
		Padding(0, 1).
		Border(lipgloss.ThickBorder()).
		BorderForeground(lipgloss.Color("99")).
		Foreground(lipgloss.Color("225"))
	return box.Render(fmt.Sprintf("CUSTOM %d/%d · %s", ctx.Index+1, ctx.Total, t.Message))
}
