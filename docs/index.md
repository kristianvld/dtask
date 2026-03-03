# dtask

<p align="center">
  <img src="/logo.svg" alt="dtask logo" style="max-width: 16em; width:40%; min-width: 7em; background-color: #fff; padding: 1em; border-radius: 2em; border: 0.5em solid #26457D;">
</p>

`dtask` is a lightweight Docker task runner designed for Docker Compose.

Documentation site: <https://kristianvld.github.io/dtask/>

<p>
  <a href="https://github.com/kristianvld/dtask/tags"><img src="https://img.shields.io/github/v/tag/kristianvld/dtask?sort=semver&label=version" alt="Version"></a>
  <a href="https://github.com/kristianvld/dtask/actions/workflows/ci.yml"><img src="https://github.com/kristianvld/dtask/actions/workflows/ci.yml/badge.svg?branch=main" alt="CI"></a>
  <a href="https://github.com/kristianvld/dtask/actions/workflows/docs.yml"><img src="https://github.com/kristianvld/dtask/actions/workflows/docs.yml/badge.svg?branch=main" alt="Docs"></a>
  <a href="https://github.com/kristianvld/dtask/pkgs/container/dtask"><img src="https://img.shields.io/badge/image-ghcr.io%2Fkristianvld%2Fdtask-blue" alt="Image"></a>
</p>

> [!WARNING]
> `run=host` and `run=compose` rely on Linux host primitives (`chroot`, host mount layout, and Docker metadata paths). On Docker Desktop (macOS/Windows), these modes can behave unexpectedly due to the Linux VM abstraction. Prefer `run=container` there unless you have validated your setup end-to-end.

`dtask` arrose from a desire to solve the `co-location` problem of scheduling tasks on the host system outside of container context. For instance, running `docker compose up --build --pull always -d` within the current compose stack every night or schedule other host-level tasks.

