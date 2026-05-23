package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/timur-developer/gcviz/internal/domain"
	"github.com/timur-developer/gcviz/internal/source/collector"
	"github.com/timur-developer/gcviz/internal/snapshot"
	"github.com/timur-developer/gcviz/internal/ui"
)

func newAttachCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attach <url>",
		Short: "Attach to a running service",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := Load(cmd, args)
			if err != nil {
				return err
			}
			if cfg.Attach.URL == "" {
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "missing url")
				return ExitError{Code: 2, Err: errors.New("missing url")}
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			c := collector.NewCollector(cfg.Attach.URL, cfg.Attach.PollInterval, nil)
			if err := c.Start(ctx); err != nil {
				return err
			}

			snapshotDir := cfg.SnapshotPath
			if snapshotDir == "" {
				snapshotDir = "."
			}
			writer := attachSnapshotWriter{dir: snapshotDir}

			model := ui.NewModel(ctx, cancel, cfg.WindowSize, snapshotDir, writer)
			prog := tea.NewProgram(model, tea.WithAltScreen())

			go func() {
				for ev := range c.Events() {
					prog.Send(ui.GCEventMsg{Event: ev, At: time.Now()})
				}
			}()
			go func() {
				for range c.Errors() {
				}
			}()

			progErrCh := make(chan error, 1)
			go func() {
				finalModel, err := prog.Run()
				if err == nil {
					if m, ok := finalModel.(ui.Model); ok {
						snapErr := writeSnapshotOnExitAttach(snapshotDir, m)
						if snapErr != nil {
							err = ExitError{Code: 1, Err: snapErr}
						}
					}
				}
				progErrCh <- err
			}()

			waitErr := c.Wait()
			cancel()
			uiErr := <-progErrCh

			if uiErr != nil && !errors.Is(uiErr, tea.ErrProgramKilled) {
				return uiErr
			}
			return waitErr
		},
	}

	cmd.Flags().String("url", "", "Runtime metrics endpoint URL")
	cmd.Flags().Duration("poll-interval", time.Second, "Polling interval for runtime metrics")

	return cmd
}

type attachSnapshotWriter struct {
	dir string
}

func (w attachSnapshotWriter) WriteSnapshot(events []domain.GCEvent, agg domain.Aggregates) (string, error) {
	path, err := snapshot.Write(w.dir, events, agg)
	if err != nil {
		return "", err
	}
	return filepath.Base(path), nil
}

func writeSnapshotOnExitAttach(dir string, m ui.Model) error {
	events, agg := m.SnapshotState()
	if len(events) == 0 {
		return nil
	}
	_, err := snapshot.Write(dir, events, agg)
	return err
}
