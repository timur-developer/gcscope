package domain

import (
	"math"
	"testing"
	"time"
)

func TestComputeAggregates_EmptyWindow(t *testing.T) {
	agg := ComputeAggregates(nil)
	if agg.HasData {
		t.Fatalf("expected HasData=false")
	}
	if agg.TargetUptime != 0 {
		t.Fatalf("expected TargetUptime=0, got %v", agg.TargetUptime)
	}
	if agg.Window.STWP50Us != 0 || agg.Window.STWP99Us != 0 || agg.Window.STWMaxUs != 0 {
		t.Fatalf("expected zero window stats, got %+v", agg.Window)
	}
	if agg.Current.GCCyclesTotal != 0 || agg.Current.LastSTWUs != 0 || agg.Current.HeapLiveMB != 0 || agg.Current.HeapGoalMB != 0 {
		t.Fatalf("expected zero current values, got %+v", agg.Current)
	}
}

func TestComputeAggregates_SingleEvent(t *testing.T) {
	window := []GCEvent{
		{
			GCNum:           10,
			TimeSinceStartS: 12.345,
			STWSweepTermMs:  0.25,
			STWMarkTermMs:   0.25,
			HeapLiveMB:      123,
			HeapGoalMB:      456,
		},
	}

	agg := ComputeAggregates(window)
	if !agg.HasData {
		t.Fatalf("expected HasData=true")
	}
	if agg.Current.GCCyclesTotal != 10 {
		t.Fatalf("GCCyclesTotal: got %d, want %d", agg.Current.GCCyclesTotal, 10)
	}
	if agg.Current.LastSTWUs != 500 {
		t.Fatalf("LastSTWUs: got %d, want %d", agg.Current.LastSTWUs, 500)
	}
	if agg.Current.HeapLiveMB != 123 || agg.Current.HeapGoalMB != 456 {
		t.Fatalf("heap values: got live=%d goal=%d", agg.Current.HeapLiveMB, agg.Current.HeapGoalMB)
	}
	if agg.TargetUptime != 12*time.Second+345*time.Millisecond {
		t.Fatalf("TargetUptime: got %v, want %v", agg.TargetUptime, 12*time.Second+345*time.Millisecond)
	}
	if agg.Window.STWP50Us != 500 || agg.Window.STWP99Us != 500 || agg.Window.STWMaxUs != 500 {
		t.Fatalf("window stats: got %+v, want all 500", agg.Window)
	}
}

func TestComputeAggregates_NearestRankPercentiles(t *testing.T) {
	// STW (us) values: [100,200,300,400,500]
	window := []GCEvent{
		{GCNum: 1, TimeSinceStartS: 1, STWSweepTermMs: 0.10, STWMarkTermMs: 0, HeapLiveMB: 1, HeapGoalMB: 10},
		{GCNum: 2, TimeSinceStartS: 2, STWSweepTermMs: 0.20, STWMarkTermMs: 0, HeapLiveMB: 2, HeapGoalMB: 20},
		{GCNum: 3, TimeSinceStartS: 3, STWSweepTermMs: 0.30, STWMarkTermMs: 0, HeapLiveMB: 3, HeapGoalMB: 30},
		{GCNum: 4, TimeSinceStartS: 4, STWSweepTermMs: 0.40, STWMarkTermMs: 0, HeapLiveMB: 4, HeapGoalMB: 40},
		{GCNum: 5, TimeSinceStartS: 5, STWSweepTermMs: 0.50, STWMarkTermMs: 0, HeapLiveMB: 5, HeapGoalMB: 50},
	}

	agg := ComputeAggregates(window)
	// nearest-rank: idx=ceil(p*n)-1
	// p50: idx=ceil(0.5*5)-1=3-1=2 => 300
	if agg.Window.STWP50Us != 300 {
		t.Fatalf("STWP50Us: got %d, want %d", agg.Window.STWP50Us, 300)
	}
	// p99: idx=ceil(0.99*5)-1=5-1=4 => 500
	if agg.Window.STWP99Us != 500 {
		t.Fatalf("STWP99Us: got %d, want %d", agg.Window.STWP99Us, 500)
	}
	if agg.Window.STWMaxUs != 500 {
		t.Fatalf("STWMaxUs: got %d, want %d", agg.Window.STWMaxUs, 500)
	}
}

