package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

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

			r := runner.NewRunner("./testbin", []string{"--workload", preset}, nil)
			if err := r.Start(ctx); err != nil {
				return err
			}

			model := ui.NewModel(ctx, cancel, cfg.WindowSize)
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
				_, err := prog.Run()
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
