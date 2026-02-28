package backoff

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/kristianvld/dtask/internal/duration"
)

const (
	KindFixed = "fixed"
	KindExp   = "exp"
)

type Strategy struct {
	Kind    string
	Fixed   time.Duration
	Initial time.Duration
	Max     time.Duration
	Factor  float64
	Jitter  float64
}

func Parse(raw string) (Strategy, error) {
	parts := strings.Split(raw, ":")
	if len(parts) < 2 {
		return Strategy{}, fmt.Errorf("invalid backoff %q", raw)
	}

	switch parts[0] {
	case KindFixed:
		if len(parts) != 2 {
			return Strategy{}, fmt.Errorf("invalid fixed backoff %q", raw)
		}
		d, err := duration.Parse(parts[1])
		if err != nil {
			return Strategy{}, fmt.Errorf("invalid fixed duration: %w", err)
		}
		if d <= 0 {
			return Strategy{}, fmt.Errorf("fixed duration must be > 0")
		}
		return Strategy{Kind: KindFixed, Fixed: d}, nil
	case KindExp:
		if len(parts) != 5 {
			return Strategy{}, fmt.Errorf("invalid exp backoff %q", raw)
		}
		initial, err := duration.Parse(parts[1])
		if err != nil {
			return Strategy{}, fmt.Errorf("invalid exp initial duration: %w", err)
		}
		max, err := duration.Parse(parts[2])
		if err != nil {
			return Strategy{}, fmt.Errorf("invalid exp max duration: %w", err)
		}
		factor, err := strconv.ParseFloat(parts[3], 64)
		if err != nil {
			return Strategy{}, fmt.Errorf("invalid exp factor: %w", err)
		}
		jitter, err := strconv.ParseFloat(parts[4], 64)
		if err != nil {
			return Strategy{}, fmt.Errorf("invalid exp jitter: %w", err)
		}
		if initial <= 0 {
			return Strategy{}, fmt.Errorf("exp initial must be > 0")
		}
		if max <= 0 {
			return Strategy{}, fmt.Errorf("exp max must be > 0")
		}
		if max < initial {
			return Strategy{}, fmt.Errorf("exp max must be >= initial")
		}
		if factor < 1 {
			return Strategy{}, fmt.Errorf("exp factor must be >= 1")
		}
		if jitter < 0 || jitter > 1 {
			return Strategy{}, fmt.Errorf("exp jitter must be between 0 and 1")
		}
		return Strategy{
			Kind:    KindExp,
			Initial: initial,
			Max:     max,
			Factor:  factor,
			Jitter:  jitter,
		}, nil
	default:
		return Strategy{}, fmt.Errorf("unknown backoff kind %q", parts[0])
	}
}

func (s Strategy) Delay(retryAttempt int, r *rand.Rand) time.Duration {
	if retryAttempt < 1 {
		retryAttempt = 1
	}

	switch s.Kind {
	case KindFixed:
		return s.Fixed
	case KindExp:
		if r == nil {
			r = rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec
		}
		growth := math.Pow(s.Factor, float64(retryAttempt-1))
		base := float64(s.Initial) * growth
		if base > float64(s.Max) {
			base = float64(s.Max)
		}
		multiplier := 1.0 + (s.Jitter * r.Float64())
		d := time.Duration(base * multiplier)
		if d > s.Max {
			return s.Max
		}
		if d < 0 {
			return 0
		}
		return d
	default:
		return 0
	}
}
