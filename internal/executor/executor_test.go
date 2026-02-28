package executor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kristianvld/dtask/internal/config"
	"github.com/kristianvld/dtask/internal/runtime"
	"github.com/kristianvld/dtask/internal/schedule"
)

func TestResolveCWD(t *testing.T) {
	t.Parallel()
	base := config.Task{Name: "x", Options: config.Options{Run: config.RunContainer, CWD: "tmp", ShellArgv: []string{"/bin/sh", "-lc"}}}
	cwd, err := ResolveCWD(base, runtime.Prepared{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if cwd != "/tmp" {
		t.Fatalf("cwd=%s", cwd)
	}

	base.Run = config.RunCompose
	cwd, err = ResolveCWD(base, runtime.Prepared{ComposeDir: "/srv/stack"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if cwd != "/srv/stack/tmp" {
		t.Fatalf("cwd=%s", cwd)
	}
}

func TestRunContainerMode(t *testing.T) {
	t.Parallel()
	spec, _ := schedule.Parse("1h", "task")
	task := config.Task{
		Name: "task",
		Options: config.Options{
			Run:       config.RunContainer,
			CWD:       ".",
			ShellArgv: []string{"/bin/sh", "-lc"},
			Timeout:   5 * time.Second,
		},
		Schedule: spec,
		Cmd:      "echo hello",
	}

	logRoot := filepath.Join(t.TempDir(), "logs")
	r := NewRunner(logRoot)
	res := r.Run(context.Background(), task, runtime.Prepared{}, 1)
	if !res.Success {
		t.Fatalf("expected success, err=%v", res.Err)
	}
	if res.LogPath == "" {
		t.Fatalf("log path missing")
	}
	if _, err := os.Stat(res.LogPath); err != nil {
		t.Fatalf("log file missing: %v", err)
	}
}

func TestRunTimeout(t *testing.T) {
	t.Parallel()
	spec, _ := schedule.Parse("1h", "task")
	task := config.Task{
		Name: "task",
		Options: config.Options{
			Run:       config.RunContainer,
			CWD:       ".",
			ShellArgv: []string{"/bin/sh", "-lc"},
			Timeout:   50 * time.Millisecond,
		},
		Schedule: spec,
		Cmd:      "sleep 1",
	}

	r := NewRunner(filepath.Join(t.TempDir(), "logs"))
	res := r.Run(context.Background(), task, runtime.Prepared{}, 1)
	if res.Success {
		t.Fatalf("expected failure")
	}
	if !res.TimedOut {
		t.Fatalf("expected timeout")
	}
}
