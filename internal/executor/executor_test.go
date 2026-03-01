package executor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

func TestBuildCommandHostUserspec(t *testing.T) {
	t.Parallel()
	task := config.Task{
		Name: "task",
		Options: config.Options{
			Run:       config.RunHost,
			User:      "1000:1000",
			ShellArgv: []string{"/bin/sh", "-lc"},
		},
		Cmd: "echo hello",
	}

	cmd, err := buildCommand(context.Background(), task, runtime.Prepared{}, "/tmp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	args := strings.Join(cmd.Args, " ")
	if !strings.Contains(args, "--userspec=1000:1000") {
		t.Fatalf("expected userspec arg, got: %q", args)
	}
}

func TestBuildCommandContainerUserCredential(t *testing.T) {
	t.Parallel()
	task := config.Task{
		Name: "task",
		Options: config.Options{
			Run:       config.RunContainer,
			User:      "1001:1002",
			ShellArgv: []string{"/bin/sh", "-lc"},
		},
		Cmd: "echo hello",
	}

	cmd, err := buildCommand(context.Background(), task, runtime.Prepared{}, "/tmp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd.SysProcAttr == nil || cmd.SysProcAttr.Credential == nil {
		t.Fatalf("expected credential to be set")
	}
	if cmd.SysProcAttr.Credential.Uid != 1001 || cmd.SysProcAttr.Credential.Gid != 1002 {
		t.Fatalf("unexpected credential uid=%d gid=%d", cmd.SysProcAttr.Credential.Uid, cmd.SysProcAttr.Credential.Gid)
	}
}

func TestBuildCommandContainerUserInvalid(t *testing.T) {
	t.Parallel()
	task := config.Task{
		Name: "task",
		Options: config.Options{
			Run:       config.RunContainer,
			User:      "backup",
			ShellArgv: []string{"/bin/sh", "-lc"},
		},
		Cmd: "echo hello",
	}

	_, err := buildCommand(context.Background(), task, runtime.Prepared{}, "/tmp")
	if err == nil {
		t.Fatalf("expected invalid container user error")
	}
	if !strings.Contains(err.Error(), `invalid container user "backup"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}
