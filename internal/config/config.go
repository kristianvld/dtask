package config

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kristianvld/dtask/internal/backoff"
	"github.com/kristianvld/dtask/internal/duration"
	"github.com/kristianvld/dtask/internal/notify"
	"github.com/kristianvld/dtask/internal/schedule"
)

type RunMode string

type NotifyPolicy string

type NotifyAttachPolicy string

const (
	RunContainer RunMode = "container"
	RunHost      RunMode = "host"
	RunCompose   RunMode = "compose"

	NotifyNever   NotifyPolicy = "never"
	NotifyFail    NotifyPolicy = "fail"
	NotifyRetry   NotifyPolicy = "retry"
	NotifySuccess NotifyPolicy = "success"
	NotifyAlways  NotifyPolicy = "always"

	AttachNever  NotifyAttachPolicy = "never"
	AttachFail   NotifyAttachPolicy = "fail"
	AttachAlways NotifyAttachPolicy = "always"

	defaultRun             = "container"
	defaultCWD             = "."
	defaultTZ              = "auto"
	defaultShell           = "/bin/bash -lc"
	defaultTimeout         = "0"
	defaultRetry           = "0"
	defaultBackoff         = "exp:10s:5m:2:0.1"
	defaultNotify          = "fail"
	defaultNotifyURL       = ""
	defaultNotifyAttachLog = "fail"
	defaultNotifyBackoff   = "exp:10s:10m:2:0.1"
	defaultNotifyRetry     = "-1"
)

type Options struct {
	Run             RunMode
	CWD             string
	TZ              string
	Location        *time.Location
	Shell           string
	ShellArgv       []string
	Timeout         time.Duration
	Retry           int
	Backoff         backoff.Strategy
	Notify          NotifyPolicy
	NotifyURL       string
	NotifyAttachLog NotifyAttachPolicy
	NotifyBackoff   backoff.Strategy
	NotifyRetry     int
}

type Task struct {
	Name string
	Options
	Schedule schedule.Spec
	Cmd      string
}

type Config struct {
	Tasks []Task
}

var taskNameRE = regexp.MustCompile(`^[a-z0-9_]+$`)

var nonDtaskAllowlist = map[string]struct{}{
	"PATH":     {},
	"HOSTNAME": {},
	"HOME":     {},
	"PWD":      {},
	"TERM":     {},
	"SHLVL":    {},
	"_":        {},
}

var globalOptionKeys = map[string]struct{}{
	"run":               {},
	"cwd":               {},
	"tz":                {},
	"shell":             {},
	"timeout":           {},
	"retry":             {},
	"backoff":           {},
	"notify":            {},
	"notify_url":        {},
	"notify_attach_log": {},
	"notify_backoff":    {},
	"notify_retry":      {},
}

var allOptionKeys = map[string]struct{}{
	"run":               {},
	"cwd":               {},
	"tz":                {},
	"shell":             {},
	"timeout":           {},
	"retry":             {},
	"backoff":           {},
	"notify":            {},
	"notify_url":        {},
	"notify_attach_log": {},
	"notify_backoff":    {},
	"notify_retry":      {},
	"schedule":          {},
	"cmd":               {},
}

func ParseEnvironment(env []string) (Config, error) {
	if len(env) == 0 {
		env = os.Environ()
	}

	globalRaw := map[string]string{}
	taskRaw := map[string]map[string]string{}

	for _, entry := range env {
		k, v, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}

		if _, ok := nonDtaskAllowlist[k]; ok {
			continue
		}

		if strings.Contains(k, ".") {
			parts := strings.Split(k, ".")
			if len(parts) != 2 {
				return Config{}, fmt.Errorf("unknown configuration key %q", k)
			}
			taskName := parts[0]
			opt := parts[1]

			if !taskNameRE.MatchString(taskName) {
				return Config{}, fmt.Errorf("invalid task name %q: must match ^[a-z0-9_]+$", taskName)
			}
			if _, ok := allOptionKeys[opt]; !ok {
				return Config{}, fmt.Errorf("unknown configuration key %q", k)
			}
			if _, ok := taskRaw[taskName]; !ok {
				taskRaw[taskName] = map[string]string{}
			}
			taskRaw[taskName][opt] = v
			continue
		}

		if _, ok := globalOptionKeys[k]; !ok {
			return Config{}, fmt.Errorf("unknown configuration key %q", k)
		}
		globalRaw[k] = v
	}

	if len(taskRaw) == 0 {
		return Config{}, fmt.Errorf("at least one task must define <task>.schedule and <task>.cmd")
	}

	base, err := defaultOptions()
	if err != nil {
		return Config{}, err
	}
	if err := applyOptionMap(&base, globalRaw, "global"); err != nil {
		return Config{}, err
	}

	names := make([]string, 0, len(taskRaw))
	for name := range taskRaw {
		names = append(names, name)
	}
	sort.Strings(names)

	tasks := make([]Task, 0, len(names))
	for _, name := range names {
		raw := taskRaw[name]
		if strings.TrimSpace(raw["schedule"]) == "" {
			return Config{}, fmt.Errorf("task %q is missing required key %q", name, "schedule")
		}
		if strings.TrimSpace(raw["cmd"]) == "" {
			return Config{}, fmt.Errorf("task %q is missing required key %q", name, "cmd")
		}

		t := base
		if err := applyOptionMap(&t, raw, fmt.Sprintf("task %q", name)); err != nil {
			return Config{}, err
		}

		spec, err := schedule.Parse(raw["schedule"], name)
		if err != nil {
			return Config{}, fmt.Errorf("task %q has invalid schedule: %w", name, err)
		}

		if t.NotifyAttachLog != AttachNever && strings.TrimSpace(t.NotifyURL) != "" {
			ok, err := notify.SupportsAttachment(t.NotifyURL)
			if err != nil {
				return Config{}, err
			}
			if !ok {
				return Config{}, fmt.Errorf("task %q notify_url scheme does not support attachments", name)
			}
		}

		tasks = append(tasks, Task{
			Name:     name,
			Options:  t,
			Schedule: spec,
			Cmd:      raw["cmd"],
		})
	}

	return Config{Tasks: tasks}, nil
}

