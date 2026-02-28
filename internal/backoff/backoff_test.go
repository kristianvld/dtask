package backoff

import (
	"math/rand"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{name: "fixed", in: "fixed:10s"},
		{name: "exp", in: "exp:10s:5m:2:0.1"},
		{name: "exp with day", in: "exp:1d:2d:2:0.1"},
		{name: "bad", in: "wat:10s", wantErr: true},
		{name: "bad fixed", in: "fixed:-1s", wantErr: true},
		{name: "bad exp factor", in: "exp:10s:5m:0.5:0.1", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := Parse(tt.in)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestDelay(t *testing.T) {
	t.Parallel()

	s, err := Parse("exp:10s:1m:2:0")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	r := rand.New(rand.NewSource(1))
	if got := s.Delay(1, r); got != 10*time.Second {
		t.Fatalf("delay(1)=%v", got)
	}
	if got := s.Delay(2, r); got != 20*time.Second {
		t.Fatalf("delay(2)=%v", got)
	}
	if got := s.Delay(5, r); got != time.Minute {
		t.Fatalf("delay(5)=%v", got)
	}
}
