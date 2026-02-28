package duration

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var tokenRE = regexp.MustCompile(`(?i)([0-9]+(?:\.[0-9]+)?)(ns|us|µs|ms|s|m|h|d)`) //nolint:lll

// Parse supports time.ParseDuration formats and extends them with `d` (24h days).
func Parse(input string) (time.Duration, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return 0, fmt.Errorf("duration is empty")
	}

	if d, err := time.ParseDuration(input); err == nil {
		return d, nil
	}

	tokens := tokenRE.FindAllStringSubmatchIndex(input, -1)
	if len(tokens) == 0 {
		return 0, fmt.Errorf("invalid duration %q", input)
	}

	consumed := 0
	total := 0.0
	for _, idx := range tokens {
		if idx[0] != consumed {
			return 0, fmt.Errorf("invalid duration %q", input)
		}

		numRaw := input[idx[2]:idx[3]]
		unitRaw := strings.ToLower(input[idx[4]:idx[5]])
		num, err := strconv.ParseFloat(numRaw, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q: %w", input, err)
		}

		switch unitRaw {
		case "ns":
			total += num
		case "us", "µs":
			total += num * float64(time.Microsecond)
		case "ms":
			total += num * float64(time.Millisecond)
		case "s":
			total += num * float64(time.Second)
		case "m":
			total += num * float64(time.Minute)
		case "h":
			total += num * float64(time.Hour)
		case "d":
			total += num * float64(24*time.Hour)
		default:
			return 0, fmt.Errorf("invalid duration unit %q", unitRaw)
		}

		consumed = idx[1]
	}

	if consumed != len(input) {
		return 0, fmt.Errorf("invalid duration %q", input)
	}

	if total > float64(int64(^uint64(0)>>1)) {
		return 0, fmt.Errorf("duration out of range")
	}

	return time.Duration(total), nil
}
