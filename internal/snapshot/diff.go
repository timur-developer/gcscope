package snapshot

import (
	"fmt"
	"strings"
)

type DiffResult struct {
	A SnapshotV1
	B SnapshotV1
}

func Diff(a, b SnapshotV1) string {
	var sb strings.Builder

	sb.WriteString("A:\n")
	writeSnapshotSummary(&sb, a)

	sb.WriteString("\nB:\n")
	writeSnapshotSummary(&sb, b)

	sb.WriteString("\nDelta (B-A):\n")
	fmt.Fprintf(&sb, "  heap_live_mb: %+d\n", b.Current.HeapLiveMB-a.Current.HeapLiveMB)
	fmt.Fprintf(&sb, "  stw_max_us:   %+d\n", b.Window.STWMaxUs-a.Window.STWMaxUs)
	fmt.Fprintf(&sb, "  stw_p50_us:   %+d\n", b.Window.STWP50Us-a.Window.STWP50Us)
	fmt.Fprintf(&sb, "  stw_p99_us:   %+d\n", b.Window.STWP99Us-a.Window.STWP99Us)

	return sb.String()
}

func writeSnapshotSummary(sb *strings.Builder, s SnapshotV1) {
	fmt.Fprintf(sb, "  gc_cycles_total: %d\n", s.Current.GCCyclesTotal)
	fmt.Fprintf(sb, "  heap_live_mb:    %d\n", s.Current.HeapLiveMB)
	fmt.Fprintf(sb, "  stw_max_us:      %d\n", s.Window.STWMaxUs)
	fmt.Fprintf(sb, "  stw_p50_us:      %d\n", s.Window.STWP50Us)
	fmt.Fprintf(sb, "  stw_p99_us:      %d\n", s.Window.STWP99Us)
}
