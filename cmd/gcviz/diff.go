package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/timur-developer/gcviz/internal/snapshot"
)

func newDiffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff <a.json> <b.json>",
		Short: "Compare two snapshot files",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := Load(cmd, args)
			if err != nil {
				return err
			}

			if cfg.Diff.A == "" || cfg.Diff.B == "" {
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "missing snapshot paths: expected `gcviz diff <a.json> <b.json>` or flags --a/--b")
				return ExitError{Code: 2, Err: errors.New("missing snapshot paths")}
			}

			a, err := snapshot.Read(cfg.Diff.A)
			if err != nil {
				return err
			}
			b, err := snapshot.Read(cfg.Diff.B)
			if err != nil {
				return err
			}

			_, err = fmt.Fprint(cmd.OutOrStdout(), snapshot.Diff(a, b))
			return err
		},
	}

	cmd.Flags().String("a", "", "Path to snapshot A")
	cmd.Flags().String("b", "", "Path to snapshot B")

	return cmd
}
