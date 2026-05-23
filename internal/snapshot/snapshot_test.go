package snapshot

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/timur-developer/gcviz/internal/domain"
)

func TestSnapshotV1_Golden(t *testing.T) {
	events := []domain.GCEvent{
		{
			GCNum:           1,
			TimeSinceStartS: 1.0,
			STWSweepTermMs:  0.10,
			STWMarkTermMs:   0.20,
			HeapLiveMB:      10,
			HeapGoalMB:      20,
		},
		{
			GCNum:           2,
			TimeSinceStartS: 2.0,
			STWSweepTermMs:  0.05,
			STWMarkTermMs:   0.07,
			HeapLiveMB:      11,
			HeapGoalMB:      21,
		},
		{
			GCNum:           3,
			TimeSinceStartS: 3.0,
			STWSweepTermMs:  0.30,
			STWMarkTermMs:   0.40,
			HeapLiveMB:      12,
			HeapGoalMB:      22,
		},
	}

	agg := domain.ComputeAggregates(events)
	snap := FromDomain(events, agg)

	got, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	got = append(got, '\n')

	goldenPath := filepath.Join("testdata", "snapshot_v1.golden.json")
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	got = bytes.ReplaceAll(got, []byte("\r\n"), []byte("\n"))
	want = bytes.ReplaceAll(want, []byte("\r\n"), []byte("\n"))

	got = bytes.TrimSpace(got)
	want = bytes.TrimSpace(want)

	if !bytes.Equal(got, want) {
		i := firstDiffIndex(got, want)
		t.Fatalf("golden mismatch (diff at byte %d, got_len=%d want_len=%d)\n--- got ---\n%s\n--- want ---\n%s",
			i, len(got), len(want), string(got), string(want))
	}
}

func TestSnapshotV1_ReadGolden(t *testing.T) {
	goldenPath := filepath.Join("testdata", "snapshot_v1.golden.json")
	_, err := Read(goldenPath)
	if err != nil {
		t.Fatalf("Read(golden): %v", err)
	}
}

func firstDiffIndex(a, b []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	if len(a) != len(b) {
		return n
	}
	return -1
}