One could alternativly solve this using `cron` or `systemd` timers, but that splits the location of defining the schedule for the task and the actual script to run, far away from each other. Alternativly, a bit of configuration of existing docker scheduling containers such as [mcuadros/ofelia](https://github.com/mcuadros/ofelia/) could solve this, but that would require mounting the current directory, make sure paths are correct when mounting or hardcoding the current directory path, which breaks if you copy the compose file to a different location or want to reuse a section.

This problem, can however, easily be solved with the example below:

```yaml
# compose.yml
services:
  dtask:
    image: ghcr.io/kristianvld/dtask
    restart: unless-stopped
    volumes:
      - /:/host
    environment:
      run: compose # run on host in the compose stack directory (requires `/:/host` mount)
      update.schedule: 02:00-04:00 # run every night at a random time between 02AM and 04AM
      update.cmd: docker compose up -d --build --pull always
```

Want to schedule a quick ad-hoc `bun` script to run every hour, without writing your own custom container or start configuring `cron` in a far away location?

```yaml
# example file structure:
# .
# ├── script.ts
# └── compose.yml
services:
  dtask-bun-script:
    image: ghcr.io/kristianvld/dtask
    volumes:
      - /:/host:ro # read-only host mount for compose-mode execution
    environment:
      run: compose
      task.schedule: 1h
      task.cmd: bun run script.ts # Uses `bun` from the host filesystem
```

## Design Goals

- Keep task configuration compact and readable in `compose.yml`.
- Easily define scheduled tasks within the host or container context.
- Keep defaults predictable and practical.
- Keep runtime lightweight.

## Quick Start

The following example is constructed to showcase all available options and possible values.

```yaml
# compose.yml
services:
  dtask:
    image: ghcr.io/kristianvld/dtask
    restart: unless-stopped
    volumes:
      # this gives access to the host filesystem, the container uses it to access docker, identify
      # the compose stack, current working directory, and gives access to any host installed tools
      # or commands, e.g. `docker compose` or `bun`
      - /:/host
    environment:
      # <key>: <value> - these are set for all tasks
      # <task>.<key>: <value> - these are set for the task <task>

      # Define how this task will run. Possible values are:
      # `host` will run the task on the host using chroot. All paths are relative to `/` on the host.
      # `compose` will run the task on the host, but change the current working directory to the compose stack directory.
      # `container` will run the task inside the container. All paths are relative to `/` in the container.
      # Both `host` and `compose` will throw an error if `/:/host` is not mounted under `volumes:`.
      # Default is `container`
      run: host
      # Set the working current working directory for the task. Supports absolute and relative paths. Paths are resolved relative to the `run`.
      # Default is `.`
      cwd: .
      # Set the timezone for scheduling and logging. `auto` will identify and use the host timezone if `/:/host` is mounted, otherwise it will use the default timezone of the container..
      # Other values are IANA timezones, e.g. `Europe/Amsterdam`, `America/New_York`, `Asia/Tokyo`, etc.
      # See https://www.iana.org/time-zones for timezone database information.
      # Default is `auto`
      tz: auto
      # The shell to use to parse the cmd. When in host/compose mode, this shell must exist on the host.
      # Default is `/bin/bash -lc`
      shell: /bin/bash -lc
      # Maxmimum timeout for a single task to run before it is considered failed and retried (if retries are defined).
      # Uses Go duration format: https://pkg.go.dev/time#ParseDuration
      # e.g. `10s`, `1h20m3s`, `2h30m`
      # Default is `0`
      timeout: 5m
      # Number of attempts after the first failed attempt. Mubst be a whole number. `-1` means infinite retries, `0` means no retries.
      # Default is `0` - no retries.
      retry: 3
      # Backoff strategy for retries. Supported values: `fixed:<duration>` or `exp:<initial>:<max>:<factor>:<jitter>`.
      # `fixed:<duration>` means the duration to wait before the next retry, same delay each time.
      # `exp:<initial>:<max>:<factor>:<jitter>` means the duration to wait before the next retry, exponential growth.
      # Duration values use Go format: https://pkg.go.dev/time#ParseDuration
      #   `initial` is the initial duration to wait before the first retry.
      #   `max` is the maximum duration to wait before the retry.
      #   `factor` is the factor to multiply the duration by after each retry.
      #   `jitter` is the jitter to add to the duration, as a percentage of the duration.
      #   delay = min(max, initial * factor^retry * (1 + jitter * random(0, 1)))
      # Default is `exp:10s:5m:2:0.1`
      backoff: exp:10s:5m:2:0.1
      # Notification policy for the task.
      # Supported values:
      # - `never` - no notifications
      # - `fail` - notifications when a task fails
      # - `retry` - notifications when a task is retried and fails
      # - `always` - notifications when a task is successful, retried or fails
      # - `success` - notifications when a task is successful (either first attempt or after retries)
      # Default is `fail`
      notify: fail
      # Apprise URL to send notifications to. Must be a valid Apprise URL or empty and will otherwise throw an error.
      # See https://github.com/caronc/apprise/wiki for supported URLs.
      # Default is empty - meaning no notifications will be sent
      notify_url: mailto://user:pass@example.com
      # When to attach the log of the stdout/stderr file as a file upload to the notification
      # Supported values: `never`, `fail`, `always`
      # Default is `fail`
      notify_attach_log: fail
      # Same as `backoff`, but for notifications that could not be delivered
      # Supported values: `fixed:<duration>` or `exp:<initial>:<max>:<factor>:<jitter>`
      # Duration values use Go format: https://pkg.go.dev/time#ParseDuration
      # Default is `exp:10s:10m:2:0.1`
      notify_backoff: exp:10s:10m:2:0.1
      # Same as `retry`, but for notifications that could not be delivered
      # Supported values: `-1` for infinite retries, `0` for no retries
      # Default is `-1` - infinite retries
      notify_retry: -1

      # A task named `update` that runs every hour
      # `schedule` is required for all tasks and cannot be defined globally
      # Supported formats:
      #   `HH:MM` - Run at the given time every day, e.g. `02:00` meaning 2:00 AM or `14:00` meaning 2:00 PM
      #   `HH:MM-HH:MM` - Run once between the given times, e.g. `02:00-04:00` meaning 2:00 AM to 4:00 AM, a random time will be picked.
      #   `<duration>` - Go duration format: https://pkg.go.dev/time#ParseDuration, e.g. `1h`, `1h30m`, `1h30m30s`, `30s`
      #   `<cron expression>` - Same format as `cron`, e.g. `0 14 * * 0` for 2:00 PM on Sundays
      update.schedule: 02:00-04:00 # Randomly run once between 02:00 and 04:00
      # The command to run when a task is scheduled
      # The command will be parsed by the `shell`. Shell parsing depends on the `shell` specified. Path is relative to `cwd`.
      # `cmd` is required for all tasks and cannot be defined globally
      update.cmd: docker compose up -d --build --pull always
      # Example override, overrides `retry` for this task only.
      update.retry: 0

  dtask-backup-example:
    image: ghcr.io/kristianvld/dtask
    restart: unless-stopped
    volumes:
      - ./backups:/backups
      - ./app:/app:ro
    environment:
      notify_url: mailto://user:pass@example.com
      # Task: incremental backup every hour
      incremental.schedule: 1h
      incremental.timeout: 10m
      incremental.retry: 1
      incremental.notify_attach_log: fail
      # Default image has busybox installed, so we can use `tar` and `date` commands.
      incremental.cmd: tar czf /backups/incr-$(date +%F-%H%M%S).tar.gz --listed-incremental=/backups/backup.snar /app/data

      # Task: full backup every Sunday at 2:00 AM
      full.schedule: 0 2 * * 0
      full.timeout: 30m
      full.retry: 3
      full.notify: always
      full.notify_attach_log: fail
      # Full backup with --listed-incremental new file to reset the base
      full.cmd: tar czf /backups/full-$(date +%F-%H%M%S).tar.gz --listed-incremental=/backups/backup.snar.new /app/data && mv /backups/backup.snar.new /backups/backup.snar
```

## Configuration Model

### Key Notation

Use these key formats in `environment`:

- Root-scoped option: `<key>`
- Task-scoped option: `<task>.<key>`
- Task names (`<task>`) must match `^[a-z0-9_]+$`

Example:

- `retry: 3`
- `stack_update.retry: 0`
- `hourly_backup.notify: always`

Invalid task names (for example `Stack-Update.schedule` or `my.task.cmd`) fail startup validation.

### Precedence

Configuration is applied in this order:

1. hardcoded defaults in `dtask`
2. global options (`<key>`)
3. task options (`<task>.<key>`)

Task options override globals for that task only.

### Required Task Keys

Each task must define:

- `<task>.schedule`
- `<task>.cmd`

At least one task must be defined. A service with no `<task>.schedule` + `<task>.cmd` pairs is invalid.

### Environment Validation

`dtask` validates container environment keys at startup and fails fast on invalid configuration.

- unknown keys are rejected
- non-`dtask` keys are only allowed for this fixed allowlist:
  - `PATH`
  - `HOSTNAME`
  - `HOME`
  - `PWD`
  - `TERM`
  - `SHLVL`
  - `_`

This strict mode avoids silently ignored typos such as `retrry=3` or `notifyy_url=...`.

### Runtime Semantics

- no overlap: if a task is already running when its next schedule tick fires, that tick is skipped
- no catch-up: after restart, dtask schedules from "now" and does not backfill missed runs
- startup emits one `startup_complete` log line with task count and notification sender state
- startup emits one `task_scheduled` log line per task with run mode, user, timezone, cwd, schedule, and next run timestamp

### Compose Variable Interpolation

`dtask` config is plain Compose `environment` values, so Docker Compose interpolation works as usual.

- You can reference shell or `.env` values with `${VAR}`.
- You can provide defaults with `${VAR:-default}`.
- You can fail fast on missing values with `${VAR:?message}`.

This is useful for:

- reusing shared values across multiple options or tasks
- keeping secrets (tokens/webhooks) out of committed compose files

Important:

- interpolation sources are shell environment and `.env`, not other keys in the same Compose `environment` block.

Example `.env`:

```dotenv
DTASK_RUN=compose
DTASK_TZ=Europe/Amsterdam
DAILY_WINDOW=02:00-04:00
DTASK_NOTIFY_URL=mailto://user:pass@example.com
```

Example:

```yaml
services:
  dtask:
    image: ghcr.io/kristianvld/dtask
    volumes:
      - /:/host
    environment:
      run: ${DTASK_RUN:-compose}
      tz: ${DTASK_TZ:-auto}

      update.schedule: ${DAILY_WINDOW}
      update.cmd: docker compose up -d --build --pull always

      cleanup.schedule: ${DAILY_WINDOW}
      cleanup.cmd: docker image prune -af

      # Secret sourced from shell/.env
      notify_url: ${DTASK_NOTIFY_URL:-}
```

## Option Index

Use this section to jump directly to any option.

All options follow one model.

| Option                                    | Scope          | Required | Default             | Purpose                                                                                  |
| ----------------------------------------- | -------------- | -------- | ------------------- | ---------------------------------------------------------------------------------------- |
| [`run`](#run)                             | global or task | no       | `container`         | execution context                                                                        |
| [`user`](#user)                           | global or task | no       | empty               | execution user (`chroot --userspec` for host/compose; numeric `uid[:gid]` for container) |
| [`cwd`](#cwd)                             | global or task | no       | `.`                 | working directory                                                                        |
| [`tz`](#tz)                               | global or task | no       | `auto`              | timezone for scheduling and logging                                                      |
| [`shell`](#shell)                         | global or task | no       | `/bin/bash -lc`     | shell used to execute `task.cmd`                                                         |
| [`timeout`](#timeout)                     | global or task | no       | `0`                 | max runtime per attempt                                                                  |
| [`retry`](#retry)                         | global or task | no       | `0`                 | task retry attempts                                                                      |
| [`backoff`](#backoff)                     | global or task | no       | `exp:10s:5m:2:0.1`  | retry delay strategy                                                                     |
| [`notify`](#notify)                       | global or task | no       | `fail`              | notification policy                                                                      |
| [`notify_url`](#notify_url)               | global or task | no       | empty               | [Apprise](https://github.com/caronc/apprise/wiki) target URL                             |
| [`notify_attach_log`](#notify_attach_log) | global or task | no       | `fail`              | log attachment policy                                                                    |
| [`notify_backoff`](#notify_backoff)       | global or task | no       | `exp:10s:10m:2:0.1` | notification delivery backoff                                                            |
| [`notify_retry`](#notify_retry)           | global or task | no       | `-1`                | notification delivery retries                                                            |
| [`task.schedule`](#taskschedule)          | task only      | yes      | none                | trigger schedule                                                                         |
| [`task.cmd`](#taskcmd)                    | task only      | yes      | none                | command to execute                                                                       |

## Options

Options can be declared in two scopes:

- global scope: `<key>`
- task scope: `<task>.<key>`

Override model:

- task-scoped values (`<task>.<key>`) override global values (`<key>`) for that task.
- all options in this section support both scopes unless explicitly marked task-only.
- `task.schedule` and `task.cmd` are required and task-only.

Example snippets in this section only show the relevant keys. `...` means omitted lines.

### `run`

- Default: `container`
- Allowed values: `container`, `host`, `compose`
- Behavior:
  - selects where the command process is executed and where binaries/files are resolved from.
  - `container`: command runs inside the dtask container context.
  - `host`: command runs in host context via the `/host` mount using chroot semantics, with relative paths resolved from host root (`/`).
  - `compose`: command runs in host context via the `/host` mount, with relative paths resolved from the compose stack directory.
  - when `user` is configured:
    - `host`/`compose`: command runs as that host user via `chroot --userspec`.
    - `container`: command runs with that Linux credential (`uid[:gid]`).
  - stdout/stderr are streamed to the dtask container logs in all modes.
- Validation:
  - `run=host` and `run=compose` require `/:/host` mount.
  - `run=host` and `run=compose` require running as root in the container (chroot).
  - `run=compose` auto-detects the compose stack directory from Docker container metadata label `com.docker.compose.project.working_dir`; unresolved compose directories fail startup.
  - on Docker Desktop (macOS/Windows), `host`/`compose` can behave unexpectedly because they depend on Linux host semantics; validate carefully or prefer `run=container`.
  - invalid enum values fail configuration.

Example:

The following example configures all tasks to run in `compose` mode, so host binaries are used and relative paths are anchored to the compose stack directory. The `lint` task is an explicit exception and runs inside the `dtask` container.

```yaml
# ...
environment:
  # ...
  run: compose
  user: 1000:1000
  # ...
  lint.run: container
```

### `user`

- Default: empty (default process user for the selected run mode)
- Type:
  - `run=host` / `run=compose`: Linux `chroot --userspec` value (for example `1000`, `1000:1000`, `backup`, `backup:backup`)
  - `run=container`: numeric `uid` or `uid:gid` (for example `1000`, `1000:1000`)
- Behavior:
  - `run=host` / `run=compose`: dtask passes this value through as `chroot --userspec=<value>`.
  - `run=container`: dtask executes the command with the provided numeric Linux credential.
- Validation:
  - empty values fail configuration.
  - whitespace in the value fails configuration.
  - for `run=container`, non-numeric users fail startup (must be `uid` or `uid:gid`).

Example:

The following example runs host tasks with named users and a container task as numeric UID/GID.

```yaml
# ...
environment:
  run: host

  backup.user: 1000:1000
  backup.schedule: 6h
  backup.cmd: tar czf /var/backups/app-$(date +%F-%H%M%S).tar.gz /srv/app

  weekly.user: backup
  weekly.schedule: 0 2 * * 0
  weekly.cmd: /usr/local/bin/weekly-maintenance

  lint.run: container
  lint.user: 1000:1000
  lint.schedule: 1h
  lint.cmd: go test ./...
```

### `cwd`

- Default: `.`
- Type: path string
- Behavior:
  - sets the working directory for command execution.
  - absolute path: used directly.
  - relative path resolution depends on `run`:
    - `run=container`: relative to container root (`/`)
    - `run=host`: relative to host root (`/`)
    - `run=compose`: relative to compose stack directory on the host
- Validation:
  - if the resolved working directory does not exist or is not accessible, task execution fails.

Example:

The following example uses one `dtask` service and demonstrates both global and task-level options.
Global defaults are `run: container` and `cwd: /app`, so tasks inherit container execution in a bind-mounted app directory.
`host_cleanup` overrides execution to `run: host` with an absolute host path (`/var/backups`), while `stack_update` overrides to `run: compose` with `cwd: .`, which resolves to the compose stack directory on the host.

```yaml
services:
  dtask:
    image: ghcr.io/kristianvld/dtask
    volumes:
      - /:/host
      - ./app:/app:ro
      - ./backups:/backups
    environment:
      run: container
      cwd: /app

      integrity_check.schedule: 1h
      integrity_check.cmd: ls -la

      app_snapshot.schedule: 6h
      app_snapshot.cwd: /backups
      app_snapshot.cmd: tar czf app-$(date +%F-%H%M%S).tar.gz /app

      host_cleanup.run: host
      host_cleanup.cwd: /var/backups
      host_cleanup.schedule: 1d
      host_cleanup.cmd: find . -type f -mtime +14 -delete

      stack_update.run: compose
      stack_update.cwd: .
      stack_update.schedule: 02:00-04:00
      stack_update.cmd: docker compose pull && docker compose up -d
```

### `tz`

- Default: `auto`
- Type: `auto` or [IANA timezone](https://www.iana.org/time-zones)
- Behavior:
  - controls both schedule interpretation and log timestamp timezone.
  - `auto`: use host timezone when available (`/:/host` mounted), otherwise container timezone.
- Validation:
  - invalid timezone values fail configuration.

Example:

The following example auto-detects timezone for scheduling and log timestamps on all tasks. The `weekly_report` task is pinned to `Europe/Amsterdam`, so its schedule is evaluated in that timezone even if host/container timezone differs.

```yaml
# ...
environment:
  # ...
  tz: auto
  # ...
  weekly_report.tz: Europe/Amsterdam
```

### `shell`

- Default: `/bin/bash -lc`
- Type: shell command string
- Behavior:
  - `task.cmd` is executed through this shell.
  - shell parsing rules apply to the command string (quoting, pipes, operators).
  - in `run=host` or `run=compose`, this shell path must exist on host.
- Validation:
  - invalid/empty shell values fail configuration.
  - if the shell binary is missing in the selected execution context, task execution fails.

Example:

The following example uses Bash as the default command parser, which is useful when tasks rely on Bash features. The `cleanup` task overrides this and uses `/bin/sh -lc` for POSIX shell behavior.

```yaml
# ...
environment:
  # ...
  shell: /bin/bash -lc
  # ...
  cleanup.shell: /bin/sh -lc
```

### `timeout`

- Default: `0`
- Type: [Go duration](https://pkg.go.dev/time#ParseDuration)
- Behavior:
  - max runtime for one attempt (including the initial run and each retry).
  - `0` disables timeout.
  - on timeout, the attempt is marked failed and normal retry/backoff policy applies.
- Validation:
  - invalid duration values fail configuration.
  - negative durations fail configuration.

Example:

The following example enforces a 10-minute runtime limit for all task attempts, so hanging tasks fail and enter normal retry handling. The `full_backup` task overrides this with a 2h30m limit because that workflow is expected to run longer.

```yaml
# ...
environment:
  # ...
  timeout: 10m
  # ...
  full_backup.timeout: 2h30m
```

### `retry`

- Default: `0`
- Type: integer
- Behavior:
  - number of additional attempts after the first failed attempt.
  - effective max attempts are `1 + retry` (except `-1`, which is unbounded).
  - `-1`: infinite retries.
  - `0`: no retries.
- Validation:
  - must be an integer.
  - values below `-1` fail configuration.

Example:

The following example retries failed tasks up to 3 additional times by default (max 4 attempts total). The `network_check` task overrides this and retries indefinitely, which is useful for eventually-available dependencies.

```yaml
# ...
environment:
  # ...
  retry: 3
  # ...
  network_check.retry: -1
```

### `backoff`

- Default: `exp:10s:5m:2:0.1`
- Type: strategy string
- Behavior:
  - controls the delay between a failed attempt and the next retry.
  - applies only when a retry is going to happen.
  - this same format is used by `notify_backoff` for notification delivery retries.

Allowed formats:

- `fixed:<duration>` where `<duration>` is [Go duration](https://pkg.go.dev/time#ParseDuration)
- `exp:<initial>:<max>:<factor>:<jitter>`
  - `initial` and `max` are [Go durations](https://pkg.go.dev/time#ParseDuration)
  - `factor` is the exponential multiplier per retry (for example `2`)
  - `jitter` is random spread from `0` to `jitter` (for example `0.1` for 10%)

Formula:

```text
delay = min(max, initial * factor^attempt * (1 + jitter * random(0, 1)))
```

- Validation:
  - invalid format strings fail configuration.
  - invalid duration parts fail configuration.
  - invalid `factor`/`jitter` values fail configuration.

The following example configures all tasks to use a fixed 20-second retry delay, giving a predictable retry cadence. The `db_migration` task overrides this and uses exponential backoff that starts at 10 seconds, doubles each retry, adds random 0 to 10% jitter, and caps at 5 minutes to reduce repeated load during persistent failures.

Example:

```yaml
# ...
environment:
  # ...
  backoff: fixed:20s
  # ...
  db_migration.backoff: exp:10s:5m:2:0.1
```

### `notify`

- Default: `fail`
- Allowed values: `never`, `fail`, `retry`, `success`, `always`
- Behavior:
  - notifications are emitted **after an attempt finishes** (never before start).
  - `never`: no notifications.
  - `fail`: one notification when the task reaches terminal failure (after retries are exhausted).
  - `retry`: one notification after each failed attempt that will be retried, and one final failure notification if retries are exhausted.
  - `success`: one notification when the task eventually succeeds (first attempt or after retries).
  - `always`: same as `retry` for failures, plus success notification when the task succeeds.
- Validation:
  - invalid enum values fail configuration.

Attempt semantics example (`retry: 3`, command exits non-zero on all attempts):

- `never`: 0 notifications
- `fail`: 1 notification (after final failed attempt)
- `retry`: 4 notifications (3 retry failures + 1 terminal failure)
- `success`: 0 notifications (no successful attempt occurred)
- `always`: 4 notifications (same as `retry` in this always-failing scenario)

Example:

The following example sends notifications only when a task reaches terminal failure by default. The `network_check` task overrides this to notify only on successful runs, while `backup` notifies on all outcomes (retry/fail/success).

```yaml
# ...
environment:
  # ...
  notify: fail
  # ...
  network_check.notify: success
  backup.notify: always
```

### `notify_url`

- Default: _empty_
- Type: [Apprise URL](https://github.com/caronc/apprise/wiki) string
- Behavior:
  - destination URL for notifications emitted by the `notify` policy.
  - if empty, notifications are skipped regardless of `notify` value.
  - if set, it must be a valid Apprise URL.
  - value is passed directly to Apprise; `dtask` does not rewrite or normalize it.
- Validation:
  - invalid URLs fail configuration.
- Notes:
  - this can be set globally (`notify_url`) or per task (`task.notify_url`).
  - dtask uses Python `apprise` CLI inside the container to deliver notifications.

Example:

The following example configures one default notification destination for all tasks. The `backup` task overrides `notify_url`, so backup notifications are routed to a separate destination.

```yaml
# ...
environment:
  # ...
  notify_url: mailto://user:pass@example.com
  # ...
  backup.notify_url: discord://webhook_id/webhook_token
```

The following example routes notifications to an ops email channel by default. The `weekly_full_backup` task overrides the destination and routes only that task to Discord-compatible webhook syntax supported by Apprise.

```yaml
# ...
environment:
  # ...
  # route specific tasks to different channels/services
  notify_url: mailto://ops@example.com
  # ...
  weekly_full_backup.notify_url: discord://webhook_id/webhook_token
```

### `notify_attach_log`

- Default: `fail`
- Allowed values: `never`, `fail`, `always`
- Behavior:
  - controls when stdout/stderr log is attached to an emitted notification.
  - this option only affects emitted notifications; if `notify=never` nothing is sent.
  - attachment support is service-dependent in Apprise backend providers.
- Validation:
  - invalid enum values fail configuration.
  - if this option is `fail` or `always`, configured `notify_url` providers must support attachments or startup fails.

Example:

The following example attaches logs only for failed runs by default, which keeps successful notifications lighter. The `backup` task overrides this and always attaches logs for audit/debug visibility.

```yaml
# ...
environment:
  # ...
  notify_attach_log: fail
  # ...
  backup.notify_attach_log: always
```

### `notify_backoff`

- Default: `exp:10s:10m:2:0.1`
- Type: strategy string
- Behavior:
  - retry delay strategy for failed notification deliveries.
  - uses the same format as `backoff`, but applies to notification delivery retries.

Allowed formats:

- `fixed:<duration>` where `<duration>` is [Go duration](https://pkg.go.dev/time#ParseDuration)
- `exp:<initial>:<max>:<factor>:<jitter>`
- Validation:
  - invalid format strings fail configuration.
  - invalid duration parts fail configuration.
  - invalid `factor`/`jitter` values fail configuration.

The following example configures notification delivery retries for all tasks to use a fixed 15-second delay. The `backup` task overrides this and uses exponential delays that start at 10 seconds, double per retry, include random 0 to 10% jitter, and cap at 10 minutes.

Example:

```yaml
# ...
environment:
  # ...
  notify_backoff: fixed:15s
  # ...
  backup.notify_backoff: exp:10s:10m:2:0.1
```

### `notify_retry`

- Default: `-1`
- Type: integer
- Behavior:
  - retry count for failed notification deliveries.
  - independent from `retry` (task execution retries).
  - `-1`: infinite retries.
  - `0`: no retries.
- Validation:
  - must be an integer.
  - values below `-1` fail configuration.

Example:

The following example retries failed notification deliveries indefinitely by default, so transient outages do not drop alerts. The `backup` task overrides this and limits delivery retries to 3.

```yaml
# ...
environment:
  # ...
  notify_retry: -1
  # ...
  backup.notify_retry: 3
```

The following two options are required on every task.

### `task.schedule`

- Default: none (required)
- Type: schedule string
- Behavior:
  - defines when task is triggered.
  - interpreted using `tz`.
  - each task must define its own `schedule`; there is no global `schedule`.
  - if a previous run is still active, the next schedule tick is skipped (no overlap).
  - supported formats:
    - `HH:MM` daily fixed time
    - `HH:MM-HH:MM` daily random window (one run per day in the window)
    - duration interval (for example `30s`, `1h`, `1h30m`) using [Go duration](https://pkg.go.dev/time#ParseDuration)
    - 5-field cron expression (for example `0 2 * * 0`)
- Validation:
  - required per task.
  - invalid schedule strings fail configuration.

Example:

The following example demonstrates three schedule modes together: a daily random window (`stack_update`), a fixed interval (`hourly_backup`), and a weekly cron schedule (`sunday_full_backup`).

```yaml
# ...
environment:
  # ...
  stack_update.schedule: 02:00-04:00
  hourly_backup.schedule: 1h
  sunday_full_backup.schedule: 0 2 * * 0
```

### `task.cmd`

- Default: none (required)
- Type: command string
- Behavior:
  - executed through configured `shell`.
  - evaluated in context defined by `run` and `cwd`.
  - should contain the full command to execute for that task.
  - exit code determines task result (`0` success, non-zero failure/retry path).
- Validation:
  - required per task.
  - empty command values fail configuration.

Example:

The following example shows `task.cmd` in realistic contexts.
`integrity_check` uses the default `run: container` with a mounted `/app` directory.
`stack_update` explicitly switches to `run: compose` for Docker Compose commands in the stack directory, and `host_script` switches to `run: host` to execute a host-installed Bun script from an absolute host path.

```yaml
services:
  dtask:
    image: ghcr.io/kristianvld/dtask
    volumes:
      - /:/host
      - ./app:/app:ro
    environment:
      run: container
      cwd: /app

      integrity_check.schedule: 1h
      integrity_check.cmd: ls -la

      stack_update.run: compose
      stack_update.schedule: 02:00-04:00
      stack_update.cmd: docker compose pull && docker compose up -d

      host_script.run: host
      host_script.cwd: /srv/myapp
      host_script.schedule: 1h
      host_script.cmd: bun run scripts/cleanup.ts
```

## Additional Examples

### Host Stack Update at Night

```yaml
services:
  dtask:
    image: ghcr.io/kristianvld/dtask
    restart: unless-stopped
    volumes:
      - /:/host
    environment:
      run: compose
      stack_update.schedule: 02:00-04:00
      stack_update.cmd: docker compose up -d --build --pull always
```

### Container Backup Every Hour

```yaml
services:
  dtask:
    image: ghcr.io/kristianvld/dtask
    restart: unless-stopped
    volumes:
      - ./backups:/backups
      - ./app:/app:ro
    environment:
      notify_url: mailto://user:pass@example.com

      hourly_backup.schedule: 1h
      hourly_backup.timeout: 10m
      hourly_backup.retry: 1
      hourly_backup.notify_attach_log: fail
      hourly_backup.cmd: tar czf /backups/incr-$(date +%F-%H%M%S).tar.gz --listed-incremental=/backups/backup.snar /app/data

      weekly_full_backup.schedule: 0 2 * * 0
      weekly_full_backup.timeout: 30m
      weekly_full_backup.retry: 3
      weekly_full_backup.notify: always
      weekly_full_backup.notify_attach_log: fail
      weekly_full_backup.cmd: tar czf /backups/full-$(date +%F-%H%M%S).tar.gz --listed-incremental=/backups/backup.snar.new /app/data && mv /backups/backup.snar.new /backups/backup.snar
```
