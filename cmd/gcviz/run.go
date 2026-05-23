package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/timur-developer/gcviz/internal/domain"
	"github.com/timur-developer/gcviz/internal/source/runner"
	"github.com/timur-developer/gcviz/internal/snapshot"
	"github.com/timur-developer/gcviz/internal/ui"
)

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run <target> [args...]",
		Short: "Run target under GC observation",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := Load(cmd, args)
			if err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			r := runner.NewRunner(cfg.Run.Target, cfg.Run.Args, nil)
			if err := r.Start(ctx); err != nil {
				return err
			}

			snapshotDir := cfg.SnapshotPath
			if snapshotDir == "" {
				snapshotDir = "."
			}
			writer := snapshotWriter{dir: snapshotDir}

			model := ui.NewModel(ctx, cancel, cfg.WindowSize, snapshotDir, writer)
			prog := tea.NewProgram(model, tea.WithAltScreen())

			go func() {
				for ev := range r.Events() {
					prog.Send(ui.GCEventMsg{Event: ev, At: time.Now()})
				}
			}()
			go func() {
				for range r.Stderr() {
				}
			}()
			go func() {
				for range r.ParseErrors() {
				}
			}()

			progErrCh := make(chan error, 1)
			go func() {
				finalModel, err := prog.Run()
				if err == nil {
					if m, ok := finalModel.(ui.Model); ok {
						snapErr := writeSnapshotOnExit(snapshotDir, m)
						if snapErr != nil {
							err = ExitError{Code: 1, Err: snapErr}
						}
					}
				}
				progErrCh <- err
			}()

			waitErr := r.Wait()
			cancel()
			uiErr := <-progErrCh

			if uiErr != nil && !errors.Is(uiErr, tea.ErrProgramKilled) {
				return uiErr
			}
			return waitErr
		},
	}

	cmd.Flags().String("target", "", "Path to target binary")

	return cmd
}

type snapshotWriter struct {
	dir string
}

func (w snapshotWriter) WriteSnapshot(events []domain.GCEvent, agg domain.Aggregates) (string, error) {
	path, err := snapshot.Write(w.dir, events, agg)
	if err != nil {
		return "", err
	}
	return filepath.Base(path), nil
}

func writeSnapshotOnExit(dir string, m ui.Model) error {
	events, agg := m.SnapshotState()
	if len(events) == 0 {
		return nil
	}
	_, err := snapshot.Write(dir, events, agg)
	return err
}
