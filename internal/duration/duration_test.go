package duration

import (
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{name: "go duration", input: "90s", want: 90 * time.Second},
		{name: "day suffix", input: "1d", want: 24 * time.Hour},
		{name: "mixed", input: "1d2h30m", want: 26*time.Hour + 30*time.Minute},
		{name: "invalid", input: "abc", wantErr: true},
		{name: "partial invalid", input: "1dx", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Parse(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}
