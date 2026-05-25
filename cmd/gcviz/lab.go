package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/timur-developer/gcviz/internal/domain"
	"github.com/timur-developer/gcviz/internal/snapshot"
	lab "github.com/timur-developer/gcviz/internal/source/lab"
	"github.com/timur-developer/gcviz/internal/source/runner"
	"github.com/timur-developer/gcviz/internal/ui"
)

func newLabCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lab <preset>",
		Short: "Run a built-in demo workload",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "too many arguments\navailable presets: %s\n", lab.AvailablePresetsString())
				return ExitError{Code: 2, Err: errors.New("too many arguments")}
			}

			cfg, err := Load(cmd, args)
			if err != nil {
				return err
			}

			preset := cfg.Lab.Preset
			if preset == "" && len(args) > 0 {
				preset = args[0]
			}
			if preset == "" {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "missing preset\navailable presets: %s\n", lab.AvailablePresetsString())
				return ExitError{Code: 2, Err: errors.New("missing preset")}
			}
			if !lab.IsValidPreset(preset) {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "unknown preset: %s\navailable presets: %s\n", preset, lab.AvailablePresetsString())
				return ExitError{Code: 2, Err: fmt.Errorf("unknown preset: %s", preset)}
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			testbinPath, cleanup, err := buildLabTestbin()
			if err != nil {
				return err
			}
			defer cleanup()

			r := runner.NewRunner(testbinPath, []string{"--workload", preset}, nil)
			if err := r.Start(ctx); err != nil {
				return err
			}

			snapshotDir := cfg.SnapshotPath
			writer := labSnapshotWriter{dir: snapshotDir}

			stwTh := ui.STWThresholds{WarnUs: cfg.STWWarnUs, BadUs: cfg.STWBadUs}
			envInfo := &ui.TargetEnvInfo{
				GOGC:       os.Getenv("GOGC"),
				GOMEMLIMIT: os.Getenv("GOMEMLIMIT"),
				GODEBUG:    runner.NormalizeGODEBUG(os.Getenv("GODEBUG")),
			}
			model := ui.NewModel(ctx, cancel, cfg.WindowSize, snapshotDir, writer, stwTh, envInfo)
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
						snapErr := writeSnapshotOnExitLab(snapshotDir, m)
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

	return cmd
}

func buildLabTestbin() (path string, cleanup func(), err error) {
	dir, err := os.MkdirTemp("", "gcviz-testbin-*")
	if err != nil {
		return "", nil, err
	}

	binName := "testbin"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}

	outPath := filepath.Join(dir, binName)
	buildCmd := exec.Command("go", "build", "-o", outPath, "./cmd/testbin")
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		_ = os.RemoveAll(dir)
		return "", nil, err
	}

	return outPath, func() { _ = os.RemoveAll(dir) }, nil
}

type labSnapshotWriter struct {
	dir string
}

func (w labSnapshotWriter) WriteSnapshot(events []domain.GCEvent, agg domain.Aggregates) (string, error) {
	path, err := snapshot.Write(w.dir, events, agg)
	if err != nil {
		return "", err
	}
	return filepath.Base(path), nil
}

func writeSnapshotOnExitLab(dir string, m ui.Model) error {
	events, agg := m.SnapshotState()
	if len(events) == 0 {
		return nil
	}
	_, err := snapshot.Write(dir, events, agg)
	return err
}
