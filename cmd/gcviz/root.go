package main

import (
	"path/filepath"

	"github.com/spf13/cobra"
)

var version = "dev"

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "gcviz",
		Short:        "TUI visualizer for Go GC",
		Long:         "gcviz is a TUI visualizer for Go GC behavior.",
		SilenceUsage: true,
	}

	cmd.SetVersionTemplate("gcviz version {{.Version}}\n")
	cmd.Version = version

	cmd.PersistentFlags().Int("window-size", 200, "Number of recent samples to keep in memory")
	cmd.PersistentFlags().String("snapshot-path", filepath.Join("tmp", "snapshots"), "Path to write snapshot files")
	cmd.PersistentFlags().Int64("stw-warn-us", 200, "STW warning threshold (microseconds)")
	cmd.PersistentFlags().Int64("stw-bad-us", 1000, "STW bad threshold (microseconds)")

	cmd.AddCommand(newRunCmd(), newAttachCmd(), newLabCmd(), newDiffCmd())

	return cmd
}
