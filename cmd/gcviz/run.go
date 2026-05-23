package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/spf13/cobra"

	"github.com/timur-developer/gcviz/internal/source/runner"
)

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run target under GC observation",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := Load(cmd, args)
			if err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
			defer stop()

			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			go watchQuit(cancel)

			runner := runner.NewRunner(cfg.Run.Target, cfg.Run.Args, nil)
			if err := runner.Start(ctx); err != nil {
				return err
			}

			go func() {
				for line := range runner.Stderr() {
					fmt.Fprintln(os.Stderr, line)
				}
			}()
			go func() {
				for range runner.Events() {
				}
			}()
			go func() {
				for err := range runner.ParseErrors() {
					fmt.Fprintln(os.Stderr, err)
				}
			}()

			return runner.Wait()
		},
	}

	cmd.Flags().String("target", "", "Path to target binary")

	return cmd
}

func watchQuit(cancel context.CancelFunc) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		switch text {
		case "q", "Q":
			cancel()
			return
		}
	}
}
