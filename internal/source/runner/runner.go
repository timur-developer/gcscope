package runner

import (
	"bufio"
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/timur-developer/gcviz/internal/domain"
)

const shutdownTimeout = 3 * time.Second

var (
	ErrAlreadyStarted = errors.New("runner already started")
	ErrNotStarted     = errors.New("runner not started")
)

type Runner struct {
	target   string
	args     []string
	extraEnv map[string]string

	mu      sync.Mutex
	cmd     *exec.Cmd
	started bool
	waitErr error

	stderrCh chan string
	doneCh   chan struct{}
	stopOnce sync.Once

	eventCh    chan domain.GCEvent
	parseErrCh chan error
}

func NewRunner(target string, args []string, extraEnv map[string]string) *Runner {
	copied := make(map[string]string, len(extraEnv))
	for k, v := range extraEnv {
		copied[k] = v
	}

	return &Runner{
		target:     target,
		args:       append([]string(nil), args...),
		extraEnv:   copied,
		stderrCh:   make(chan string),
		eventCh:    make(chan domain.GCEvent),
		parseErrCh: make(chan error),
		doneCh:     make(chan struct{}),
	}
}

func (r *Runner) Start(ctx context.Context) error {
	r.mu.Lock()
	if r.started {
		r.mu.Unlock()
		return ErrAlreadyStarted
	}
	if r.target == "" {
		r.mu.Unlock()
		return errors.New("target is empty")
	}

	cmd := exec.Command(r.target, r.args...)
	cmd.Env = mergeEnv(os.Environ(), r.extraEnv)

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		r.mu.Unlock()
		return err
	}

	if err := cmd.Start(); err != nil {
		r.mu.Unlock()
		return err
	}

	r.cmd = cmd
	r.started = true
	r.mu.Unlock()

	go func() {
		parser := NewParser()
		defer close(r.stderrCh)
		defer close(r.eventCh)
		defer close(r.parseErrCh)

		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			r.stderrCh <- line

			event, err := parser.ParseLine(line)
			if err != nil {
				r.parseErrCh <- err
				continue
			}
			if event != nil {
				r.eventCh <- *event
			}
		}
		if event := parser.Flush(); event != nil {
			r.eventCh <- *event
		}
	}()

	go r.wait()
	go r.watchContext(ctx)

	return nil
}

func (r *Runner) Stderr() <-chan string {
	return r.stderrCh
}

func (r *Runner) Events() <-chan domain.GCEvent {
	return r.eventCh
}

func (r *Runner) ParseErrors() <-chan error {
	return r.parseErrCh
}

func (r *Runner) Wait() error {
	r.mu.Lock()
	started := r.started
	r.mu.Unlock()
	if !started {
		return ErrNotStarted
	}

	<-r.doneCh

	r.mu.Lock()
	defer r.mu.Unlock()
	return r.waitErr
}

func (r *Runner) Close() error {
	r.stop()
	return r.Wait()
}

func (r *Runner) wait() {
	err := r.cmd.Wait()

	r.mu.Lock()
	r.waitErr = err
	r.mu.Unlock()

	close(r.doneCh)
}

func (r *Runner) watchContext(ctx context.Context) {
	select {
	case <-ctx.Done():
		r.stop()
	case <-r.doneCh:
	}
}

func (r *Runner) stop() {
	r.stopOnce.Do(func() {
		r.mu.Lock()
		cmd := r.cmd
		r.mu.Unlock()
		if cmd == nil || cmd.Process == nil {
			return
		}

		_ = cmd.Process.Signal(terminateSignal())

		timer := time.NewTimer(shutdownTimeout)
		defer timer.Stop()

		select {
		case <-r.doneCh:
			return
		case <-timer.C:
			_ = cmd.Process.Kill()
		}
	})
}

func terminateSignal() os.Signal {
	if runtime.GOOS == "windows" {
		return os.Interrupt
	}
	return syscall.SIGTERM
}

func mergeEnv(base []string, extra map[string]string) []string {
	result := make(map[string]string, len(base)+len(extra)+1)
	order := make([]string, 0, len(base)+len(extra)+1)

	for _, item := range base {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		if _, exists := result[key]; !exists {
			order = append(order, key)
		}
		result[key] = parts[1]
	}

	keys := make([]string, 0, len(extra))
	for key := range extra {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if _, exists := result[key]; !exists {
			order = append(order, key)
		}
		result[key] = extra[key]
	}

	result["GODEBUG"] = normalizeGODEBUG(result["GODEBUG"])
	if !contains(order, "GODEBUG") {
		order = append(order, "GODEBUG")
	}

	merged := make([]string, 0, len(result))
	for _, key := range order {
		merged = append(merged, key+"="+result[key])
	}

	return merged
}

func normalizeGODEBUG(value string) string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts)+2)
	foundGctrace := false
	foundGcpacer := false

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		switch {
		case strings.HasPrefix(part, "gctrace="):
			if !foundGctrace {
				out = append(out, "gctrace=1")
				foundGctrace = true
			}
		case strings.HasPrefix(part, "gcpacertrace="):
			if !foundGcpacer {
				out = append(out, "gcpacertrace=1")
				foundGcpacer = true
			}
		default:
			out = append(out, part)
		}
	}

	if !foundGctrace {
		out = append(out, "gctrace=1")
	}
	if !foundGcpacer {
		out = append(out, "gcpacertrace=1")
	}

	return strings.Join(out, ",")
}

func contains(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}