func TestComputeAggregates_NearestRankPercentiles_EvenWindow(t *testing.T) {
	// STW (us) values: [100,200,300,400]
	window := []GCEvent{
		{GCNum: 1, TimeSinceStartS: 1, STWSweepTermMs: 0.10, STWMarkTermMs: 0, HeapLiveMB: 1, HeapGoalMB: 10},
		{GCNum: 2, TimeSinceStartS: 2, STWSweepTermMs: 0.20, STWMarkTermMs: 0, HeapLiveMB: 2, HeapGoalMB: 20},
		{GCNum: 3, TimeSinceStartS: 3, STWSweepTermMs: 0.30, STWMarkTermMs: 0, HeapLiveMB: 3, HeapGoalMB: 30},
		{GCNum: 4, TimeSinceStartS: 4, STWSweepTermMs: 0.40, STWMarkTermMs: 0, HeapLiveMB: 4, HeapGoalMB: 40},
	}

	agg := ComputeAggregates(window)
	// nearest-rank: idx=ceil(p*n)-1
	// p50: idx=ceil(0.5*4)-1=2-1=1 => 200
	if agg.Window.STWP50Us != 200 {
		t.Fatalf("STWP50Us: got %d, want %d", agg.Window.STWP50Us, 200)
	}
	// p99: idx=ceil(0.99*4)-1=4-1=3 => 400
	if agg.Window.STWP99Us != 400 {
		t.Fatalf("STWP99Us: got %d, want %d", agg.Window.STWP99Us, 400)
	}
	if agg.Window.STWMaxUs != 400 {
		t.Fatalf("STWMaxUs: got %d, want %d", agg.Window.STWMaxUs, 400)
	}
}

func TestComputeAggregates_CurrentValuesFromLastEvent(t *testing.T) {
	// Window order is part of the contract: last element is most recent and is used for Current values.
	window := []GCEvent{
		{GCNum: 100, TimeSinceStartS: 10, STWSweepTermMs: 0.10, HeapLiveMB: 1, HeapGoalMB: 10},
		{GCNum: 101, TimeSinceStartS: 11, STWSweepTermMs: 0.20, HeapLiveMB: 2, HeapGoalMB: 20},
		{GCNum: 999, TimeSinceStartS: 12, STWSweepTermMs: 0.30, HeapLiveMB: 3, HeapGoalMB: 30},
	}

	agg := ComputeAggregates(window)
	if agg.Current.GCCyclesTotal != 999 {
		t.Fatalf("GCCyclesTotal: got %d, want %d", agg.Current.GCCyclesTotal, 999)
	}
	if agg.Current.HeapLiveMB != 3 || agg.Current.HeapGoalMB != 30 {
		t.Fatalf("heap current: got live=%d goal=%d, want live=3 goal=30", agg.Current.HeapLiveMB, agg.Current.HeapGoalMB)
	}
	if agg.Current.LastSTWUs != 300 {
		t.Fatalf("LastSTWUs: got %d, want %d", agg.Current.LastSTWUs, 300)
	}
	if agg.TargetUptime != 12*time.Second {
		t.Fatalf("TargetUptime: got %v, want %v", agg.TargetUptime, 12*time.Second)
	}
}

func TestComputeAggregates_GCRateAndForcedCount(t *testing.T) {
	window := []GCEvent{
		{GCNum: 1, TimeSinceStartS: 0, STWSweepTermMs: 0.10, HeapGoalMB: 10, Forced: true},
		{GCNum: 2, TimeSinceStartS: 10, STWSweepTermMs: 0.10, HeapGoalMB: 10, Forced: false},
		{GCNum: 3, TimeSinceStartS: 20, STWSweepTermMs: 0.10, HeapGoalMB: 10, Forced: true},
		{GCNum: 4, TimeSinceStartS: 30, STWSweepTermMs: 0.10, HeapGoalMB: 10, Forced: false},
	}

	agg := ComputeAggregates(window)

	if agg.Window.ForcedCount != 2 {
		t.Fatalf("ForcedCount: got %d, want %d", agg.Window.ForcedCount, 2)
	}

	// cycles = 3 over 30s => 6 cycles/min
	if math.Abs(agg.Window.GCsPerMin-6.0) > 1e-9 {
		t.Fatalf("GCsPerMin: got %f, want %f", agg.Window.GCsPerMin, 6.0)
	}

	if agg.Window.AvgGCInterval != 10*time.Second {
		t.Fatalf("AvgGCInterval: got %v, want %v", agg.Window.AvgGCInterval, 10*time.Second)
	}
}
