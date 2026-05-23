package main

import "github.com/spf13/cobra"

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
	cmd.PersistentFlags().String("snapshot-path", "", "Path to write snapshot files")

	cmd.AddCommand(newRunCmd(), newAttachCmd(), newLabCmd(), newDiffCmd())

	return cmd
}
