package ui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestBoxedSized_OuterDimensions(t *testing.T) {
	out := boxedSized("Title", "Body", 40, 10)
	if got, want := lipgloss.Width(out), 40; got != want {
		t.Fatalf("width=%d, want %d", got, want)
	}
	if got, want := lipgloss.Height(out), 10; got != want {
		t.Fatalf("height=%d, want %d", got, want)
	}
}
