package runner

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeGODEBUG(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "empty",
			input: "",
			want:  []string{"gctrace=1", "gcpacertrace=1"},
		},
		{
			name:  "already set",
			input: "gctrace=1,gcpacertrace=1",
			want:  []string{"gctrace=1", "gcpacertrace=1"},
		},
		{
			name:  "override values",
			input: "gctrace=0,gcpacertrace=0",
			want:  []string{"gctrace=1", "gcpacertrace=1"},
		},
		{
			name:  "keeps other flags",
			input: "panic=1,gctrace=0",
			want:  []string{"panic=1", "gctrace=1", "gcpacertrace=1"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeGODEBUG(tc.input)
			gotParts := strings.Split(got, ",")
			if !containsAll(gotParts, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, gotParts)
			}
		})
	}
}

func TestMergeEnv(t *testing.T) {
	base := []string{
		"PATH=/bin",
		"GODEBUG=panic=1,gctrace=0",
	}

	extra := map[string]string{
		"FOO": "bar",
	}

	merged := mergeEnv(base, extra)
	got := envSliceToMap(merged)

	want := map[string]string{
		"PATH":    "/bin",
		"FOO":     "bar",
		"GODEBUG": "panic=1,gctrace=1,gcpacertrace=1",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestRunnerEmitsParsedEvents(t *testing.T) {
	runner := NewRunner(os.Args[0], []string{"-test.run=TestRunnerHelperProcess"}, map[string]string{
		"GCVIZ_RUNNER_HELPER": "1",
	})

	if err := runner.Start(context.Background()); err != nil {
		t.Fatalf("unexpected start error: %v", err)
	}

	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		for range runner.Stderr() {
		}
	}()

	parseErrDone := make(chan struct{})
	go func() {
		defer close(parseErrDone)
		for err := range runner.ParseErrors() {
			t.Errorf("unexpected parse error: %v", err)
		}
	}()

	var events []int
	for event := range runner.Events() {
		events = append(events, event.GCNum)
	}

	<-stderrDone
	<-parseErrDone

	if err := runner.Wait(); err != nil {
		t.Fatalf("unexpected wait error: %v", err)
	}

	if !reflect.DeepEqual(events, []int{1}) {
		t.Fatalf("expected parsed GC event 1, got %v", events)
	}
}

func TestRunnerHelperProcess(t *testing.T) {
	if os.Getenv("GCVIZ_RUNNER_HELPER") != "1" {
		return
	}

	fmt.Fprintln(os.Stderr, "gc 1 @0.041s 1%: 0.53+0.55+0 ms clock, 8.6+0/0/0+0 ms cpu, 3->4->1 MB, 4 MB goal, 0 MB stacks, 0 MB globals, 16 P")
	os.Exit(0)
}

func envSliceToMap(items []string) map[string]string {
	result := make(map[string]string, len(items))
	for _, item := range items {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) != 2 {
			continue
		}
		result[parts[0]] = parts[1]
	}
	return result
}

func containsAll(haystack []string, needles []string) bool {
	set := make(map[string]struct{}, len(haystack))
	for _, item := range haystack {
		set[item] = struct{}{}
	}
	for _, needle := range needles {
		if _, ok := set[needle]; !ok {
			return false
		}
	}
	return true
}
