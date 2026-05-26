package ui

import (
	"fmt"
	"math"
	"time"

	tslc "github.com/NimbleMarkets/ntcharts/linechart/timeserieslinechart"
	"github.com/charmbracelet/lipgloss"
)

type historyPoint struct {
	At    time.Time
	Value float64
}

type chartView struct {
	XSpan      time.Duration // 0 means "all"
	YZoomSteps int           // 0 means auto-fit to visible window
	YPanSteps  int           // vertical pan; positive moves view up
	Focused    bool
}

func renderHeapLiveHistory(points []historyPoint, frame frameMode, view chartView, w, h int) string {
	if len(points) == 0 {
		return framedSizedBy(frame, heapTitle(view), "(no data)", w, h)
	}

	inner := InnerRect(frameStyle(frame), Rect{W: w, H: h})
	chartW, chartH := clampChartSize(inner.W, inner.H-1)
	c := tslc.New(
		chartW,
		chartH,
		tslc.WithXLabelFormatter(tslc.HourTimeLabelFormatter()),
		tslc.WithXYSteps(0, 2),
		tslc.WithAxesStyles(lipgloss.NewStyle().Foreground(borderColor), lipgloss.NewStyle().Foreground(lipgloss.Color("#c0c0c0"))),
	)

	for _, p := range points {
		c.Push(tslc.TimePoint{Time: p.At, Value: p.Value})
	}

	applyChartView(&c, points, view, 1.0, 0)
	c.DrawBraille()

	return framedSizedBy(frame, heapTitle(view), c.View(), w, h)
}

func renderSTWPercentilesHistory(p50 []historyPoint, p99 []historyPoint, max []historyPoint, frame frameMode, view chartView, w, h int) string {
	if len(p50) == 0 && len(p99) == 0 && len(max) == 0 {
		return framedSizedBy(frame, stwTitle(view), "(no data)", w, h)
	}

	inner := InnerRect(frameStyle(frame), Rect{W: w, H: h})
	chartW, chartH := clampChartSize(inner.W, inner.H-2)
	c := tslc.New(
		chartW,
		chartH,
		tslc.WithXLabelFormatter(tslc.HourTimeLabelFormatter()),
		tslc.WithXYSteps(0, 2),
		tslc.WithAxesStyles(lipgloss.NewStyle().Foreground(borderColor), lipgloss.NewStyle().Foreground(lipgloss.Color("#c0c0c0"))),
		tslc.WithDataSetStyle("p50", lipgloss.NewStyle().Foreground(lipgloss.Color("#2ec4b6"))),
		tslc.WithDataSetStyle("p99", lipgloss.NewStyle().Foreground(lipgloss.Color("#d64f4f"))),
		tslc.WithDataSetStyle("max", lipgloss.NewStyle().Foreground(lipgloss.Color("#c9a227"))),
	)

	for _, p := range p50 {
		c.PushDataSet("p50", tslc.TimePoint{Time: p.At, Value: p.Value})
	}
	for _, p := range p99 {
		c.PushDataSet("p99", tslc.TimePoint{Time: p.At, Value: p.Value})
	}
	for _, p := range max {
		c.PushDataSet("max", tslc.TimePoint{Time: p.At, Value: p.Value})
	}

	all := make([]historyPoint, 0, len(p50)+len(p99)+len(max))
	all = append(all, p50...)
	all = append(all, p99...)
	all = append(all, max...)
	applyChartView(&c, all, view, 10.0, 0)
	c.DrawBrailleAll()

	body := c.View() + "\n" + fmt.Sprintf("legend: %s %s %s",
		lipgloss.NewStyle().Foreground(lipgloss.Color("#2ec4b6")).Render("p50"),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#d64f4f")).Render("p99"),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#c9a227")).Render("max"),
	)

	return framedSizedBy(frame, stwTitle(view), body, w, h)
}

func clampChartSize(w, h int) (int, int) {
	if w <= 0 {
		w = 40
	}
	if h <= 0 {
		h = 10
	}
	if w < 20 {
		w = 20
	}
	if h < 6 {
		h = 6
	}
	return w, h
}

