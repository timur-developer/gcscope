package runner

import (
	"math"
	"testing"
)

func TestParser_ParseGCLineBasic(t *testing.T) {
	parser := NewParser()
	line := "gc 1 @0.041s 1%: 0.53+0.55+0 ms clock, 8.6+0/0/0+0 ms cpu, 3->4->1 MB, 4 MB goal, 0 MB stacks, 0 MB globals, 16 P"

	event, err := parser.ParseLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event != nil {
		t.Fatalf("expected no event yet")
	}

	event = parser.Flush()
	if event == nil {
		t.Fatalf("expected event after flush")
	}
	if event.GCNum != 1 {
		t.Fatalf("expected GCNum 1, got %d", event.GCNum)
	}
	if !floatEqual(event.TimeSinceStartS, 0.041) {
		t.Fatalf("expected TimeSinceStartS 0.041, got %f", event.TimeSinceStartS)
	}
	if !floatEqual(event.GCCPUPercent, 1) {
		t.Fatalf("expected GCCPUPercent 1, got %f", event.GCCPUPercent)
	}
	if !floatEqual(event.STWSweepTermMs, 0.53) || !floatEqual(event.MarkMs, 0.55) || !floatEqual(event.STWMarkTermMs, 0) {
		t.Fatalf("unexpected clock values: %+v", event)
	}
	if event.HeapStartMB != 3 || event.HeapEndMB != 4 || event.HeapLiveMB != 1 {
		t.Fatalf("unexpected heap values: %+v", event)
	}
	if event.HeapGoalMB != 4 || event.NumP != 16 {
		t.Fatalf("unexpected goal or P: %+v", event)
	}
	if event.Forced {
		t.Fatalf("expected Forced=false")
	}
}

func TestParser_ParseGCLineForced(t *testing.T) {
	parser := NewParser()
	line := "gc 2 @0.001s 39%: 0.52+0+0 ms clock, 8.4+0/0/0+0 ms cpu, 0->0->0 MB, 4 MB goal, 0 MB stacks, 0 MB globals, 16 P (forced)"

	_, err := parser.ParseLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	event := parser.Flush()
	if event == nil {
		t.Fatalf("expected event after flush")
	}
	if !event.Forced {
		t.Fatalf("expected Forced=true")
	}
}

func TestParser_ParsePacerLines(t *testing.T) {
	parser := NewParser()
	gcLine := "gc 10 @0.100s 2%: 0.10+0.20+0.30 ms clock, 1.0+0/0/0+0 ms cpu, 1->2->1 MB, 8 MB goal, 0 MB stacks, 0 MB globals, 4 P"
	sweepLine := "pacer: sweep done at heap size 2MB; allocated 1MB during sweep; swept 588 pages at +0.000000e+000 pages/byte"
	assistLine := "pacer: assist ratio=+1.884413e+000 (scan 0 MB in 3->4 MB) workers=4++0.000000e+000"
	cpuLine := "pacer: 25% CPU (25 exp.) for 582304+70560+472098 B work (1053010 B exp.) in 3635504 B -> 4639136 B (Δgoal 444832, cons/mark +1.996765e-001)"

	_, err := parser.ParseLine(gcLine)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = parser.ParseLine(sweepLine)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = parser.ParseLine(assistLine)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = parser.ParseLine(cpuLine)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	event := parser.Flush()
	if event == nil {
		t.Fatalf("expected event after flush")
	}
	if event.SweepHeapSizeMB != 2 || event.PagesSwept != 588 {
		t.Fatalf("unexpected sweep values: %+v", event)
	}
	if !floatEqual(event.AssistRatio, 1.884413) {
		t.Fatalf("unexpected assist ratio: %f", event.AssistRatio)
	}
	if event.AssistWorkers != 4 {
		t.Fatalf("unexpected assist workers: %d", event.AssistWorkers)
	}
	if event.CPUPercent != 25 {
		t.Fatalf("unexpected cpu percent: %d", event.CPUPercent)
	}
	if !floatEqual(event.ConsMark, 0.1996765) {
		t.Fatalf("unexpected cons/mark: %f", event.ConsMark)
	}
}

func TestParser_GroupingOnNextGC(t *testing.T) {
	parser := NewParser()
	gc1 := "gc 41 @1.000s 1%: 0.10+0.20+0.30 ms clock, 1.0+0/0/0+0 ms cpu, 1->2->1 MB, 4 MB goal, 0 MB stacks, 0 MB globals, 8 P"
	pacerSweepForNext := "pacer: sweep done at heap size 7MB; allocated 1MB during sweep; swept 10 pages at +0.000000e+000 pages/byte"
	gc2 := "gc 42 @1.100s 1%: 0.11+0.21+0.31 ms clock, 1.1+0/0/0+0 ms cpu, 2->3->2 MB, 5 MB goal, 0 MB stacks, 0 MB globals, 8 P"

	_, err := parser.ParseLine(gc1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = parser.ParseLine(pacerSweepForNext)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	event, err := parser.ParseLine(gc2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event == nil {
		t.Fatalf("expected previous event")
	}
	if event.GCNum != 41 {
		t.Fatalf("expected GCNum 41, got %d", event.GCNum)
	}
	if event.SweepHeapSizeMB != 7 || event.PagesSwept != 10 {
		t.Fatalf("unexpected sweep values: %+v", event)
	}

	event = parser.Flush()
	if event == nil {
		t.Fatalf("expected second event")
	}
	if event.GCNum != 42 {
		t.Fatalf("expected GCNum 42, got %d", event.GCNum)
	}
}

func TestParser_SkipNonGCLine(t *testing.T) {
	parser := NewParser()

	event, err := parser.ParseLine("command-line-arguments")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event != nil {
		t.Fatalf("expected no event")
	}
}

func TestParser_ParseErrors(t *testing.T) {
	parser := NewParser()

	_, err := parser.ParseLine("gc bad line")
	if err == nil {
		t.Fatalf("expected error for gc line")
	}

	_, err = parser.ParseLine("pacer: sweep done at heap size XMB")
	if err == nil {
		t.Fatalf("expected error for pacer line without gc")
	}

	gcLine := "gc 1 @0.001s 1%: 0.1+0.2+0.3 ms clock, 1.0+0/0/0+0 ms cpu, 1->1->1 MB, 2 MB goal, 0 MB stacks, 0 MB globals, 1 P"
	_, err = parser.ParseLine(gcLine)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = parser.ParseLine("pacer: unknown format")
	if err == nil {
		t.Fatalf("expected error for unknown pacer format")
	}
}

func floatEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-6
}

