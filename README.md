# gcviz

TUI visualizer for Go GC behavior.

## Quickstart (from source)

If you cloned the repository and want to run without installing anything globally:

```bash
make lab-alloc
```

Other scenarios:

```bash
make lab-churn
make lab-idle
make lab-spike

make run TARGET=./myservice ARGS="-- --config ./config.yml"
make attach URL=http://127.0.0.1:8080/gcviz/metrics
make diff A=./a.json B=./b.json
```

If you don't have `make`, use `go run` directly:

```bash
go run ./cmd/gcviz lab alloc
go run ./cmd/gcviz run ./myservice -- --config ./config.yml
go run ./cmd/gcviz attach http://127.0.0.1:8080/gcviz/metrics
go run ./cmd/gcviz diff ./a.json ./b.json
```

## Install

```bash
go install github.com/timur-developer/gcviz/cmd/gcviz@latest
```

## Usage

### Lab mode

Run a built-in demo workload (no external services required):

```bash
gcviz lab alloc
gcviz lab churn
gcviz lab idle
gcviz lab spike
```

### Run mode

Run a target binary under observation (stderr is parsed from `GODEBUG=gctrace=1,gcpacertrace=1`):

```bash
gcviz run ./myservice -- --config ./config.yml
```

### Attach mode

Attach to a running service exposing `runtime/metrics` via `pkg/reporter`:

```bash
gcviz attach http://127.0.0.1:8080/gcviz/metrics
```

### Snapshot & diff

```bash
gcviz diff ./a.json ./b.json
```

## Attach Mode Prerequisites

`attach` mode expects a target service exposing `runtime/metrics` in `gcviz` JSON format.

If you don't want to expose `pprof`, use the embedded HTTP handler from `pkg/reporter`:

- See: `pkg/reporter/README.md`

## Embedded testbin (lab)

`lab` mode runs an embedded demo workload binary (`cmd/testbin`). Prebuilt binaries are stored in:

```
internal/source/lab/testbin/<os>_<arch>/
```

Supported targets:

- `linux_amd64`, `linux_arm64`
- `darwin_amd64`, `darwin_arm64`
- `windows_amd64`, `windows_arm64`

To rebuild embedded binaries:

```bash
make testbin
```

This uses the current Go toolchain and overwrites the files under `internal/source/lab/testbin/`.

## Releases (maintainers)

Releases are published via GoReleaser on tag push.

1. Create and push a tag (example):

```bash
git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0
```

2. GitHub Actions runs `.github/workflows/release.yml`, which executes GoReleaser using `.goreleaser.yaml` and uploads artifacts to GitHub Releases.

Local snapshot build (no publish):

```bash
make release-snapshot
```