func applyChartView(c *tslc.Model, points []historyPoint, view chartView, minRange float64, clampMin float64) {
	if len(points) == 0 {
		return
	}

	last := points[len(points)-1]
	vis := points
	if view.XSpan > 0 {
		start := last.At.Add(-view.XSpan)
		c.SetViewTimeRange(start, last.At)
		vis = filterByTime(points, start, last.At)
		if len(vis) == 0 {
			vis = points
		}
	}

	minY, maxY := minMaxY(vis)
	if !isFinite(minY) || !isFinite(maxY) {
		return
	}
	if maxY < minY {
		minY, maxY = maxY, minY
	}
	if clampMin > minY {
		minY = clampMin
	}

	rng := maxY - minY
	if rng < minRange {
		rng = minRange
	}

	// Default: fit to visible data range (no zoom, no pan).
	if view.YZoomSteps <= 0 && view.YPanSteps == 0 {
		pad := rng * 0.05
		if pad < minRange*0.05 {
			pad = minRange * 0.05
		}
		minV := minY - pad
		maxV := maxY + pad
		if clampMin > minV {
			minV = clampMin
		}
		c.SetViewYRange(minV, maxV)
		return
	}

	// Zoomed/panned view: start from visible range and apply zoom around center.
	center := (minY + maxY) / 2
	if len(vis) > 0 {
		center = vis[len(vis)-1].Value
	}
	if view.YZoomSteps < 0 {
		view.YZoomSteps = 0
	}
	if view.YZoomSteps > 0 {
		scale := math.Pow(0.75, float64(view.YZoomSteps))
		if scale < 0.05 {
			scale = 0.05
		}
		rng = rng * scale
		if rng < minRange {
			rng = minRange
		}
	}

	minV := center - rng/2
	maxV := center + rng/2
	if minV == maxV {
		maxV = minV + minRange
	}

	if view.YPanSteps != 0 {
		step := rng * 0.2
		if step < minRange*0.2 {
			step = minRange * 0.2
		}
		shift := float64(view.YPanSteps) * step
		minV += shift
		maxV += shift
	}

	// Clamp view to reasonable bounds so the user can't pan into empty space.
	// We clamp against the visible data range with a small padding.
	pad := (maxY - minY) * 0.05
	if pad < minRange*0.05 {
		pad = minRange * 0.05
	}
	boundMin := minY - pad
	boundMax := maxY + pad
	if clampMin > boundMin {
		boundMin = clampMin
	}
	if boundMax-boundMin < minRange {
		boundMax = boundMin + minRange
	}
	if minV < boundMin {
		d := boundMin - minV
		minV += d
		maxV += d
	}
	if maxV > boundMax {
		d := maxV - boundMax
		minV -= d
		maxV -= d
	}
	if minV < boundMin {
		minV = boundMin
		maxV = boundMin + rng
	}
	if maxV > boundMax {
		maxV = boundMax
		minV = boundMax - rng
	}
	if maxV-minV < minRange {
		maxV = minV + minRange
		if maxV > boundMax {
			maxV = boundMax
			minV = maxV - minRange
		}
	}

	// Ensure expected range contains requested view range. Otherwise SetViewYRange can fail
	// (it clamps to expected values), and because charts are re-created every frame, this
	// would look like a "zoom reset".
	expMin := c.MinY()
	expMax := c.MaxY()
	newExpMin := expMin
	newExpMax := expMax
	if minV < newExpMin {
		newExpMin = minV
	}
	if maxV > newExpMax {
		newExpMax = maxV
	}
	if newExpMin != expMin || newExpMax != expMax {
		c.SetYRange(newExpMin, newExpMax)
	}

	c.SetViewYRange(minV, maxV)
}

func filterByTime(points []historyPoint, minT, maxT time.Time) []historyPoint {
	out := make([]historyPoint, 0, len(points))
	for _, p := range points {
		if p.At.Before(minT) || p.At.After(maxT) {
			continue
		}
		out = append(out, p)
	}
	return out
}

func minMaxY(points []historyPoint) (minY, maxY float64) {
	minY = points[0].Value
	maxY = points[0].Value
	for i := 1; i < len(points); i++ {
		v := points[i].Value
		if v < minY {
			minY = v
		}
		if v > maxY {
			maxY = v
		}
	}
	return minY, maxY
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func heapTitle(view chartView) string {
	title := "Heap live over time (MB) x:" + xSpanLabel(view.XSpan)
	if view.Focused {
		title += " (focused)"
	}
	return title
}

func stwTitle(view chartView) string {
	title := "STW p50/p99/max over time (us) x:" + xSpanLabel(view.XSpan)
	if view.Focused {
		title += " (focused)"
	}
	return title
}

func xSpanLabel(d time.Duration) string {
	switch d {
	case 0:
		return "all"
	case time.Minute:
		return "1m"
	case 5 * time.Minute:
		return "5m"
	case 15 * time.Minute:
		return "15m"
	case time.Hour:
		return "1h"
	default:
		// Fallback: keep it short and stable.
		if d < time.Minute {
			return fmt.Sprintf("%ds", int(d.Seconds()))
		}
		if d < time.Hour {
			return fmt.Sprintf("%dm", int(d.Minutes()))
		}
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
}
