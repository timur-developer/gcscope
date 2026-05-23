package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/timur-developer/gcviz/internal/domain"
)

const SchemaVersion = 1

type SnapshotV1 struct {
	Version int              `json:"version"`
	Current CurrentValuesV1  `json:"current"`
	Window  WindowStatsV1    `json:"window"`
	Events  []GCEventV1      `json:"events"`
}

type CurrentValuesV1 struct {
	GCCyclesTotal int   `json:"gc_cycles_total"`
	LastSTWUs     int64 `json:"last_stw_us"`
	HeapLiveMB    int   `json:"heap_live_mb"`
	HeapGoalMB    int   `json:"heap_goal_mb"`
}

type WindowStatsV1 struct {
	STWP50Us int64 `json:"stw_p50_us"`
	STWP99Us int64 `json:"stw_p99_us"`
	STWMaxUs int64 `json:"stw_max_us"`
}

type GCEventV1 struct {
	GCNum            int     `json:"gc_num"`
	TimeSinceStartS  float64 `json:"time_since_start_s"`
	GCCPUPercent     float64 `json:"gc_cpu_percent"`
	STWSweepTermMs   float64 `json:"stw_sweep_term_ms"`
	MarkMs           float64 `json:"mark_ms"`
	STWMarkTermMs    float64 `json:"stw_mark_term_ms"`
	HeapStartMB      int     `json:"heap_start_mb"`
	HeapEndMB        int     `json:"heap_end_mb"`
	HeapLiveMB       int     `json:"heap_live_mb"`
	HeapGoalMB       int     `json:"heap_goal_mb"`
	NumP             int     `json:"num_p"`
	Forced           bool    `json:"forced"`
	SweepHeapSizeMB  int     `json:"sweep_heap_size_mb"`
	PagesSwept       int     `json:"pages_swept"`
	AssistRatio      float64 `json:"assist_ratio"`
	AssistWorkers    int     `json:"assist_workers"`
	CPUPercent       int     `json:"cpu_percent"`
	ConsMark         float64 `json:"cons_mark"`
}

func FromDomain(events []domain.GCEvent, agg domain.Aggregates) SnapshotV1 {
	out := SnapshotV1{
		Version: SchemaVersion,
		Current: CurrentValuesV1{
			GCCyclesTotal: agg.Current.GCCyclesTotal,
			LastSTWUs:     agg.Current.LastSTWUs,
			HeapLiveMB:    agg.Current.HeapLiveMB,
			HeapGoalMB:    agg.Current.HeapGoalMB,
		},
		Window: WindowStatsV1{
			STWP50Us: agg.Window.STWP50Us,
			STWP99Us: agg.Window.STWP99Us,
			STWMaxUs: agg.Window.STWMaxUs,
		},
		Events: make([]GCEventV1, 0, len(events)),
	}

	for _, ev := range events {
		out.Events = append(out.Events, GCEventV1{
			GCNum:            ev.GCNum,
			TimeSinceStartS:  ev.TimeSinceStartS,
			GCCPUPercent:     ev.GCCPUPercent,
			STWSweepTermMs:   ev.STWSweepTermMs,
			MarkMs:           ev.MarkMs,
			STWMarkTermMs:    ev.STWMarkTermMs,
			HeapStartMB:      ev.HeapStartMB,
			HeapEndMB:        ev.HeapEndMB,
			HeapLiveMB:       ev.HeapLiveMB,
			HeapGoalMB:       ev.HeapGoalMB,
			NumP:             ev.NumP,
			Forced:           ev.Forced,
			SweepHeapSizeMB:  ev.SweepHeapSizeMB,
			PagesSwept:       ev.PagesSwept,
			AssistRatio:      ev.AssistRatio,
			AssistWorkers:    ev.AssistWorkers,
			CPUPercent:       ev.CPUPercent,
			ConsMark:         ev.ConsMark,
		})
	}

	return out
}

func Write(dir string, events []domain.GCEvent, agg domain.Aggregates) (string, error) {
	if dir == "" {
		dir = "."
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create snapshot dir: %w", err)
	}

	snap := FromDomain(events, agg)
	b, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal snapshot: %w", err)
	}
	b = append(b, '\n')

	name := snapshotFileName(time.Now())
	path := filepath.Join(dir, name)

	if err := writeFileAtomic(path, b, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func Read(path string) (SnapshotV1, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return SnapshotV1{}, fmt.Errorf("read snapshot: %w", err)
	}

	var snap SnapshotV1
	if err := json.Unmarshal(b, &snap); err != nil {
		return SnapshotV1{}, fmt.Errorf("unmarshal snapshot: %w", err)
	}
	if snap.Version != SchemaVersion {
		return SnapshotV1{}, fmt.Errorf("unsupported snapshot version: %d", snap.Version)
	}

	return snap, nil
}

func snapshotFileName(t time.Time) string {
	return fmt.Sprintf("gcviz-snapshot-%s.json", t.Format("2006-01-02T15-04-05"))
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	f, err := os.CreateTemp(dir, base+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmp := f.Name()

	ok := false
	defer func() {
		_ = f.Close()
		if !ok {
			_ = os.Remove(tmp)
		}
	}()

	if err := f.Chmod(perm); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename snapshot: %w", err)
	}
	ok = true
	return nil
}
