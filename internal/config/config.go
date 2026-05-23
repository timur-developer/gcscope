package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

const (
	envWindowSize   = "GCVIZ_WINDOW_SIZE"
	envSnapshotPath = "GCVIZ_SNAPSHOT_PATH"
	envAttachURL    = "GCVIZ_ATTACH_URL"
	envPollInterval = "GCVIZ_POLL_INTERVAL"
	envLabPreset    = "GCVIZ_LAB_PRESET"
	envRunTarget    = "GCVIZ_RUN_TARGET"
	envDiffA        = "GCVIZ_DIFF_A"
	envDiffB        = "GCVIZ_DIFF_B"
)

type Config struct {
	WindowSize   int
	SnapshotPath string
	Run          RunConfig
	Attach       AttachConfig
	Lab          LabConfig
	Diff         DiffConfig
}

type RunConfig struct {
	Target string
	Args   []string
}

type AttachConfig struct {
	URL          string
	PollInterval time.Duration
}

type LabConfig struct {
	Preset string
}

type DiffConfig struct {
	A string
	B string
}

func Default() Config {
	return Config{
		WindowSize: 200,
		Attach: AttachConfig{
			PollInterval: time.Second,
		},
	}
}

func Load(cmd *cobra.Command, args []string) (Config, error) {
	cfg := Default()

	if err := cfg.applyEnv(); err != nil {
		return Config{}, err
	}

	if err := cfg.applyFlags(cmd); err != nil {
		return Config{}, err
	}

	cfg.applyArgs(cmd, args)

	return cfg, nil
}

func (c *Config) applyEnv() error {
	if value, ok := os.LookupEnv(envWindowSize); ok {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid %s: %w", envWindowSize, err)
		}
		c.WindowSize = parsed
	}

	if value, ok := os.LookupEnv(envSnapshotPath); ok {
		c.SnapshotPath = value
	}

	if value, ok := os.LookupEnv(envAttachURL); ok {
		c.Attach.URL = value
	}

	if value, ok := os.LookupEnv(envPollInterval); ok {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("invalid %s: %w", envPollInterval, err)
		}
		c.Attach.PollInterval = parsed
	}

	if value, ok := os.LookupEnv(envLabPreset); ok {
		c.Lab.Preset = value
	}

	if value, ok := os.LookupEnv(envRunTarget); ok {
		c.Run.Target = value
	}

	if value, ok := os.LookupEnv(envDiffA); ok {
		c.Diff.A = value
	}

	if value, ok := os.LookupEnv(envDiffB); ok {
		c.Diff.B = value
	}

	return nil
}

func (c *Config) applyFlags(cmd *cobra.Command) error {
	if cmd.Flags().Lookup("window-size") != nil {
		value, err := cmd.Flags().GetInt("window-size")
		if err != nil {
			return err
		}
		c.WindowSize = value
	}

	if cmd.Flags().Lookup("snapshot-path") != nil {
		value, err := cmd.Flags().GetString("snapshot-path")
		if err != nil {
			return err
		}
		c.SnapshotPath = value
	}

	if cmd.Flags().Lookup("target") != nil {
		value, err := cmd.Flags().GetString("target")
		if err != nil {
			return err
		}
		c.Run.Target = value
	}

	if cmd.Flags().Lookup("url") != nil {
		value, err := cmd.Flags().GetString("url")
		if err != nil {
			return err
		}
		c.Attach.URL = value
	}

	if cmd.Flags().Lookup("poll-interval") != nil {
		value, err := cmd.Flags().GetDuration("poll-interval")
		if err != nil {
			return err
		}
		c.Attach.PollInterval = value
	}

	if cmd.Flags().Lookup("preset") != nil {
		value, err := cmd.Flags().GetString("preset")
		if err != nil {
			return err
		}
		c.Lab.Preset = value
	}

	if cmd.Flags().Lookup("a") != nil {
		value, err := cmd.Flags().GetString("a")
		if err != nil {
			return err
		}
		c.Diff.A = value
	}

	if cmd.Flags().Lookup("b") != nil {
		value, err := cmd.Flags().GetString("b")
		if err != nil {
			return err
		}
		c.Diff.B = value
	}

	return nil
}

func (c *Config) applyArgs(cmd *cobra.Command, args []string) {
	switch cmd.Name() {
	case "run":
		if len(args) > 0 {
			c.Run.Target = args[0]
			if len(args) > 1 {
				c.Run.Args = args[1:]
			}
		}
	case "lab":
		if len(args) > 0 {
			c.Lab.Preset = args[0]
		}
	case "diff":
		if len(args) > 0 {
			c.Diff.A = args[0]
		}
		if len(args) > 1 {
			c.Diff.B = args[1]
		}
	}
}
