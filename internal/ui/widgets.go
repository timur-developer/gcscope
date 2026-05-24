package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/timur-developer/gcviz/internal/domain"
)

var (
	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#5f5f5f")).
			Padding(0, 1)
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#2ec4b6"))

	okStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#7cb342"))
	warnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#c9a227"))
	badStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#d64f4f"))
)

func boxed(title, body string) string {
	body = strings.TrimRight(body, "\n")

	content := titleStyle.Render(title) + "\n" + body
	return boxStyle.Render(content)
}

func boxedSized(title, body string, w, h int) string {
	if w <= 0 || h <= 0 {
		return boxed(title, body)
	}

	fx, fy := boxStyle.GetFrameSize()
	insideW := w - fx
	if insideW < 1 {
		insideW = 1
	}
	insideH := h - fy
	if insideH < 1 {
		insideH = 1
	}

	body = strings.TrimRight(body, "\n")
	lines := strings.Split(body, "\n")

	out := make([]string, 0, insideH)
	out = append(out, padRightANSI(ansi.Truncate(titleStyle.Render(title), insideW, ""), insideW))

	for _, ln := range lines {
		if len(out) >= insideH {
			break
		}
		out = append(out, padRightANSI(ansi.Truncate(ln, insideW, ""), insideW))
	}
	for len(out) < insideH {
		out = append(out, strings.Repeat(" ", insideW))
	}

	content := strings.Join(out, "\n")
	return boxStyle.Width(w).Height(h).Render(content)
}

func padRightANSI(s string, w int) string {
	sw := lipgloss.Width(s)
	if sw >= w {
		return s
	}
	return s + strings.Repeat(" ", w-sw)
}

func renderSTWBarChart(window []domain.GCEvent, cursor int, maxBars int, h int, w int) string {
	inner := InnerRect(boxStyle, Rect{W: w, H: h})
	if maxBars <= 0 {
		maxBars = inner.W
	}
	if maxBars < 10 {
		maxBars = 10
	}

	values := lastN(window, maxBars)
	if len(values) == 0 {
		return boxedSized("STW per cycle", "(max: -us)\n\n(no data)", w, h)
	}

	if cursor < 0 {
		cursor = 0
	}
	if cursor >= len(values) {
		cursor = len(values) - 1
	}

	stwUs := make([]int64, 0, len(values))
	var max int64
	for _, ev := range values {
		v := stwPerCycleUs(ev)
		stwUs = append(stwUs, v)
		if v > max {
			max = v
		}
	}

	// boxedSized already spends 1 line on the title, so body must fit into inner.H-1 lines.
	// Keep axis + cursor marker visible even on tiny terminals by shrinking the chart first
	// and dropping the "(max ...)" label when needed.
	bodyLines := inner.H - 1
	if bodyLines < 3 {
		return boxedSized("STW per cycle", "(terminal too small)", w, h)
	}
	showMax := bodyLines >= 4
	reserved := 2 // axis + marker
	if showMax {
		reserved++
	}
	chartH := bodyLines - reserved
	if chartH < 1 {
		chartH = 1
	}

	lines := renderBars(stwUs, max, chartH, cursor)
	axis := strings.Repeat("─", len(values))
	marker := renderCursorMarker(len(values), cursor)
	var body string
	if showMax {
		body = fmt.Sprintf("(max: %dµs)\n%s\n%s\n%s", max, strings.Join(lines, "\n"), axis, marker)
	} else {
		body = fmt.Sprintf("%s\n%s\n%s", strings.Join(lines, "\n"), axis, marker)
	}
	return boxedSized("STW per cycle", body, w, h)
}

func lastN(window []domain.GCEvent, n int) []domain.GCEvent {
	if n <= 0 || len(window) == 0 {
		return nil
	}
	if len(window) <= n {
		return window
	}
	return window[len(window)-n:]
}

func stwPerCycleUs(ev domain.GCEvent) int64 {
	ms := ev.STWSweepTermMs + ev.STWMarkTermMs
	return int64(math.Round(ms * 1000))
}