func defaultOptions() (Options, error) {
	timeout, err := duration.Parse(defaultTimeout)
	if err != nil {
		return Options{}, err
	}
	bo, err := backoff.Parse(defaultBackoff)
	if err != nil {
		return Options{}, err
	}
	nbo, err := backoff.Parse(defaultNotifyBackoff)
	if err != nil {
		return Options{}, err
	}
	argv := strings.Fields(defaultShell)
	if len(argv) == 0 {
		return Options{}, fmt.Errorf("default shell is invalid")
	}

	return Options{
		Run:             RunMode(defaultRun),
		CWD:             defaultCWD,
		TZ:              defaultTZ,
		Shell:           defaultShell,
		ShellArgv:       argv,
		Timeout:         timeout,
		Retry:           0,
		Backoff:         bo,
		Notify:          NotifyPolicy(defaultNotify),
		NotifyURL:       defaultNotifyURL,
		NotifyAttachLog: NotifyAttachPolicy(defaultNotifyAttachLog),
		NotifyBackoff:   nbo,
		NotifyRetry:     -1,
	}, nil
}

func applyOptionMap(o *Options, raw map[string]string, scope string) error {
	for key, value := range raw {
		if key == "schedule" || key == "cmd" {
			continue
		}
		if err := applyOption(o, key, value, scope); err != nil {
			return err
		}
	}
	return nil
}

func applyOption(o *Options, key, value, scope string) error {
	value = strings.TrimSpace(value)

	switch key {
	case "run":
		switch RunMode(value) {
		case RunContainer, RunHost, RunCompose:
			o.Run = RunMode(value)
		default:
			return fmt.Errorf("%s has invalid run %q", scope, value)
		}
	case "cwd":
		if value == "" {
			return fmt.Errorf("%s has empty cwd", scope)
		}
		o.CWD = value
	case "tz":
		if value == "" {
			return fmt.Errorf("%s has empty tz", scope)
		}
		if value == "auto" {
			o.TZ = value
			o.Location = nil
			return nil
		}
		loc, err := time.LoadLocation(value)
		if err != nil {
			return fmt.Errorf("%s has invalid tz %q", scope, value)
		}
		o.TZ = value
		o.Location = loc
	case "shell":
		argv := strings.Fields(value)
		if len(argv) == 0 {
			return fmt.Errorf("%s has invalid shell %q", scope, value)
		}
		o.Shell = value
		o.ShellArgv = argv
	case "timeout":
		d, err := duration.Parse(value)
		if err != nil {
			return fmt.Errorf("%s has invalid timeout %q", scope, value)
		}
		if d < 0 {
			return fmt.Errorf("%s has invalid timeout %q", scope, value)
		}
		o.Timeout = d
	case "retry":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("%s has invalid retry %q", scope, value)
		}
		if n < -1 {
			return fmt.Errorf("%s has invalid retry %q", scope, value)
		}
		o.Retry = n
	case "backoff":
		bo, err := backoff.Parse(value)
		if err != nil {
			return fmt.Errorf("%s has invalid backoff %q: %w", scope, value, err)
		}
		o.Backoff = bo
	case "notify":
		switch NotifyPolicy(value) {
		case NotifyNever, NotifyFail, NotifyRetry, NotifySuccess, NotifyAlways:
			o.Notify = NotifyPolicy(value)
		default:
			return fmt.Errorf("%s has invalid notify %q", scope, value)
		}
	case "notify_url":
		if err := notify.ValidateURL(value); err != nil {
			return fmt.Errorf("%s: %w", scope, err)
		}
		o.NotifyURL = value
	case "notify_attach_log":
		switch NotifyAttachPolicy(value) {
		case AttachNever, AttachFail, AttachAlways:
			o.NotifyAttachLog = NotifyAttachPolicy(value)
		default:
			return fmt.Errorf("%s has invalid notify_attach_log %q", scope, value)
		}
	case "notify_backoff":
		bo, err := backoff.Parse(value)
		if err != nil {
			return fmt.Errorf("%s has invalid notify_backoff %q: %w", scope, value, err)
		}
		o.NotifyBackoff = bo
	case "notify_retry":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("%s has invalid notify_retry %q", scope, value)
		}
		if n < -1 {
			return fmt.Errorf("%s has invalid notify_retry %q", scope, value)
		}
		o.NotifyRetry = n
	default:
		return fmt.Errorf("unknown configuration key %q", key)
	}

	return nil
}
