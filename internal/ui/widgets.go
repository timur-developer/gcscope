package ui

import (
	"fmt"
	"math"
	"strconv"
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

	gcLabelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#5f5f5f"))
	heapLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#7aa2f7"))
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

	// lipgloss.Style.Width/Height() apply *before* borders. I.e. the final rendered
	// box size is (width/height) + (border sizes) + (margins). We work in "outer"
	// coordinates here (w/h are the desired final sizes), so we must subtract
	// border sizes when setting Width/Height to avoid trimming the right/bottom
	// border on render.
	//
	// FrameSize includes padding + border (+ margins). "Inside" is content area.
	fx, fy := boxStyle.GetFrameSize()
	bx := boxStyle.GetHorizontalBorderSize()
	by := boxStyle.GetVerticalBorderSize()
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
	preBorderW := w - bx
	if preBorderW < 1 {
		preBorderW = 1
	}
	preBorderH := h - by
	if preBorderH < 1 {
		preBorderH = 1
	}
	return boxStyle.Width(preBorderW).Height(preBorderH).Render(content)
}

func padRightANSI(s string, w int) string {
	sw := lipgloss.Width(s)
	if sw >= w {
		return s
	}
	return s + strings.Repeat(" ", w-sw)
}

func stwBarsCapacity(innerW int) (barWidth int, barGap int, bars int) {
	if innerW <= 0 {
		return 1, 0, 0
	}

	// Prefer thicker bars, but keep enough history visible.
	const minBars = 18

	candidates := [][2]int{
		// Prefer a small gap when it still leaves enough history visible.
		{3, 1},
		{2, 1},
		{1, 1},
		{3, 0},
		{2, 0},
		{1, 0},
	}

	for idx, c := range candidates {
		w := c[0]
		g := c[1]
		if innerW < w {
			continue
		}

		den := w + g
		if den <= 0 {
			den = 1
		}

		// bars*w + (bars-1)*g <= innerW  =>  bars <= (innerW + g) / (w + g)
		b := (innerW + g) / den
		if b < 1 {
			b = 1
		}

		if b >= minBars || idx == len(candidates)-1 {
			return w, g, b
		}
	}

	return 1, 0, 1
}

type barData struct {
	gcNum    int
	totalUs  int64
	sweepUs  int64
	markUs   int64
	heapLive int
}

