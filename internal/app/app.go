package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/kristianvld/dtask/internal/config"
	"github.com/kristianvld/dtask/internal/executor"
	"github.com/kristianvld/dtask/internal/notify"
	"github.com/kristianvld/dtask/internal/runtime"
)

type notifier interface {
	Send(ctx context.Context, req notify.Request) error
}

type taskRunner interface {
	Run(ctx context.Context, task config.Task, prepared runtime.Prepared, attempt int) executor.Result
}

type App struct {
	cfg      config.Config
	prepared runtime.Prepared
	runner   taskRunner
	notifier notifier
	logger   *log.Logger
}

func Run(ctx context.Context, env []string) error {
	cfg, err := config.ParseEnvironment(env)
	if err != nil {
		return err
	}

	prepared, err := runtime.Prepare(&cfg, runtime.RealEnv{})
	if err != nil {
		return err
	}

	var notifierImpl notifier
	if hasNotifyURL(cfg) {
		s, err := notify.NewCommandSender()
		if err != nil {
			return err
		}
		notifierImpl = s
	}

	a := &App{
		cfg:      cfg,
		prepared: prepared,
		runner:   executor.NewRunner(""),
		notifier: notifierImpl,
		logger:   log.New(os.Stdout, "", log.LstdFlags),
	}

	return a.Start(ctx)
}

func hasNotifyURL(cfg config.Config) bool {
	for _, t := range cfg.Tasks {
		if strings.TrimSpace(t.NotifyURL) != "" {
			return true
		}
	}
	return false
}

func (a *App) Start(ctx context.Context) error {
	var wg sync.WaitGroup
	for _, task := range a.cfg.Tasks {
		task := task
		wg.Add(1)
		go func() {
			defer wg.Done()
			a.runTaskLoop(ctx, task)
		}()
	}

	<-ctx.Done()
	wg.Wait()
	return nil
}

func (a *App) runTaskLoop(ctx context.Context, task config.Task) {
	loc := runtime.ResolveLocation(task, a.prepared)
	tickCh := make(chan time.Time, 1)
	doneCh := make(chan struct{}, 1)
	running := false

	go a.emitTicks(ctx, task, loc, tickCh)

	for {
		select {
		case <-ctx.Done():
			return
		case <-doneCh:
			running = false
		case <-tickCh:
			if running {
				a.logger.Printf("task=%s status=skipped reason=overlap", task.Name)
				a.maybeNotifySkip(ctx, task)
				continue
			}
			running = true
			go func() {
				a.executeTaskWithPolicy(ctx, task)
				doneCh <- struct{}{}
			}()
		}
	}
}

func (a *App) emitTicks(ctx context.Context, task config.Task, loc *time.Location, out chan<- time.Time) {
	after := time.Now().In(loc)
	for {
		next := task.Schedule.Next(after, loc)
		wait := time.Until(next)
		if wait < 0 {
			wait = 0
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case t := <-timer.C:
			after = next
			select {
			case out <- t:
			case <-ctx.Done():
				return
			}
		}
	}
}

func (a *App) executeTaskWithPolicy(ctx context.Context, task config.Task) {
	attempt := 1
	for {
		res := a.runner.Run(ctx, task, a.prepared, attempt)
		if res.Success {
			a.logger.Printf("task=%s status=success attempt=%d duration=%s", task.Name, attempt, res.Duration)
			a.maybeNotifyResult(ctx, task, "success", res)
			return
		}

		willRetry := task.Retry == -1 || attempt <= task.Retry
		if willRetry {
			a.logger.Printf("task=%s status=failed attempt=%d retrying=true err=%v", task.Name, attempt, res.Err)
			a.maybeNotifyResult(ctx, task, "retry_fail", res)
			delay := task.Backoff.Delay(attempt, nil)
			if !a.wait(ctx, delay) {
				return
			}
			attempt++
			continue
		}

		a.logger.Printf("task=%s status=failed attempt=%d retrying=false err=%v", task.Name, attempt, res.Err)
		a.maybeNotifyResult(ctx, task, "terminal_fail", res)
		return
	}
}

