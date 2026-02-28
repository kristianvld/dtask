package schedule

import (
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "daily time", in: "02:00", want: KindDailyTime},
		{name: "window", in: "02:00-04:00", want: KindDailyWindow},
		{name: "interval", in: "1h30m", want: KindInterval},
		{name: "interval day", in: "1d", want: KindInterval},
		{name: "cron", in: "0 2 * * 0", want: KindCron},
		{name: "bad", in: "wat", wantErr: true},
		{name: "bad window", in: "04:00-02:00", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s, err := Parse(tt.in, "task")
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if s.Kind != tt.want {
				t.Fatalf("kind=%s want=%s", s.Kind, tt.want)
			}
		})
	}
}

func TestNextDailyTime(t *testing.T) {
	t.Parallel()
	loc := time.FixedZone("X", 0)
	s, _ := Parse("02:00", "x")
	after := time.Date(2026, 1, 1, 1, 0, 0, 0, loc)
	if got := s.Next(after, loc); !got.Equal(time.Date(2026, 1, 1, 2, 0, 0, 0, loc)) {
		t.Fatalf("unexpected next: %v", got)
	}
	after = time.Date(2026, 1, 1, 2, 0, 0, 0, loc)
	if got := s.Next(after, loc); !got.Equal(time.Date(2026, 1, 2, 2, 0, 0, 0, loc)) {
		t.Fatalf("unexpected next: %v", got)
	}
}

func TestNextWindowStablePerDay(t *testing.T) {
	t.Parallel()
	loc := time.FixedZone("X", 0)
	s, _ := Parse("02:00-04:00", "update_task")
	a := time.Date(2026, 1, 1, 1, 0, 0, 0, loc)
	b := time.Date(2026, 1, 1, 1, 10, 0, 0, loc)
	na := s.Next(a, loc)
	nb := s.Next(b, loc)
	if !na.Equal(nb) {
		t.Fatalf("expected stable daily time, got %v and %v", na, nb)
	}
}

func TestCronNext(t *testing.T) {
	t.Parallel()
	loc := time.UTC
	s, err := Parse("0 2 * * 0", "x")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	after := time.Date(2026, 1, 1, 1, 0, 0, 0, loc)
	next := s.Next(after, loc)
	if next.Weekday() != time.Sunday || next.Hour() != 2 || next.Minute() != 0 {
		t.Fatalf("unexpected next cron time: %v", next)
	}
}
