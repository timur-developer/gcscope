package ui

import (
	"fmt"
	"time"

	tslc "github.com/NimbleMarkets/ntcharts/linechart/timeserieslinechart"
	"github.com/charmbracelet/lipgloss"
)

type historyPoint struct {
	At    time.Time
	Value float64
}

func renderHeapLiveHistory(points []historyPoint, frame frameMode, w, h int) string {
	if len(points) == 0 {
		return framedSizedBy(frame, "Heap live over time (MB)", "(no data)", w, h)
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
	c.DrawBraille()

	return framedSizedBy(frame, "Heap live over time (MB)", c.View(), w, h)
}

func renderSTWPercentilesHistory(p50 []historyPoint, p99 []historyPoint, frame frameMode, w, h int) string {
	if len(p50) == 0 && len(p99) == 0 {
		return framedSizedBy(frame, "STW p50/p99 over time (us)", "(no data)", w, h)
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
	)

	for _, p := range p50 {
		c.PushDataSet("p50", tslc.TimePoint{Time: p.At, Value: p.Value})
	}
	for _, p := range p99 {
		c.PushDataSet("p99", tslc.TimePoint{Time: p.At, Value: p.Value})
	}
	c.DrawBrailleAll()

	body := c.View() + "\n" + fmt.Sprintf("legend: %s %s",
		lipgloss.NewStyle().Foreground(lipgloss.Color("#2ec4b6")).Render("p50"),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#d64f4f")).Render("p99"),
	)

	return framedSizedBy(frame, "STW p50/p99 over time (us)", body, w, h)
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
