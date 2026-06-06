package toast

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestDirectPushGeneratesIDAndMessageShowRoutesThroughModel(t *testing.T) {
	m := New(WithDefaultDuration(time.Hour))
	var id ID
	var cmd tea.Cmd
	m, id, cmd = m.Push(NewToast("direct"))
	if id == "" || m.Len() != 1 || cmd == nil {
		t.Fatalf("push id=%q len=%d cmd=%v", id, m.Len(), cmd)
	}

	msg := Show(Info("from command", WithID("cmd")))()
	m, cmd = m.Update(msg)
	if cmd == nil {
		t.Fatal("visible non-persistent Toast should schedule timer")
	}
	if got := m.Visible()[0].Message; got != "from command" {
		t.Fatalf("newest top Toast = %q", got)
	}
}

func TestQueueReplacementDismissalAndTimers(t *testing.T) {
	m := New(WithMaxVisible(1), WithMaxQueued(2), WithDefaultDuration(time.Hour))
	var cmd tea.Cmd
	m, _, cmd = m.Push(NewToast("one", WithID("one")))
	if cmd == nil {
		t.Fatal("visible Toast schedules timer")
	}
	m, _, _ = m.Push(NewToast("two", WithID("two")))
	m, _, _ = m.Push(NewToast("three", WithID("three")))
	m, _, _ = m.Push(NewToast("four", WithID("four")))
	if got := ids(m.Queued()); strings.Join(got, ",") != "three,four" {
		t.Fatalf("oldest queued dropped, got %v", got)
	}

	m, _, cmd = m.Push(NewToast("updated", WithID("one")))
	if cmd == nil || m.Visible()[0].Message != "updated" {
		t.Fatalf("visible replacement did not update and restart timer")
	}
	old := expirationMsg{id: "one", generation: m.visible[0].generation - 1}
	m, _ = m.Update(old)
	if m.Visible()[0].ID != "one" {
		t.Fatal("stale expiration dismissed replacement")
	}

	m, _, cmd = m.Push(NewToast("updated queued", WithID("four")))
	if cmd != nil {
		t.Fatal("queued replacement should not start timer")
	}
	if m.Queued()[1].Message != "updated queued" {
		t.Fatal("queued replacement did not preserve position")
	}

	m, cmd = m.Dismiss("one")
	if m.Visible()[0].ID != "three" || cmd == nil {
		t.Fatalf("queue did not drain with timer: visible=%v cmd=%v", ids(m.Visible()), cmd)
	}
	m, _ = m.Update(DismissMsg{ID: "four"})
	if m.Len() != 1 {
		t.Fatalf("dismiss should remove queued too, len=%d", m.Len())
	}
	m, _ = m.Update(DismissAllMsg{})
	if m.Len() != 0 {
		t.Fatalf("dismiss all len=%d", m.Len())
	}
}

func TestPersistentAndQueueLimits(t *testing.T) {
	m := New(WithMaxVisible(1), WithMaxQueued(0))
	m, _, cmd := m.Push(NewToast("p", WithID("p"), WithPersistent()))
	if cmd != nil {
		t.Fatal("persistent Toast should not schedule timer")
	}
	m, _, _ = m.Push(NewToast("lost", WithID("lost")))
	if m.Len() != 1 || m.Visible()[0].ID != "p" {
		t.Fatalf("persistent should count against stack and disabled queue drops new Toast")
	}

	m = New(WithMaxVisible(1), WithMaxQueued(0))
	m, _, _ = m.Push(NewToast("v", WithID("v")))
	m, _, _ = m.Push(NewToast("q", WithID("q")))
	m, _, _ = m.Push(NewToast("replacement wins", WithID("q")))
	if m.Len() != 1 {
		t.Fatalf("new identity should be dropped when queue disabled")
	}
}

func TestRenderOrderCopiesPrecedenceWrappingTruncationAndOptions(t *testing.T) {
	m := New(WithMaxVisible(3), WithPlacement(TopRight), WithWidth(10), WithMaxHeight(3), WithGap(0))
	m, _, _ = m.Push(NewToast("old", WithID("old")))
	m, _, _ = m.Push(NewToast("middle", WithID("middle")))
	m, _, _ = m.Push(NewToast("new", WithID("new")))
	if got := strings.Join(ids(m.Visible()), ","); got != "new,middle,old" {
		t.Fatalf("top render order %s", got)
	}
	copy := m.Visible()
	copy[0].Message = "mutated"
	if m.Visible()[0].Message == "mutated" {
		t.Fatal("Visible must return a copy")
	}

	view := m.View()
	if !strings.Contains(view, "new") || strings.Contains(view, "\n\n") {
		t.Fatalf("unexpected view/gap: %q", view)
	}

	m = New(WithPlacement(BottomRight), WithMaxVisible(2), WithWidth(12), WithMaxHeight(0))
	m, _, _ = m.Push(NewToast("old", WithID("old")))
	m, _, _ = m.Push(NewToast("new", WithID("new"), WithTitle("Title"), WithContent("custom content wins")))
	if got := strings.Join(ids(m.Visible()), ","); got != "old,new" {
		t.Fatalf("bottom render order %s", got)
	}
	if v := m.View(); !strings.Contains(v, "custom") || strings.Contains(v, "Title") {
		t.Fatalf("content precedence failed: %q", v)
	}

	m = New(WithRenderer(func(t Toast, c RenderContext) string { return "rendered:" + t.Message }))
	m, _, _ = m.Push(NewToast("x", WithContent("ignored")))
	if m.View() != "rendered:x" {
		t.Fatalf("renderer precedence failed: %q", m.View())
	}

	m = New(WithMaxVisible(0), WithMaxQueued(-1), WithWidth(0), WithMaxHeight(-1), WithGap(-1))
	if m.maxVisible != defaultMaxVisible || m.maxQueued != defaultMaxQueued || m.width != defaultWidth || m.maxHeight != defaultMaxHeight || m.gap != defaultGap {
		t.Fatal("invalid options not normalized")
	}
}

func TestOverlayPlacementWithExplicitAndStoredDimensions(t *testing.T) {
	style := lipgloss.NewStyle()
	m := New(WithPlacement(TopRight), WithOverlayMargin(1, 1, 1, 1), WithStyle(KindNone, style), WithWidth(5), WithMaxHeight(0))
	m, _, _ = m.Push(NewToast("hey", WithID("hey")))
	base := strings.Join([]string{"..........", "..........", "..........", ".........."}, "\n")
	out := m.OverlayWithSize(base, 10, 4)
	if !strings.Contains(strings.Split(out, "\n")[1], "hey") {
		t.Fatalf("explicit overlay did not place Toast: %q", out)
	}

	m, _ = m.Update(tea.WindowSizeMsg{Width: 10, Height: 4})
	out = m.Overlay(base)
	if !strings.Contains(out, "hey") {
		t.Fatalf("stored dimension overlay missing Toast: %q", out)
	}
	if got := m.Overlay(""); !strings.Contains(got, "hey") {
		t.Fatalf("empty base should return stack: %q", got)
	}
}

func TestOverlayFallbackDoesNotClipToastWhenBaseIsNarrowerThanStack(t *testing.T) {
	m := New(WithStyle(KindNone, lipgloss.NewStyle()), WithWidth(20), WithMaxHeight(0), WithOverlayMargin(1, 1, 1, 1))
	m, _, _ = m.Push(NewToast("this should remain visible", WithID("wide")))

	out := m.Overlay("short")

	if !strings.Contains(out, "should remain") || !strings.Contains(out, "isible") {
		t.Fatalf("fallback overlay clipped Toast to base width: %q", out)
	}
}

func TestExpirationRemovesCurrentGenerationAndDrainsQueue(t *testing.T) {
	m := New(WithMaxVisible(1), WithDefaultDuration(time.Hour))
	m, _, _ = m.Push(NewToast("one", WithID("one")))
	gen := m.visible[0].generation
	m, _, _ = m.Push(NewToast("two", WithID("two")))
	m, cmd := m.Update(expirationMsg{id: "one", generation: gen})
	if m.Visible()[0].ID != "two" || cmd == nil {
		t.Fatalf("expiration did not drain queue and schedule timer")
	}
}

func ids(ts []Toast) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = string(t.ID)
	}
	return out
}
