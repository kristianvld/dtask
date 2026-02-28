package app

import (
	"context"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kristianvld/dtask/internal/backoff"
	"github.com/kristianvld/dtask/internal/config"
	"github.com/kristianvld/dtask/internal/executor"
	"github.com/kristianvld/dtask/internal/notify"
	"github.com/kristianvld/dtask/internal/runtime"
	"github.com/kristianvld/dtask/internal/schedule"
)

type fakeRunner struct {
	mu    sync.Mutex
	count int
	sleep time.Duration
	res   executor.Result
}

func (f *fakeRunner) Run(ctx context.Context, task config.Task, prepared runtime.Prepared, attempt int) executor.Result {
	f.mu.Lock()
	f.count++
	f.mu.Unlock()
	select {
	case <-ctx.Done():
		return executor.Result{TaskName: task.Name, Attempt: attempt, Err: ctx.Err()}
	case <-time.After(f.sleep):
	}
	res := f.res
	res.TaskName = task.Name
	res.Attempt = attempt
	if res.EndedAt.IsZero() {
		res.EndedAt = time.Now()
	}
	return res
}

func (f *fakeRunner) Count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.count
}

type fakeNotifier struct {
	mu       sync.Mutex
	requests []notify.Request
	fails    int
}

func (f *fakeNotifier) Send(_ context.Context, req notify.Request) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fails > 0 {
		f.fails--
		return os.ErrPermission
	}
	f.requests = append(f.requests, req)
	return nil
}

func (f *fakeNotifier) Requests() []notify.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]notify.Request, len(f.requests))
	copy(out, f.requests)
	return out
}

func TestNoOverlapSkips(t *testing.T) {
	t.Parallel()
	spec, err := schedule.Parse("10ms", "task")
	if err != nil {
		t.Fatalf("parse schedule: %v", err)
	}
	bo, _ := backoff.Parse("fixed:10ms")
	task := config.Task{
		Name: "task",
		Options: config.Options{
			Run:             config.RunContainer,
			CWD:             ".",
			ShellArgv:       []string{"/bin/sh", "-lc"},
			Backoff:         bo,
			NotifyBackoff:   bo,
			NotifyRetry:     0,
			Notify:          config.NotifyAlways,
			NotifyURL:       "discord://webhook_id/webhook_token",
			NotifyAttachLog: config.AttachNever,
		},
		Schedule: spec,
		Cmd:      "true",
	}

	r := &fakeRunner{sleep: 80 * time.Millisecond, res: executor.Result{Success: true}}
	n := &fakeNotifier{}
	a := &App{
		cfg:      config.Config{Tasks: []config.Task{task}},
		prepared: runtime.Prepared{},
		runner:   r,
		notifier: n,
		logger:   log.New(ioDiscard{}, "", 0),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 160*time.Millisecond)
	defer cancel()

	go a.runTaskLoop(ctx, task)
	<-ctx.Done()
	// let async worker finish processing
	time.Sleep(20 * time.Millisecond)

	if r.Count() == 0 {
		t.Fatalf("expected at least one task run")
	}

	foundSkip := false
	for _, req := range n.Requests() {
		if strings.Contains(strings.ToLower(req.Title), "skipped") {
			foundSkip = true
			break
		}
	}
	if !foundSkip {
		t.Fatalf("expected at least one skip notification")
	}
}

func TestShouldNotifyMatrix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		policy config.NotifyPolicy
		event  string
		want   bool
	}{
		{config.NotifyNever, "success", false},
		{config.NotifyFail, "terminal_fail", true},
		{config.NotifyFail, "retry_fail", false},
		{config.NotifyRetry, "retry_fail", true},
		{config.NotifyRetry, "terminal_fail", true},
		{config.NotifySuccess, "success", true},
		{config.NotifyAlways, "success", true},
		{config.NotifyAlways, "terminal_fail", true},
	}
	for _, tt := range tests {
		if got := shouldNotify(tt.policy, tt.event); got != tt.want {
			t.Fatalf("policy=%s event=%s got=%t want=%t", tt.policy, tt.event, got, tt.want)
		}
	}
}

func TestSendNotificationWithRetry(t *testing.T) {
	t.Parallel()
	bo, _ := backoff.Parse("fixed:1ms")
	task := config.Task{
		Name: "task",
		Options: config.Options{
			NotifyRetry:   2,
			NotifyBackoff: bo,
		},
	}
	n := &fakeNotifier{fails: 2}
	a := &App{notifier: n, logger: log.New(ioDiscard{}, "", 0)}

	err := a.sendNotificationWithRetry(context.Background(), task, notify.Request{URL: "discord://x/y"})
	if err != nil {
		t.Fatalf("expected success after retries: %v", err)
	}
	if len(n.Requests()) != 1 {
		t.Fatalf("expected one successful send, got %d", len(n.Requests()))
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }
