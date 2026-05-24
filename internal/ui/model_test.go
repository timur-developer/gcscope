package ui

import (
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/timur-developer/gcviz/internal/domain"
)

func TestModel_PauseFreezesWindowAndCursor(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m := NewModel(ctx, cancel, 100, "", nil)

	at := time.Unix(0, 0)
	for i := 1; i <= 5; i++ {
		ev := domain.GCEvent{
			GCNum:           i,
			TimeSinceStartS: float64(i),
			HeapLiveMB:      10 + i,
			HeapGoalMB:      100,
		}
		updated, _ := m.Update(GCEventMsg{Event: ev, At: at.Add(time.Duration(i) * time.Second)})
		m = updated.(Model)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = updated.(Model)

	if !m.paused {
		t.Fatalf("expected paused=true")
	}
	if got, want := len(m.pausedWindow), 5; got != want {
		t.Fatalf("pausedWindow len=%d, want %d", got, want)
	}
	if got, want := m.cursor, 4; got != want {
		t.Fatalf("cursor=%d, want %d", got, want)
	}

	// New events should not change paused snapshot.
	for i := 6; i <= 9; i++ {
		ev := domain.GCEvent{
			GCNum:           i,
			TimeSinceStartS: float64(i),
			HeapLiveMB:      10 + i,
			HeapGoalMB:      100,
		}
		updated, _ := m.Update(GCEventMsg{Event: ev, At: at.Add(time.Duration(i) * time.Second)})
		m = updated.(Model)
	}

	if got, want := len(m.pausedWindow), 5; got != want {
		t.Fatalf("pausedWindow len after new events=%d, want %d", got, want)
	}
	if got, want := m.cursor, 4; got != want {
		t.Fatalf("cursor after new events=%d, want %d", got, want)
	}
}

func TestModel_ScrubBounds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m := NewModel(ctx, cancel, 100, "", nil)

	at := time.Unix(0, 0)
	for i := 1; i <= 3; i++ {
		ev := domain.GCEvent{GCNum: i, TimeSinceStartS: float64(i), HeapGoalMB: 100}
		updated, _ := m.Update(GCEventMsg{Event: ev, At: at.Add(time.Duration(i) * time.Second)})
		m = updated.(Model)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = updated.(Model)
	if !m.paused {
		t.Fatalf("expected paused=true")
	}

	// Move left past start clamps to 0.
	for range 10 {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
		m = updated.(Model)
	}
	if got, want := m.cursor, 0; got != want {
		t.Fatalf("cursor=%d, want %d", got, want)
	}

	// Move right past end clamps to last.
	for range 10 {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
		m = updated.(Model)
	}
	if got, want := m.cursor, 2; got != want {
		t.Fatalf("cursor=%d, want %d", got, want)
	}

	// home/end jump.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyHome})
	m = updated.(Model)
	if got, want := m.cursor, 0; got != want {
		t.Fatalf("cursor=%d, want %d", got, want)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	m = updated.(Model)
	if got, want := m.cursor, 2; got != want {
		t.Fatalf("cursor=%d, want %d", got, want)
	}
}

func TestModel_STWLabelsModeCycles(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m := NewModel(ctx, cancel, 10, "", nil)
	if m.stwLabelsMode != stwLabelGCAndSTW {
		t.Fatalf("initial stwLabelsMode=%v, want %v", m.stwLabelsMode, stwLabelGCAndSTW)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = updated.(Model)
	if m.stwLabelsMode != stwLabelGCAndHeap {
		t.Fatalf("stwLabelsMode=%v, want %v", m.stwLabelsMode, stwLabelGCAndHeap)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = updated.(Model)
	if m.stwLabelsMode != stwLabelGCOnly {
		t.Fatalf("stwLabelsMode=%v, want %v", m.stwLabelsMode, stwLabelGCOnly)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = updated.(Model)
	if m.stwLabelsMode != stwLabelGCAndSTW {
		t.Fatalf("stwLabelsMode=%v, want %v", m.stwLabelsMode, stwLabelGCAndSTW)
	}
}
