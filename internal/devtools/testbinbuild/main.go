package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type buildTarget struct {
	goos   string
	goarch string
	outExt string
}

func main() {
	root, err := repoRoot()
	if err != nil {
		fatal(err)
	}

	targets := []buildTarget{
		{goos: "linux", goarch: "amd64"},
		{goos: "linux", goarch: "arm64"},
		{goos: "darwin", goarch: "amd64"},
		{goos: "darwin", goarch: "arm64"},
		{goos: "windows", goarch: "amd64", outExt: ".exe"},
		{goos: "windows", goarch: "arm64", outExt: ".exe"},
	}

	for _, t := range targets {
		outDir := filepath.Join(root, "internal", "source", "lab", "testbin", t.goos+"_"+t.goarch)
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			fatal(fmt.Errorf("mkdir %s: %w", outDir, err))
		}

		outPath := filepath.Join(outDir, "testbin"+t.outExt)
		if err := buildTestbin(root, t.goos, t.goarch, outPath); err != nil {
			fatal(err)
		}
	}

	fmt.Println("ok")
}

func repoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	path := wd
	for {
		if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
			return path, nil
		}

		parent := filepath.Dir(path)
		if parent == path {
			return "", fmt.Errorf("go.mod not found (start=%s)", wd)
		}
		path = parent
	}
}

func buildTestbin(root, goos, goarch, outPath string) error {
	cmd := exec.Command("go", "build", "-trimpath", "-o", outPath, "./cmd/testbin")
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=0",
		"GOOS="+goos,
		"GOARCH="+goarch,
	)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build testbin for %s/%s: %w\n%s", goos, goarch, err, out.String())
	}
	return nil
}

func fatal(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err.Error())
	os.Exit(1)
}
