package config

import (
	"testing"
	"time"
)

func TestParseEnvironmentValidAllOptions(t *testing.T) {
	t.Parallel()
	env := []string{
		"PATH=/usr/bin",
		"run=container",
		"cwd=/work",
		"tz=UTC",
		"shell=/bin/sh -lc",
		"timeout=5m",
		"retry=2",
		"backoff=fixed:15s",
		"notify=retry",
		"notify_url=discord://webhook_id/webhook_token",
		"notify_attach_log=never",
		"notify_backoff=exp:10s:2m:2:0.1",
		"notify_retry=3",
		"backup.schedule=0 2 * * 0",
		"backup.cmd=echo backup",
		"backup.retry=1",
	}

	cfg, err := ParseEnvironment(env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Tasks) != 1 {
		t.Fatalf("expected one task, got %d", len(cfg.Tasks))
	}
	task := cfg.Tasks[0]
	if task.Name != "backup" {
		t.Fatalf("name=%s", task.Name)
	}
	if task.Run != RunContainer {
		t.Fatalf("run=%s", task.Run)
	}
	if task.CWD != "/work" {
		t.Fatalf("cwd=%s", task.CWD)
	}
	if task.Location != time.UTC {
		t.Fatalf("expected UTC location")
	}
	if task.Timeout != 5*time.Minute {
		t.Fatalf("timeout=%v", task.Timeout)
	}
	if task.Retry != 1 {
		t.Fatalf("retry override failed: %d", task.Retry)
	}
	if task.Notify != NotifyRetry {
		t.Fatalf("notify=%s", task.Notify)
	}
	if task.NotifyRetry != 3 {
		t.Fatalf("notify_retry=%d", task.NotifyRetry)
	}
}

func TestParseEnvironmentErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		env  []string
	}{
		{name: "unknown key", env: []string{"FOO=bar", "task.schedule=1h", "task.cmd=true"}},
		{name: "bad task name", env: []string{"Task.schedule=1h", "Task.cmd=true"}},
		{name: "missing schedule", env: []string{"task.cmd=true"}},
		{name: "missing cmd", env: []string{"task.schedule=1h"}},
		{name: "bad run", env: []string{"run=nope", "task.schedule=1h", "task.cmd=true"}},
		{name: "bad timeout", env: []string{"timeout=-1s", "task.schedule=1h", "task.cmd=true"}},
		{name: "bad retry", env: []string{"retry=-2", "task.schedule=1h", "task.cmd=true"}},
		{name: "bad backoff", env: []string{"backoff=nope", "task.schedule=1h", "task.cmd=true"}},
		{name: "bad notify", env: []string{"notify=nope", "task.schedule=1h", "task.cmd=true"}},
		{name: "bad notify_url", env: []string{"notify_url=://bad", "task.schedule=1h", "task.cmd=true"}},
		{name: "bad notify attach", env: []string{"notify_attach_log=nope", "task.schedule=1h", "task.cmd=true"}},
		{name: "bad notify backoff", env: []string{"notify_backoff=nope", "task.schedule=1h", "task.cmd=true"}},
		{name: "bad notify retry", env: []string{"notify_retry=-2", "task.schedule=1h", "task.cmd=true"}},
		{name: "invalid schedule", env: []string{"task.schedule=wat", "task.cmd=true"}},
		{name: "attachment unsupported", env: []string{"notify_url=custom://x", "notify_attach_log=always", "task.schedule=1h", "task.cmd=true"}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := ParseEnvironment(tt.env); err == nil {
				t.Fatalf("expected error")
			}
		})
	}
}

func TestAllowlistEnvVars(t *testing.T) {
	t.Parallel()
	env := []string{
		"PATH=/usr/bin",
		"HOSTNAME=container",
		"HOME=/root",
		"PWD=/",
		"TERM=xterm",
		"SHLVL=1",
		"_=/bin/dtask",
		"task.schedule=1h",
		"task.cmd=true",
	}
	if _, err := ParseEnvironment(env); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