func renderSTWBarChart(window []domain.GCEvent, cursor int, mode stwLabelMode, maxBars int, h int, w int) string {
	inner := InnerRect(boxStyle, Rect{W: w, H: h})
	barW, barGap, capBars := stwBarsCapacity(inner.W)
	if maxBars > 0 && capBars > maxBars {
		capBars = maxBars
	}

	values := lastN(window, capBars)
	if len(values) == 0 {
		return boxedSized("STW per cycle", "(no data)", w, h)
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= len(values) {
		cursor = len(values) - 1
	}

	bars := make([]barData, 0, len(values))
	var maxTotal int64
	for _, ev := range values {
		sweepUs := int64(math.Round(ev.STWSweepTermMs * 1000))
		markUs := int64(math.Round(ev.STWMarkTermMs * 1000))
		total := sweepUs + markUs
		bars = append(bars, barData{
			gcNum:    ev.GCNum,
			totalUs:  total,
			sweepUs:  sweepUs,
			markUs:   markUs,
			heapLive: ev.HeapLiveMB,
		})
		if total > maxTotal {
			maxTotal = total
		}
	}
	if maxTotal <= 0 {
		maxTotal = 1
	}

	bodyLines := inner.H - 1
	if bodyLines < 3 {
		return boxedSized("STW per cycle", "(terminal too small)", w, h)
	}

	labelLinesWanted := 2
	if mode == stwLabelGCOnly {
		labelLinesWanted = 1
	}
	showSelectedOnly := false
	if barW <= 1 {
		// Full per-bar labels become unreadable noise when bars are 1 cell wide.
		// Keep a single "selected" label line instead.
		labelLinesWanted = 1
		showSelectedOnly = true
	}

	// If values won't fit into per-bar slots, or when bars have no gap (labels would "glue"
	// into unreadable long numbers), fall back to a cursor-only value label line.
	valueOverlay := false
	if !showSelectedOnly && labelLinesWanted >= 2 && len(bars) > 0 {
		if barGap == 0 && mode != stwLabelGCOnly {
			valueOverlay = true
		}

		maxLen := 0
		switch mode {
		case stwLabelGCAndSTW:
			for _, bd := range bars {
				n := len(strconv.FormatInt(bd.totalUs, 10))
				if n > maxLen {
					maxLen = n
				}
			}
		case stwLabelGCAndHeap:
			for _, bd := range bars {
				n := len(strconv.Itoa(bd.heapLive))
				if n > maxLen {
					maxLen = n
				}
			}
		}
		if maxLen > barW {
			valueOverlay = true
		}
	}

	// When there's no inter-bar gap, x-axis labels become hard to read ("101112...").
	// Show GC labels more sparsely to keep the axis legible.
	gcStep := 1
	if !showSelectedOnly && barGap == 0 {
		gcStep = 2
	}

	// Reserve bottom lines: axis + cursor + labels.
	axisLines := 2 // axis + cursor marker
	labels := labelLinesWanted
	if bodyLines < axisLines+labels+1 {
		// Not enough space: drop label lines first.
		if bodyLines >= axisLines+1 {
			labels = 0
		}
	}

	showHeader := bodyLines >= axisLines+labels+2
	reserved := axisLines + labels
	if showHeader {
		reserved++
	}

	chartH := bodyLines - reserved
	if chartH < 1 {
		chartH = 1
	}

	chart := renderSTWStackedBars(bars, maxTotal, chartH, cursor, barW, barGap)
	axis := renderBarAxis(len(bars), barW, barGap)
	cursorMarker := renderBarCursorMarker(len(bars), cursor, barW, barGap)
	label1, label2 := renderSTWLabels(bars, mode, cursor, barW, barGap, gcStep, showSelectedOnly, valueOverlay, inner.W)

	lines := make([]string, 0, bodyLines)
	if showHeader {
		sel := bars[cursor]
		selText := ""
		switch mode {
		case stwLabelGCAndSTW:
			selText = fmt.Sprintf("sel: #%d %s", sel.gcNum, formatUs(sel.totalUs))
		case stwLabelGCAndHeap:
			selText = fmt.Sprintf("sel: #%d %dMB", sel.gcNum, sel.heapLive)
		default:
			selText = fmt.Sprintf("sel: #%d", sel.gcNum)
		}

		header := fmt.Sprintf("(max: %s) legend: %s %s",
			formatUs(maxTotal),
			lipgloss.NewStyle().Foreground(lipgloss.Color("#c9a227")).Render("sweep"),
			lipgloss.NewStyle().Foreground(lipgloss.Color("#d64f4f")).Render("mark"),
		)
		// Try to include selected value if it fits; otherwise keep the header compact.
		if inner.W > 0 {
			candidate := header + "  " + selText
			if lipgloss.Width(candidate) <= inner.W {
				header = candidate
			}
		}
		lines = append(lines, header)
	}
	lines = append(lines, chart...)
	lines = append(lines, axis, cursorMarker)
	if labels >= 1 {
		lines = append(lines, label1)
	}
	if labels >= 2 {
		lines = append(lines, label2)
	}

	return boxedSized("STW per cycle", strings.Join(lines, "\n"), w, h)
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

func renderBarAxis(n int, barW int, gap int) string {
	if n <= 0 {
		return ""
	}
	var b strings.Builder
	for i := 0; i < n; i++ {
		if i > 0 && gap > 0 {
			b.WriteString(strings.Repeat(" ", gap))
		}
		b.WriteString(strings.Repeat("─", barW))
	}
	return b.String()
}

func renderBarCursorMarker(n int, cursor int, barW int, gap int) string {
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
		if i > 0 && gap > 0 {
			b.WriteString(strings.Repeat(" ", gap))
		}
		if i == cursor {
			left := barW / 2
			right := barW - left - 1
			b.WriteString(strings.Repeat(" ", left))
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#c0c0c0")).Bold(true).Render("^"))
			b.WriteString(strings.Repeat(" ", right))
		} else {
			b.WriteString(strings.Repeat(" ", barW))
		}
	}
	return b.String()
}

func renderSTWStackedBars(bars []barData, maxTotal int64, height int, cursor int, barW int, gap int) []string {
	if height < 1 {
		height = 1
	}
	if maxTotal <= 0 {
		maxTotal = 1
	}

	sweepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#c9a227"))
	markStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#d64f4f"))

	type seg struct{ sweep, mark int }
	segs := make([]seg, 0, len(bars))
	for _, bd := range bars {
		totalH := int(math.Round(float64(bd.totalUs) / float64(maxTotal) * float64(height)))
		if totalH < 0 {
			totalH = 0
		}
		if totalH > height {
			totalH = height
		}
		var sweepH int
		if bd.totalUs > 0 {
			sweepH = int(math.Round(float64(bd.sweepUs) / float64(bd.totalUs) * float64(totalH)))
		}
		if sweepH < 0 {
			sweepH = 0
		}
		if sweepH > totalH {
			sweepH = totalH
		}
		markH := totalH - sweepH
		segs = append(segs, seg{sweep: sweepH, mark: markH})
	}

	out := make([]string, 0, height)
	for row := height; row >= 1; row-- {
		var b strings.Builder
		for i := range bars {
			if i > 0 && gap > 0 {
				b.WriteString(strings.Repeat(" ", gap))
			}

			s := segs[i]
			var ch string
			var style lipgloss.Style
			if row <= s.sweep {
				ch = "█"
				style = sweepStyle
			} else if row <= s.sweep+s.mark {
				ch = "█"
				style = markStyle
			} else {
				ch = " "
				style = lipgloss.NewStyle()
			}

			cell := strings.Repeat(ch, barW)
			if i == cursor {
				style = style.Bold(true).Underline(true)
				// Make the cursor bar visually distinct even when it's empty.
				if strings.TrimSpace(cell) == "" {
					cell = strings.Repeat("░", barW)
					style = lipgloss.NewStyle().Foreground(lipgloss.Color("#5f5f5f")).Bold(true).Underline(true)
				}
			}
			b.WriteString(style.Render(cell))
		}
		out = append(out, b.String())
	}
	return out
}

func renderSTWLabels(bars []barData, mode stwLabelMode, cursor int, barW int, gap int, gcStep int, selectedOnly bool, valueOverlay bool, totalW int) (string, string) {
	if selectedOnly {
		if len(bars) == 0 {
			return "", ""
		}
		if cursor < 0 {
			cursor = 0
		}
		if cursor >= len(bars) {
			cursor = len(bars) - 1
		}

		sel := bars[cursor]
		var label string
		switch mode {
		case stwLabelGCAndSTW:
			label = fmt.Sprintf("#%d %dus", sel.gcNum, sel.totalUs)
			label = stwStyle(sel.totalUs).Render(label)
		case stwLabelGCAndHeap:
			label = fmt.Sprintf("#%d %dMB", sel.gcNum, sel.heapLive)
			label = heapLabelStyle.Render(label)
		default:
			label = fmt.Sprintf("#%d", sel.gcNum)
		}

		axisW := len(bars)*barW + (len(bars)-1)*gap
		if axisW < 0 {
			axisW = 0
		}
		if totalW > 0 && axisW > totalW {
			axisW = totalW
		}

		pos := cursor * (barW + gap)
		if pos < 0 {
			pos = 0
		}
		if axisW > 0 {
			labelW := lipgloss.Width(label)
			if labelW < axisW {
				maxPos := axisW - labelW
				if pos > maxPos {
					pos = maxPos
				}
			}
		}
		if axisW > 0 && pos >= axisW {
			pos = axisW - 1
		}

		base := strings.Repeat(" ", axisW)
		out := overlayAt(base, label, pos, axisW)
		return out, ""
	}

	line1Parts := make([]string, 0, len(bars))
	line2Parts := make([]string, 0, len(bars))

	if gcStep < 1 {
		gcStep = 1
	}

	for i, bd := range bars {
		gc := ""
		showGC := true
		if gcStep > 1 && i%gcStep != 0 && i != cursor {
			showGC = false
		}
		if showGC {
			gc = formatGCLabel(bd.gcNum, barW)
			gc = gcLabelStyle.Render(centerTrunc(gc, barW))
		} else {
			gc = strings.Repeat(" ", barW)
		}
		line1Parts = append(line1Parts, gc)

		if valueOverlay {
			line2Parts = append(line2Parts, strings.Repeat(" ", barW))
			continue
		}

		switch mode {
		case stwLabelGCAndSTW:
			raw := formatIntIfFits(bd.totalUs, barW)
			raw = centerTrunc(raw, barW)
			line2Parts = append(line2Parts, stwStyle(bd.totalUs).Render(raw))
		case stwLabelGCAndHeap:
			raw := formatIntIfFits(int64(bd.heapLive), barW)
			raw = centerTrunc(raw, barW)
			line2Parts = append(line2Parts, heapLabelStyle.Render(raw))
		default:
			line2Parts = append(line2Parts, strings.Repeat(" ", barW))
		}
	}

	join := func(parts []string) string {
		if len(parts) == 0 {
			return ""
		}
		return strings.Join(parts, strings.Repeat(" ", gap))
	}

	line1 := join(line1Parts)
	line2 := join(line2Parts)

	if valueOverlay && len(bars) > 0 && mode != stwLabelGCOnly {
		if cursor < 0 {
			cursor = 0
		}
		if cursor >= len(bars) {
			cursor = len(bars) - 1
		}

		sel := bars[cursor]
		var label string
		switch mode {
		case stwLabelGCAndSTW:
			label = fmt.Sprintf("%dus", sel.totalUs)
			label = stwStyle(sel.totalUs).Render(label)
		case stwLabelGCAndHeap:
			label = fmt.Sprintf("%dMB", sel.heapLive)
			label = heapLabelStyle.Render(label)
		}

		axisW := len(bars)*barW + (len(bars)-1)*gap
		if axisW < 0 {
			axisW = 0
		}
		if totalW > 0 && axisW > totalW {
			axisW = totalW
		}
		pos := cursor * (barW + gap)
		if pos < 0 {
			pos = 0
		}
		if axisW > 0 {
			labelW := lipgloss.Width(label)
			if labelW < axisW {
				maxPos := axisW - labelW
				if pos > maxPos {
					pos = maxPos
				}
			}
		}
		if axisW > 0 && pos >= axisW {
			pos = axisW - 1
		}
		base := strings.Repeat(" ", axisW)
		line2 = overlayAt(base, label, pos, axisW)
	}

	return line1, line2
}

func formatGCLabel(gcNum int, barW int) string {
	if barW <= 0 {
		return ""
	}
	if barW >= 4 {
		return fmt.Sprintf("#%d", gcNum)
	}

	mod := 1
	for i := 0; i < barW; i++ {
		mod *= 10
	}
	if mod <= 0 {
		mod = 10
	}
	v := gcNum % mod
	if v < 0 {
		v = -v
	}
	return strconv.Itoa(v)
}

func formatIntIfFits(v int64, w int) string {
	if w <= 0 {
		return ""
	}
	if v < 0 {
		v = 0
	}
	s := strconv.FormatInt(v, 10)
	if lipgloss.Width(s) > w {
		return ""
	}
	return s
}

func overlayAt(base string, s string, pos int, limit int) string {
	if limit <= 0 {
		limit = lipgloss.Width(base)
	}
	if pos < 0 {
		pos = 0
	}
	prefix := ""
	if pos > 0 {
		prefix = strings.Repeat(" ", pos)
	}
	out := prefix + s
	if limit > 0 {
		out = ansi.Truncate(out, limit, "")
	}
	return out
}

func centerTrunc(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = ansi.Truncate(s, w, "")
	sw := lipgloss.Width(s)
	if sw >= w {
		return s
	}
	pad := w - sw
	left := pad / 2
	right := pad - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

func formatUs(us int64) string {
	if us < 0 {
		us = 0
	}
	if us >= 1000 {
		ms := float64(us) / 1000.0
		if ms < 10 {
			return fmt.Sprintf("%.1fms", ms)
		}
		return fmt.Sprintf("%.0fms", ms)
	}
	return fmt.Sprintf("%dus", us)
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
