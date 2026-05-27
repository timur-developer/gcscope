package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/timur-developer/gcviz/internal/domain"
)

func renderCurrentValues(agg domain.Aggregates, frame frameMode, th STWThresholds, w, h int) string {
	var b strings.Builder

	if !agg.HasData {
		fmt.Fprintf(&b, "GC cycles total: -\n")
		fmt.Fprintf(&b, "last STW (us):    -\n")
		fmt.Fprintf(&b, "heap live (MB):  -\n")
		fmt.Fprintf(&b, "heap goal (MB):  -\n")
		return framedSizedBy(frame, "Current Values", b.String(), w, h)
	}

	fmt.Fprintf(&b, "GC cycles total: %d\n", agg.Current.GCCyclesTotal)
	fmt.Fprintf(&b, "last STW (us):    %s\n", stwStyle(th, agg.Current.LastSTWUs).Render(fmt.Sprintf("%d", agg.Current.LastSTWUs)))
	fmt.Fprintf(&b, "heap live (MB):  %d\n", agg.Current.HeapLiveMB)
	fmt.Fprintf(&b, "heap goal (MB):  %d\n", agg.Current.HeapGoalMB)

	ratio := 0.0
	if agg.Current.HeapGoalMB > 0 {
		ratio = float64(agg.Current.HeapLiveMB) / float64(agg.Current.HeapGoalMB)
	}
	fill := lipgloss.NewStyle().Foreground(lipgloss.Color("#2ec4b6"))
	empty := lipgloss.NewStyle().Foreground(lipgloss.Color("#3a3a44"))
	fmt.Fprintf(&b, "heap:           %s %d/%d\n", progressBar(20, ratio, fill, empty), agg.Current.HeapLiveMB, agg.Current.HeapGoalMB)

	return framedSizedBy(frame, "Current Values", b.String(), w, h)
}

