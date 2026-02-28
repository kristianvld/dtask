package schedule

import (
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
	"time"

	"github.com/kristianvld/dtask/internal/duration"
)

const (
	KindInterval    = "interval"
	KindDailyTime   = "daily_time"
	KindDailyWindow = "daily_window"
	KindCron        = "cron"
)

type Spec struct {
	Raw string

	Kind string

	Interval time.Duration

	DailyHour   int
	DailyMinute int

	WindowStartMinute int
	WindowEndMinute   int
	TaskName          string

	Cron CronSpec
}

func Parse(raw string, taskName string) (Spec, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Spec{}, fmt.Errorf("schedule is empty")
	}

	if strings.Count(raw, " ") >= 4 {
		cron, err := parseCron(raw)
		if err != nil {
			return Spec{}, err
		}
		return Spec{Raw: raw, Kind: KindCron, Cron: cron}, nil
	}

	if strings.Contains(raw, "-") {
		parts := strings.Split(raw, "-")
		if len(parts) != 2 {
			return Spec{}, fmt.Errorf("invalid schedule %q", raw)
		}
		sh, sm, err := parseHHMM(parts[0])
		if err != nil {
			return Spec{}, fmt.Errorf("invalid start time: %w", err)
		}
		eh, em, err := parseHHMM(parts[1])
		if err != nil {
			return Spec{}, fmt.Errorf("invalid end time: %w", err)
		}
		start := sh*60 + sm
		end := eh*60 + em
		if end <= start {
			return Spec{}, fmt.Errorf("window end must be after start")
		}
		return Spec{
			Raw:               raw,
			Kind:              KindDailyWindow,
			WindowStartMinute: start,
			WindowEndMinute:   end,
			TaskName:          taskName,
		}, nil
	}

	if h, m, err := parseHHMM(raw); err == nil {
		return Spec{Raw: raw, Kind: KindDailyTime, DailyHour: h, DailyMinute: m}, nil
	}

	d, err := duration.Parse(raw)
	if err != nil {
		return Spec{}, fmt.Errorf("invalid schedule %q", raw)
	}
	if d <= 0 {
		return Spec{}, fmt.Errorf("schedule interval must be > 0")
	}
	return Spec{Raw: raw, Kind: KindInterval, Interval: d}, nil
}

func (s Spec) Next(after time.Time, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.Local
	}
	after = after.In(loc)

	switch s.Kind {
	case KindInterval:
		return after.Add(s.Interval)
	case KindDailyTime:
		next := time.Date(after.Year(), after.Month(), after.Day(), s.DailyHour, s.DailyMinute, 0, 0, loc)
		if !next.After(after) {
			next = next.Add(24 * time.Hour)
		}
		return next
	case KindDailyWindow:
		for i := 0; i < 3660; i++ {
			day := time.Date(after.Year(), after.Month(), after.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, i)
			target := s.windowTarget(day)
			if target.After(after) {
				return target
			}
		}
		return after.Add(24 * time.Hour)
	case KindCron:
		return s.Cron.Next(after)
	default:
		return after.Add(24 * time.Hour)
	}
}

func (s Spec) windowTarget(day time.Time) time.Time {
	windowMinutes := s.WindowEndMinute - s.WindowStartMinute
	if windowMinutes <= 1 {
		m := s.WindowStartMinute
		return time.Date(day.Year(), day.Month(), day.Day(), m/60, m%60, 0, 0, day.Location())
	}

	h := fnv.New64a()
	_, _ = h.Write([]byte(s.TaskName))
	_, _ = h.Write([]byte(day.Format("2006-01-02")))
	offset := int(h.Sum64() % uint64(windowMinutes))
	m := s.WindowStartMinute + offset
	return time.Date(day.Year(), day.Month(), day.Day(), m/60, m%60, 0, 0, day.Location())
}

func parseHHMM(s string) (int, int, error) {
	parts := strings.Split(strings.TrimSpace(s), ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("expected HH:MM")
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid hour")
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid minute")
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, fmt.Errorf("time out of range")
	}
	return h, m, nil
}

type CronSpec struct {
	Raw     string
	Minute  cronField
	Hour    cronField
	Day     cronField
	Month   cronField
	Weekday cronField
}