func renderBars(values []int64, max int64, height int, cursor int) []string {
	if height <= 0 {
		height = 1
	}
	if max <= 0 {
		max = 1
	}

	heights := make([]int, 0, len(values))
	for _, v := range values {
		h := int(math.Round(float64(v) / float64(max) * float64(height)))
		if h < 0 {
			h = 0
		}
		if h > height {
			h = height
		}
		heights = append(heights, h)
	}

	out := make([]string, 0, height)
	for row := height; row >= 1; row-- {
		var b strings.Builder
		for i, h := range heights {
			if h >= row {
				v := values[i]
				style := stwStyle(v)
				ch := "█"
				if i == cursor {
					// Use a clearly different glyph instead of reverse-video (which may render as "invisible"
					// depending on terminal theme).
					style = style.Bold(true).Underline(true)
					ch = "▓"
				}
				b.WriteString(style.Render(ch))
			} else {
				b.WriteByte(' ')
			}
		}
		out = append(out, b.String())
	}
	return out
}

func renderCursorMarker(n int, cursor int) string {
	if n <= 0 {
		return ""
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= n {
		cursor = n - 1
	}

	var b strings.Builder
	for i := 0; i < n; i++ {
		if i == cursor {
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#c0c0c0")).Bold(true).Render("^"))
		} else {
			b.WriteByte(' ')
		}
	}
	return b.String()
}

func stwStyle(us int64) lipgloss.Style {
	switch {
	case us < 200:
		return okStyle
	case us < 1000:
		return warnStyle
	default:
		return badStyle
	}
}

func stwFillStyle(us int64) lipgloss.Style {
	switch {
	case us < 200:
		return lipgloss.NewStyle().Background(lipgloss.Color("#7cb342"))
	case us < 1000:
		return lipgloss.NewStyle().Background(lipgloss.Color("#c9a227"))
	default:
		return lipgloss.NewStyle().Background(lipgloss.Color("#d64f4f"))
	}
}

func progressBar(width int, ratio float64, fill lipgloss.Style, empty lipgloss.Style) string {
	if width <= 0 {
		width = 10
	}
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}

	full := int(math.Round(ratio * float64(width)))
	if full < 0 {
		full = 0
	}
	if full > width {
		full = width
	}

	var b strings.Builder
	for i := 0; i < width; i++ {
		if i < full {
			b.WriteString(fill.Render(" "))
		} else {
			b.WriteString(empty.Render(" "))
		}
	}
	return b.String()
}

func renderCycleDetails(window []domain.GCEvent, cursor int, w, h int) string {
	if len(window) == 0 {
		return boxedSized("Cycle Details", "(no data)", w, h)
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= len(window) {
		cursor = len(window) - 1
	}

	ev := window[cursor]
	stwTotalUs := stwPerCycleUs(ev)
	stwSweepUs := int64(math.Round(ev.STWSweepTermMs * 1000))
	stwMarkUs := int64(math.Round(ev.STWMarkTermMs * 1000))

	forced := "no"
	if ev.Forced {
		forced = "yes"
	}

	body := fmt.Sprintf(
		"GC #:            %d\n"+
			"time since start: %.1fs\n"+
			"forced:          %s\n"+
			"\n"+
			"STW total (us):  %s\n"+
			"  sweep term:    %d\n"+
			"  mark term:     %d\n"+
			"\n"+
			"heap (MB):\n"+
			"  start/end:     %d/%d\n"+
			"  live/goal:     %d/%d\n"+
			"\n"+
			"gc cpu (%%):      %.1f\n"+
			"assist ratio:    %.2f\n"+
			"pages swept:     %d\n",
		ev.GCNum,
		ev.TimeSinceStartS,
		forced,
		stwStyle(stwTotalUs).Render(fmt.Sprintf("%d", stwTotalUs)),
		stwSweepUs,
		stwMarkUs,
		ev.HeapStartMB,
		ev.HeapEndMB,
		ev.HeapLiveMB,
		ev.HeapGoalMB,
		ev.GCCPUPercent,
		ev.AssistRatio,
		ev.PagesSwept,
	)

	return boxedSized("Cycle Details", body, w, h)
}