func renderInformation(window []domain.GCEvent, agg domain.Aggregates, now time.Time, lastUpdate time.Time, snapshotDir string, snap snapshotStatus, frame frameMode, th STWThresholds, targetEnv *TargetEnvInfo, w, h int) string {
	innerH := 0
	if w > 0 && h > 0 {
		innerH = InnerRect(frameStyle(frame), Rect{W: w, H: h}).H
	}
	bodyH := innerH
	if bodyH > 0 {
		bodyH--
	}

	var lines []string

	if lastUpdate.IsZero() {
		lines = append(lines, "time since last GC: -")
	} else {
		lines = append(lines, fmt.Sprintf("time since last GC: %s", formatSeconds(now.Sub(lastUpdate))))
	}

	if !agg.HasData {
		lines = append(lines, "max STW (us):      -")
		lines = append(lines, "gc:               -")
		lines = append(lines, "stw:              -")
		lines = append(lines, fmt.Sprintf("stw thresholds:    warn=%dus bad=%dus", th.WarnUs, th.BadUs))
		lines = append(lines, formatEnvLines(targetEnv, bodyH, true)...)
		lines = append(lines, "uptime:           -")
		lines = append(lines, snapshotLines(snapshotDir, snap, bodyH)...)
		return framedSizedBy(frame, "Information", strings.Join(fitLines(lines, bodyH), "\n"), w, h)
	}

	lines = append(lines, fmt.Sprintf("max STW (us):      %s", stwStyle(th, agg.Window.STWMaxUs).Render(fmt.Sprintf("%d", agg.Window.STWMaxUs))))

	gcLine := "gc:               -"
	if agg.Window.GCsPerMin > 0 && agg.Window.AvgGCInterval > 0 {
		gcLine = fmt.Sprintf("gc:               %.1f/min (avg %s)", agg.Window.GCsPerMin, formatDuration(agg.Window.AvgGCInterval))
	} else if agg.Window.GCsPerMin > 0 {
		gcLine = fmt.Sprintf("gc:               %.1f/min", agg.Window.GCsPerMin)
	} else if agg.Window.AvgGCInterval > 0 {
		gcLine = fmt.Sprintf("gc:               avg %s", formatDuration(agg.Window.AvgGCInterval))
	}
	lines = append(lines, gcLine)

	_, badCount := countSTWClasses(window, th)
	var stwLine string
	if len(window) > 0 {
		badPct := float64(badCount) / float64(len(window)) * 100.0
		stwLine = fmt.Sprintf("stw:              bad %d/%d (%.1f%%), forced %d", badCount, len(window), badPct, agg.Window.ForcedCount)
	} else {
		stwLine = fmt.Sprintf("stw:              forced %d", agg.Window.ForcedCount)
	}
	lines = append(lines, stwLine)

	lines = append(lines, fmt.Sprintf("uptime:           %s", formatDuration(agg.TargetUptime)))

	// Progress line is nice, but low priority when vertical space is tight.
	if innerH == 0 || innerH >= 10 {
		den := float64(th.BadUs)
		if den <= 0 {
			den = 1000.0
		}
		ratio := float64(agg.Current.LastSTWUs) / den
		fill := stwFillStyle(th, agg.Current.LastSTWUs)
		empty := lipgloss.NewStyle().Foreground(lipgloss.Color("#3a3a44"))
		lines = append(lines, fmt.Sprintf("last STW:        %s %d", progressBar(20, ratio, fill, empty), agg.Current.LastSTWUs))
	}

	lines = append(lines, fmt.Sprintf("stw thresholds:    warn=%dus bad=%dus", th.WarnUs, th.BadUs))
	lines = append(lines, formatEnvLines(targetEnv, bodyH, false)...)
	lines = append(lines, snapshotLines(snapshotDir, snap, bodyH)...)

	return framedSizedBy(frame, "Information", strings.Join(fitLines(lines, bodyH), "\n"), w, h)
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
		"l       toggle STW labels",
		"g       toggle layout",
		"z       focus chart (heap/stw)",
		"+/-     zoom Y (focused chart)",
		"0       reset Y zoom (focused chart)",
		"shift+up/down pan Y (focused chart)",
		"[ / ]   zoom X (time window)",
		"r       reset all zoom",
		"",
		"flags:  --stw-warn-us, --stw-bad-us (or env: GCVIZ_STW_WARN_US, GCVIZ_STW_BAD_US)",
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

func valueOrDash(v string) string {
	if v == "" {
		return "-"
	}
	return v
}

func countSTWClasses(window []domain.GCEvent, th STWThresholds) (warn int, bad int) {
	for _, ev := range window {
		us := stwPerCycleUs(ev)
		if us >= th.BadUs {
			bad++
			continue
		}
		if us >= th.WarnUs {
			warn++
		}
	}
	return warn, bad
}

func fitLines(lines []string, innerH int) []string {
	if innerH <= 0 {
		return lines
	}
	if len(lines) <= innerH {
		return lines
	}
	return lines[:innerH]
}

func snapshotLines(snapshotDir string, snap snapshotStatus, innerH int) []string {
	if snapshotDir == "" {
		if snap.FileName != "" {
			return []string{fmt.Sprintf("snapshot:         %s", snap.FileName)}
		}
		if snap.ErrMsg != "" {
			return []string{fmt.Sprintf("snapshot error:   %s", snap.ErrMsg)}
		}
		return []string{"snapshot:         -"}
	}

	// When space is tight, prefer showing snapshot state over directory.
	if innerH > 0 && innerH < 10 {
		if snap.FileName != "" {
			return []string{fmt.Sprintf("snapshot:         %s", snap.FileName)}
		}
		if snap.ErrMsg != "" {
			return []string{fmt.Sprintf("snapshot error:   %s", snap.ErrMsg)}
		}
		return []string{"snapshot:         -"}
	}

	lines := []string{fmt.Sprintf("snapshot dir:     %s", snapshotDir)}
	if snap.FileName != "" {
		lines = append(lines, fmt.Sprintf("snapshot:         %s", snap.FileName))
		return lines
	}
	if snap.ErrMsg != "" {
		lines = append(lines, fmt.Sprintf("snapshot error:   %s", snap.ErrMsg))
		return lines
	}
	lines = append(lines, "snapshot:         -")
	return lines
}

func formatEnvLines(targetEnv *TargetEnvInfo, innerH int, noData bool) []string {
	// Minimal mode on small heights to keep room for key stats + snapshot lines.
	compact := innerH > 0 && innerH < 12

	if targetEnv == nil {
		if compact {
			return []string{"env context:      n/a (attach mode)"}
		}
		return []string{
			"GOGC:             n/a",
			"GOMEMLIMIT:       n/a",
			"GODEBUG:          n/a",
		}
	}

	if compact && !noData {
		// Keep it short: show only GODEBUG as the most relevant "what are we parsing".
		return []string{fmt.Sprintf("GODEBUG:          %s", valueOrDash(targetEnv.GODEBUG))}
	}

	return []string{
		fmt.Sprintf("GOGC:             %s", valueOrDash(targetEnv.GOGC)),
		fmt.Sprintf("GOMEMLIMIT:       %s", valueOrDash(targetEnv.GOMEMLIMIT)),
		fmt.Sprintf("GODEBUG:          %s", valueOrDash(targetEnv.GODEBUG)),
	}
}