type cronField struct {
	Any     bool
	Allowed map[int]struct{}
}

func (f cronField) matches(v int) bool {
	if f.Any {
		return true
	}
	_, ok := f.Allowed[v]
	return ok
}

func parseCron(raw string) (CronSpec, error) {
	fields := strings.Fields(raw)
	if len(fields) != 5 {
		return CronSpec{}, fmt.Errorf("invalid cron %q", raw)
	}

	minute, err := parseCronField(fields[0], 0, 59, false)
	if err != nil {
		return CronSpec{}, fmt.Errorf("invalid cron minute: %w", err)
	}
	hour, err := parseCronField(fields[1], 0, 23, false)
	if err != nil {
		return CronSpec{}, fmt.Errorf("invalid cron hour: %w", err)
	}
	day, err := parseCronField(fields[2], 1, 31, false)
	if err != nil {
		return CronSpec{}, fmt.Errorf("invalid cron day: %w", err)
	}
	month, err := parseCronField(fields[3], 1, 12, false)
	if err != nil {
		return CronSpec{}, fmt.Errorf("invalid cron month: %w", err)
	}
	weekday, err := parseCronField(fields[4], 0, 7, true)
	if err != nil {
		return CronSpec{}, fmt.Errorf("invalid cron weekday: %w", err)
	}

	return CronSpec{
		Raw:     raw,
		Minute:  minute,
		Hour:    hour,
		Day:     day,
		Month:   month,
		Weekday: weekday,
	}, nil
}

func parseCronField(raw string, min, max int, sunday7 bool) (cronField, error) {
	raw = strings.TrimSpace(raw)
	if raw == "*" {
		return cronField{Any: true}, nil
	}

	field := cronField{Allowed: map[int]struct{}{}}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return cronField{}, fmt.Errorf("empty field segment")
		}

		step := 1
		rangePart := part
		if strings.Contains(part, "/") {
			bits := strings.Split(part, "/")
			if len(bits) != 2 {
				return cronField{}, fmt.Errorf("invalid step syntax")
			}
			rangePart = bits[0]
			v, err := strconv.Atoi(bits[1])
			if err != nil || v <= 0 {
				return cronField{}, fmt.Errorf("invalid step value")
			}
			step = v
		}

		start := min
		end := max

		switch {
		case rangePart == "*":
			// keep defaults
		case strings.Contains(rangePart, "-"):
			bounds := strings.Split(rangePart, "-")
			if len(bounds) != 2 {
				return cronField{}, fmt.Errorf("invalid range")
			}
			s, err := strconv.Atoi(bounds[0])
			if err != nil {
				return cronField{}, fmt.Errorf("invalid range start")
			}
			e, err := strconv.Atoi(bounds[1])
			if err != nil {
				return cronField{}, fmt.Errorf("invalid range end")
			}
			start = s
			end = e
		default:
			v, err := strconv.Atoi(rangePart)
			if err != nil {
				return cronField{}, fmt.Errorf("invalid value")
			}
			start = v
			end = v
		}

		if start < min || end > max || end < start {
			return cronField{}, fmt.Errorf("value out of bounds")
		}
		for i := start; i <= end; i += step {
			value := i
			if sunday7 && value == 7 {
				value = 0
			}
			field.Allowed[value] = struct{}{}
		}
	}

	return field, nil
}

func (c CronSpec) Next(after time.Time) time.Time {
	loc := after.Location()
	t := after.Truncate(time.Minute).Add(time.Minute)
	for i := 0; i < 525600*5; i++ {
		if c.matches(t.In(loc)) {
			return t.In(loc)
		}
		t = t.Add(time.Minute)
	}
	return after.Add(24 * time.Hour)
}

func (c CronSpec) matches(t time.Time) bool {
	if !c.Minute.matches(t.Minute()) || !c.Hour.matches(t.Hour()) || !c.Month.matches(int(t.Month())) {
		return false
	}

	domMatch := c.Day.matches(t.Day())
	dowMatch := c.Weekday.matches(int(t.Weekday()))

	if c.Day.Any && c.Weekday.Any {
		return true
	}
	if c.Day.Any {
		return dowMatch
	}
	if c.Weekday.Any {
		return domMatch
	}
	return domMatch || dowMatch
}