func (a *App) wait(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func (a *App) maybeNotifySkip(ctx context.Context, task config.Task) {
	if a.notifier == nil || strings.TrimSpace(task.NotifyURL) == "" {
		return
	}
	if task.Notify != config.NotifyAlways {
		return
	}

	_ = a.sendNotificationWithRetry(ctx, task, notify.Request{
		URL:        task.NotifyURL,
		NotifyType: "warning",
		Title:      fmt.Sprintf("⏭️ dtask - %s skipped", task.Name),
		Body: fmt.Sprintf(
			"Task %q was skipped because a previous run is still in progress.\n\nNo action is needed unless this repeats frequently.",
			task.Name,
		),
	})
}

func (a *App) maybeNotifyResult(ctx context.Context, task config.Task, event string, res executor.Result) {
	if a.notifier == nil || strings.TrimSpace(task.NotifyURL) == "" {
		return
	}

	if !shouldNotify(task.Notify, event) {
		return
	}

	req := notify.Request{
		URL:        task.NotifyURL,
		NotifyType: eventNotifyType(event),
		Title:      fmt.Sprintf("%s dtask - %s", eventHeadline(event), task.Name),
		Body: fmt.Sprintf(
			"%s for task %q.\n\nAttempt: %d\nExit code: %d\nTimed out: %s\nDuration: %s\nError: %s",
			eventSentence(event),
			task.Name,
			res.Attempt,
			res.ExitCode,
			boolLabel(res.TimedOut),
			res.Duration.Round(time.Millisecond),
			errorLabel(res.Err),
		),
	}

	if includeLog(task.NotifyAttachLog, event) && res.LogPath != "" {
		req.Attachments = append(req.Attachments, res.LogPath)
		req.Body += "\nA full execution log is attached."
	}

	if err := a.sendNotificationWithRetry(ctx, task, req); err != nil {
		a.logger.Printf("task=%s status=notify_failed err=%v", task.Name, err)
	}
}

func eventHeadline(event string) string {
	switch event {
	case "success":
		return "✅ Task completed"
	case "retry_fail":
		return "⚠️ Task failed (retrying)"
	case "terminal_fail":
		return "❌ Task failed"
	default:
		return "ℹ️ Task update"
	}
}

func eventSentence(event string) string {
	switch event {
	case "success":
		return "Task run completed successfully"
	case "retry_fail":
		return "Task run failed but will be retried"
	case "terminal_fail":
		return "Task run failed and no more retries are scheduled"
	default:
		return "Task state changed"
	}
}

func eventNotifyType(event string) string {
	switch event {
	case "success":
		return "success"
	case "retry_fail":
		return "warning"
	case "terminal_fail":
		return "failure"
	default:
		return "info"
	}
}

func boolLabel(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func errorLabel(err error) string {
	if err == nil {
		return "none"
	}
	return err.Error()
}

func shouldNotify(policy config.NotifyPolicy, event string) bool {
	switch policy {
	case config.NotifyNever:
		return false
	case config.NotifyFail:
		return event == "terminal_fail"
	case config.NotifyRetry:
		return event == "retry_fail" || event == "terminal_fail"
	case config.NotifySuccess:
		return event == "success"
	case config.NotifyAlways:
		return true
	default:
		return false
	}
}

func includeLog(policy config.NotifyAttachPolicy, event string) bool {
	switch policy {
	case config.AttachNever:
		return false
	case config.AttachAlways:
		return true
	case config.AttachFail:
		return event == "retry_fail" || event == "terminal_fail"
	default:
		return false
	}
}

func (a *App) sendNotificationWithRetry(ctx context.Context, task config.Task, req notify.Request) error {
	if a.notifier == nil {
		return nil
	}

	attempt := 1
	for {
		err := a.notifier.Send(ctx, req)
		if err == nil {
			return nil
		}

		canRetry := task.NotifyRetry == -1 || attempt <= task.NotifyRetry
		if !canRetry {
			return err
		}
		delay := task.NotifyBackoff.Delay(attempt, nil)
		if !a.wait(ctx, delay) {
			return ctx.Err()
		}
		attempt++
	}
}
