package toast

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func DefaultTheme() Theme {
	base := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	return Theme{
		None:    base.BorderForeground(lipgloss.Color("245")),
		Info:    base.BorderForeground(lipgloss.Color("39")),
		Success: base.BorderForeground(lipgloss.Color("42")),
		Warning: base.BorderForeground(lipgloss.Color("214")),
		Error:   base.BorderForeground(lipgloss.Color("203")),
	}
}

func (m Model) View() string {
	entries := m.renderEntries()
	if len(entries) == 0 {
		return ""
	}
	parts := make([]string, len(entries))
	for i, e := range entries {
		parts[i] = m.renderToast(e.toast, i, len(entries))
	}
	sep := strings.Repeat("\n", m.gap+1)
	return strings.Join(parts, sep)
}

func (m Model) Overlay(base string) string {
	w, h := m.winW, m.winH
	if w <= 0 || h <= 0 {
		w, h = lipgloss.Width(base), lipgloss.Height(base)
	}
	return m.OverlayWithSize(base, w, h)
}

func (m Model) OverlayWithSize(base string, width, height int) string {
	stack := m.View()
	if stack == "" {
		return base
	}
	if base == "" || width <= 0 || height <= 0 {
		return stack
	}
	canvas := normalizeCanvas(base, width, height)
	sw, sh := lipgloss.Width(stack), lipgloss.Height(stack)
	x := m.margin.Left
	switch m.placement {
	case TopRight, BottomRight:
		x = width - m.margin.Right - sw
	case TopCenter, BottomCenter:
		x = (width - sw) / 2
	}
	y := m.margin.Top
	switch m.placement {
	case BottomLeft, BottomRight, BottomCenter:
		y = height - m.margin.Bottom - sh
	}
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	return overlayAt(canvas, stack, x, y)
}

func (m Model) renderEntries() []entry {
	entries := append([]entry(nil), m.visible...)
	if isTop(m.placement) {
		for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
			entries[i], entries[j] = entries[j], entries[i]
		}
	}
	return entries
}

func isTop(p Placement) bool { return p == TopLeft || p == TopRight || p == TopCenter }

func (m Model) renderToast(t Toast, index, total int) string {
	style := m.styleFor(t.Kind).Width(m.width)
	ctx := RenderContext{Width: m.width, MaxHeight: m.maxHeight, Style: style, Placement: m.placement, Index: index, Total: total}
	if m.renderer != nil {
		return m.renderer(t, ctx)
	}
	body := t.Content
	if body == "" {
		if t.Title != "" {
			body = lipgloss.NewStyle().Bold(true).Render(t.Title)
		}
		if t.Message != "" {
			if body != "" {
				body += "\n"
			}
			body += t.Message
		}
	}
	innerWidth := m.width - style.GetHorizontalFrameSize()
	if innerWidth < 1 {
		innerWidth = 1
	}
	body = wrap(body, innerWidth)
	rendered := style.Render(body)
	if m.maxHeight > 0 && lipgloss.Height(rendered) > m.maxHeight {
		rendered = truncateHeight(rendered, m.maxHeight)
	}
	return rendered
}

func (m Model) styleFor(kind Kind) lipgloss.Style {
	switch kind {
	case KindInfo:
		return m.theme.Info
	case KindSuccess:
		return m.theme.Success
	case KindWarning:
		return m.theme.Warning
	case KindError:
		return m.theme.Error
	default:
		return m.theme.None
	}
}

func setStyle(t *Theme, kind Kind, style lipgloss.Style) {
	switch kind {
	case KindInfo:
		t.Info = style
	case KindSuccess:
		t.Success = style
	case KindWarning:
		t.Warning = style
	case KindError:
		t.Error = style
	default:
		t.None = style
	}
}

func wrap(s string, width int) string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		for lipgloss.Width(line) > width {
			cut := width
			for cut > 0 && lipgloss.Width(line[:cut]) > width {
				cut--
			}
			if cut <= 0 {
				cut = 1
			}
			out = append(out, line[:cut])
			line = line[cut:]
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func truncateHeight(s string, max int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= max {
		return s
	}
	lines = lines[:max]
	last := lines[max-1]
	w := lipgloss.Width(last)
	if w == 0 {
		lines[max-1] = "…"
	} else if len(last) > 0 {
		lines[max-1] = last[:len(last)-1] + "…"
	}
	return strings.Join(lines, "\n")
}

func normalizeCanvas(base string, width, height int) []string {
	lines := strings.Split(base, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	for i, line := range lines {
		pad := width - lipgloss.Width(line)
		if pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		lines[i] = line
	}
	return lines
}

func overlayAt(canvas []string, stack string, x, y int) string {
	lines := strings.Split(stack, "\n")
	for i, line := range lines {
		row := y + i
		if row < 0 || row >= len(canvas) {
			continue
		}
		canvas[row] = replaceAt(canvas[row], line, x)
	}
	return strings.TrimRight(strings.Join(canvas, "\n"), " ")
}

func replaceAt(base, over string, x int) string {
	if x >= len(base) {
		return base
	}
	if x < 0 {
		x = 0
	}
	end := x + len(over)
	if end > len(base) {
		end = len(base)
	}
	return base[:x] + over[:end-x] + base[end:]
}
