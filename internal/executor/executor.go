package executor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/kristianvld/dtask/internal/config"
	"github.com/kristianvld/dtask/internal/runtime"
)

type Result struct {
	TaskName  string
	Attempt   int
	Success   bool
	ExitCode  int
	TimedOut  bool
	StartedAt time.Time
	EndedAt   time.Time
	Duration  time.Duration
	LogPath   string
	Err       error
}

type Runner struct {
	LogRoot string
}

func NewRunner(logRoot string) *Runner {
	if strings.TrimSpace(logRoot) == "" {
		logRoot = "/tmp/dtask/logs"
	}
	return &Runner{LogRoot: logRoot}
}

func (r *Runner) Run(ctx context.Context, task config.Task, prepared runtime.Prepared, attempt int) Result {
	started := time.Now()
	res := Result{TaskName: task.Name, Attempt: attempt, StartedAt: started, ExitCode: -1}

	cwd, err := ResolveCWD(task, prepared)
	if err != nil {
		res.Err = err
		res.EndedAt = time.Now()
		res.Duration = res.EndedAt.Sub(started)
		return res
	}

	logDir := filepath.Join(r.LogRoot, task.Name)
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		res.Err = fmt.Errorf("create log dir: %w", err)
		res.EndedAt = time.Now()
		res.Duration = res.EndedAt.Sub(started)
		return res
	}
	logFile, err := os.Create(filepath.Join(logDir, fmt.Sprintf("%d-attempt-%d.log", started.Unix(), attempt)))
	if err != nil {
		res.Err = fmt.Errorf("create log file: %w", err)
		res.EndedAt = time.Now()
		res.Duration = res.EndedAt.Sub(started)
		return res
	}
	defer func() { _ = logFile.Close() }()
	res.LogPath = logFile.Name()

	cmdCtx := ctx
	cancel := func() {}
	if task.Timeout > 0 {
		cmdCtx, cancel = context.WithTimeout(ctx, task.Timeout)
	}
	defer cancel()

	cmd, err := buildCommand(cmdCtx, task, prepared, cwd)
	if err != nil {
		res.Err = err
		res.EndedAt = time.Now()
		res.Duration = res.EndedAt.Sub(started)
		return res
	}

	cmd.Stdout = io.MultiWriter(os.Stdout, logFile)
	cmd.Stderr = io.MultiWriter(os.Stderr, logFile)

	err = cmd.Run()
	res.EndedAt = time.Now()
	res.Duration = res.EndedAt.Sub(started)

	if err == nil {
		res.Success = true
		res.ExitCode = 0
		return res
	}

	res.Err = err
	if errors.Is(cmdCtx.Err(), context.DeadlineExceeded) {
		res.TimedOut = true
	}

	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if ws, ok := ee.Sys().(syscall.WaitStatus); ok {
			res.ExitCode = ws.ExitStatus()
		}
	}
	if res.ExitCode < 0 {
		res.ExitCode = 1
	}

	return res
}

func ResolveCWD(task config.Task, prepared runtime.Prepared) (string, error) {
	cwd := strings.TrimSpace(task.CWD)
	if cwd == "" {
		return "", fmt.Errorf("task %q has empty cwd", task.Name)
	}

	if filepath.IsAbs(cwd) {
		return filepath.Clean(cwd), nil
	}

	switch task.Run {
	case config.RunContainer, config.RunHost:
		return filepath.Clean(filepath.Join("/", cwd)), nil
	case config.RunCompose:
		if strings.TrimSpace(prepared.ComposeDir) == "" {
			return "", fmt.Errorf("compose directory is not resolved")
		}
		return filepath.Clean(filepath.Join(prepared.ComposeDir, cwd)), nil
	default:
		return "", fmt.Errorf("unknown run mode %q", task.Run)
	}
}

func buildCommand(ctx context.Context, task config.Task, prepared runtime.Prepared, cwd string) (*exec.Cmd, error) {
	if len(task.ShellArgv) == 0 {
		return nil, fmt.Errorf("task %q has empty shell", task.Name)
	}

	shellArgv := append([]string(nil), task.ShellArgv...)

	switch task.Run {
	case config.RunContainer:
		cmdArgv := append(shellArgv, task.Cmd)
		cmd := exec.CommandContext(ctx, cmdArgv[0], cmdArgv[1:]...)
		cmd.Dir = cwd
		if u := strings.TrimSpace(task.User); u != "" {
			cred, err := parseContainerUser(u)
			if err != nil {
				return nil, fmt.Errorf("task %q has invalid container user %q: %w", task.Name, u, err)
			}
			cmd.SysProcAttr = &syscall.SysProcAttr{Credential: cred}
		}
		return cmd, nil
	case config.RunHost, config.RunCompose:
		cmdText := fmt.Sprintf("cd %s && %s", shellQuote(cwd), task.Cmd)
		chrootArgv := []string{}
		if u := strings.TrimSpace(task.User); u != "" {
			chrootArgv = append(chrootArgv, "--userspec="+u)
		}
		chrootArgv = append(chrootArgv, "/host")
		chrootArgv = append(chrootArgv, shellArgv...)
		chrootArgv = append(chrootArgv, cmdText)
		return exec.CommandContext(ctx, "chroot", chrootArgv...), nil
	default:
		return nil, fmt.Errorf("unknown run mode %q", task.Run)
	}
}

func parseContainerUser(v string) (*syscall.Credential, error) {
	parts := strings.Split(v, ":")
	if len(parts) > 2 {
		return nil, fmt.Errorf("expected <uid> or <uid>:<gid>")
	}

	uid, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("expected numeric uid")
	}

	gid := uid
	if len(parts) == 2 {
		gid, err = strconv.ParseUint(parts[1], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("expected numeric gid")
		}
	}

	return &syscall.Credential{
		Uid: uint32(uid),
		Gid: uint32(gid),
	}, nil
}

func shellQuote(v string) string {
	return "'" + strings.ReplaceAll(v, "'", "'\\''") + "'"
}
