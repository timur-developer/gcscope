package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/timur-developer/gcviz/internal/domain"
)

func renderCurrentValues(agg domain.Aggregates, w, h int) string {
	var b strings.Builder

	if !agg.HasData {
		fmt.Fprintf(&b, "GC cycles total: -\n")
		fmt.Fprintf(&b, "last STW (us):    -\n")
		fmt.Fprintf(&b, "heap live (MB):  -\n")
		fmt.Fprintf(&b, "heap goal (MB):  -\n")
		return boxedSized("Current Values", b.String(), w, h)
	}

	fmt.Fprintf(&b, "GC cycles total: %d\n", agg.Current.GCCyclesTotal)
	fmt.Fprintf(&b, "last STW (us):    %s\n", stwStyle(agg.Current.LastSTWUs).Render(fmt.Sprintf("%d", agg.Current.LastSTWUs)))
	fmt.Fprintf(&b, "heap live (MB):  %d\n", agg.Current.HeapLiveMB)
	fmt.Fprintf(&b, "heap goal (MB):  %d\n", agg.Current.HeapGoalMB)

	ratio := 0.0
	if agg.Current.HeapGoalMB > 0 {
		ratio = float64(agg.Current.HeapLiveMB) / float64(agg.Current.HeapGoalMB)
	}
	fill := lipgloss.NewStyle().Background(lipgloss.Color("#2ec4b6"))
	empty := lipgloss.NewStyle().Background(lipgloss.Color("#2b2b2b"))
	fmt.Fprintf(&b, "heap:           %s %d/%d\n", progressBar(20, ratio, fill, empty), agg.Current.HeapLiveMB, agg.Current.HeapGoalMB)

	return boxedSized("Current Values", b.String(), w, h)
}

func renderInformation(agg domain.Aggregates, now time.Time, lastUpdate time.Time, snapshotDir string, snap snapshotStatus, w, h int) string {
	var b strings.Builder

	if lastUpdate.IsZero() {
		fmt.Fprintf(&b, "time since last GC: -\n")
	} else {
		fmt.Fprintf(&b, "time since last GC: %s\n", formatSeconds(now.Sub(lastUpdate)))
	}

	if !agg.HasData {
		fmt.Fprintf(&b, "max STW (us):      -\n")
		fmt.Fprintf(&b, "uptime:           -\n")
		writeSnapshotInfo(&b, snapshotDir, snap)
		return boxedSized("Information", b.String(), w, h)
	}

	fmt.Fprintf(&b, "max STW (us):      %s\n", stwStyle(agg.Window.STWMaxUs).Render(fmt.Sprintf("%d", agg.Window.STWMaxUs)))
	fmt.Fprintf(&b, "uptime:           %s\n", formatDuration(agg.TargetUptime))

	ratio := float64(agg.Current.LastSTWUs) / 1000.0
	fill := stwFillStyle(agg.Current.LastSTWUs)
	empty := lipgloss.NewStyle().Background(lipgloss.Color("#2b2b2b"))
	fmt.Fprintf(&b, "last STW:        %s %d\n", progressBar(20, ratio, fill, empty), agg.Current.LastSTWUs)

	writeSnapshotInfo(&b, snapshotDir, snap)
	return boxedSized("Information", b.String(), w, h)
}

func renderHelp(width, height int) string {
	body := strings.Join([]string{
		"q       quit",
		"ctrl+c  quit",
		"s       snapshot",
		"space   pause/resume",
		"left    scrub (paused)",
		"right   scrub (paused)",
		"home    jump to first (paused)",
		"end     jump to last (paused)",
		"?       toggle help (Shift+/)",
		"h       toggle help",
		"f1      toggle help",
	}, "\n")

	box := boxed("Help", body)

	if width > 0 && height > 0 {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
	}
	return box
}

func writeSnapshotInfo(b *strings.Builder, snapshotDir string, snap snapshotStatus) {
	if snapshotDir != "" {
		fmt.Fprintf(b, "snapshot dir:     %s\n", snapshotDir)
	}

	if snap.FileName != "" {
		fmt.Fprintf(b, "snapshot:         %s\n", snap.FileName)
		return
	}
	if snap.ErrMsg != "" {
		fmt.Fprintf(b, "snapshot error:   %s\n", snap.ErrMsg)
		return
	}
	fmt.Fprintf(b, "snapshot:         -\n")
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		return "0s"
	}

	// Keep it compact for MVP.
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		min := int(d / time.Minute)
		sec := int((d % time.Minute) / time.Second)
		return fmt.Sprintf("%dm%02ds", min, sec)
	}

	h := int(d / time.Hour)
	min := int((d % time.Hour) / time.Minute)
	sec := int((d % time.Minute) / time.Second)
	return fmt.Sprintf("%dh%02dm%02ds", h, min, sec)
}

func formatSeconds(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}
