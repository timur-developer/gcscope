package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/timur-developer/gcviz/internal/domain"
)

func TestRenderSTWBarChart_HasHeaderGap(t *testing.T) {
	window := []domain.GCEvent{
		{GCNum: 1, STWSweepTermMs: 0.10, STWMarkTermMs: 0.20, HeapLiveMB: 10},
		{GCNum: 2, STWSweepTermMs: 0.15, STWMarkTermMs: 0.25, HeapLiveMB: 12},
		{GCNum: 3, STWSweepTermMs: 0.05, STWMarkTermMs: 0.40, HeapLiveMB: 13},
		{GCNum: 4, STWSweepTermMs: 0.20, STWMarkTermMs: 0.10, HeapLiveMB: 11},
	}

	out := renderSTWBarChart(window, 2, frameBoxed, stwLabelGCAndSTW, STWThresholds{WarnUs: 200, BadUs: 1000}, 0, 20, 120)
	lines := strings.Split(ansi.Strip(out), "\n")

	headerIdx := -1
	for i, ln := range lines {
		if strings.Contains(ln, "legend:") {
			headerIdx = i
			break
		}
	}
	if headerIdx == -1 {
		t.Fatalf("expected header line (legend), got:\n%s", ansi.Strip(out))
	}
	if headerIdx+1 >= len(lines) {
		t.Fatalf("expected a gap line after header, got end of output")
	}
	// Boxed mode still prints borders, so the line isn't strictly empty.
	// We only require it to have no visible content besides borders/padding.
	trimmed := strings.Trim(lines[headerIdx+1], " \t\r\n│")
	if trimmed != "" {
		t.Fatalf("expected an empty gap line after header, got %q", lines[headerIdx+1])
	}
}
