# gcviz — Go Garbage Collector Visualizer

![gcviz demo](docs/assets/demo_hero.gif)

[![CI](https://github.com/timur-developer/gcviz/actions/workflows/ci.yml/badge.svg)](https://github.com/timur-developer/gcviz/actions/workflows/ci.yml)
![Go](https://img.shields.io/badge/go-1.25%2B-00ADD8?logo=go&logoColor=white)
![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)

Read this in other languages: [Russian](docs/README.ru.md)

`gcviz` is a terminal UI (TUI) visualizer for Go GC behavior: GC cycles, STW pauses, heap dynamics, and pacing, live.

The main mode is `run`: it launches your Go binary with `GODEBUG=gctrace=1,gcpacertrace=1` and parses stderr. No code changes required.

## Quickstart

![gcviz launch](docs/assets/demo_launch.gif)

Prereqs: Go 1.25+, a reasonably sized terminal.

Run a built-in demo workload:

```bash
go run ./cmd/gcviz lab churn
```

Or via Makefile:

```bash
make lab-churn
```

## Usage

### run (primary)

Run your Go program under observation:

```bash
gcviz run ./path/to/your-binary -- --your-flag value
```

From source (no install):

```bash
go run ./cmd/gcviz run ./path/to/your-binary -- --your-flag value
```

Makefile shortcut:

```bash
make run TARGET=./path/to/your-binary ARGS="-- --your-flag value"
```

### lab

Built-in demo presets:

```bash
gcviz lab alloc
gcviz lab churn
gcviz lab idle
gcviz lab spike
```

### attach (secondary)

Attach to a running service that exposes `runtime/metrics` in `gcviz` JSON format.

1. Add `pkg/reporter` to your service:

```go
package main

import (
	"log"
	"net/http"

	"github.com/timur-developer/gcviz/pkg/reporter"
)

func main() {
	rep := reporter.New()
	mux := http.NewServeMux()
	mux.Handle(rep.Path(), rep.Handler())
	log.Fatal(http.ListenAndServe(":8080", mux))
}
```

2. Attach:

```bash
gcviz attach http://127.0.0.1:8080/gcviz/metrics
```

### diff

Compare two snapshots:

```bash
gcviz diff ./a.json ./b.json
```

## Controls

![gcviz features](docs/assets/demo_features.gif)

Core:

- `?` / `h` / `f1` toggle help
- `q` quit
- `space` pause/resume live updates
- `left` / `right` scrub history when paused
- `s` write a snapshot

Layout and labels:

- `g` toggle layout (spaced/tight)
- `l` toggle STW bar labels mode

Charts:

- `z` switch focused chart (Heap/STW)
- `+` / `-` zoom Y for focused chart
- `0` reset Y zoom/pan for focused chart
- `[` / `]` zoom time span (X axis)
- `shift+up` / `shift+down` pan Y for focused chart
- `r` reset all zoom/pan and time span

## Configuration

Global flags (and their env overrides):

- `--window-size` (`GCVIZ_WINDOW_SIZE`) samples kept in memory (default: 200)
- `--snapshot-path` (`GCVIZ_SNAPSHOT_PATH`) snapshot directory (default: `tmp/snapshots`)
- `--exit-snapshot` (`GCVIZ_EXIT_SNAPSHOT`) write a snapshot on exit (default: true)
- `--no-alt-screen` (`GCVIZ_NO_ALT_SCREEN`) disable alt screen buffer
- `--stw-warn-us` (`GCVIZ_STW_WARN_US`) STW warning threshold (default: 200)
- `--stw-bad-us` (`GCVIZ_STW_BAD_US`) STW bad threshold (default: 1000)

Mode-specific env vars:

- `GCVIZ_RUN_TARGET`
- `GCVIZ_ATTACH_URL`, `GCVIZ_POLL_INTERVAL`
- `GCVIZ_LAB_PRESET`
- `GCVIZ_DIFF_A`, `GCVIZ_DIFF_B`

## Snapshots

- Default directory: `tmp/snapshots`
- Manual snapshot: press `s`
- Exit snapshot: enabled by default; skipped if a manual snapshot was created recently

## Notes / FAQ

- `attach` mode cannot see the target process env (`GOGC`, `GOMEMLIMIT`, `GODEBUG`), so UI shows `n/a` for these fields.
- Very small STW values may display as 0 due to `gctrace` formatting.

## Development

```bash
make ci
make lint
make test
make build
```

Maintainers:

```bash
make testbin
make release-snapshot
```

## License

MIT. See [LICENSE](LICENSE).

