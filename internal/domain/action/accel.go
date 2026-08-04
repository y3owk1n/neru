package action

import (
	"math"
	"strconv"
	"strings"
)

// ScaleDeltaFlags multiplies the --dx/--dy values in actionStr.
//
// Tokenizing with Fields rather than SplitArgs because this rejoins: SplitArgs
// consumes the quotes it parses. Only move_mouse_relative is ever scaled and its
// flags are integers, so there is nothing to quote.
func ScaleDeltaFlags(actionStr string, multiplier float64) string {
	if multiplier == 1 {
		return actionStr
	}

	tokens := strings.Fields(actionStr)

	for idx, token := range tokens {
		flag, value, joined := strings.Cut(token, "=")
		if flag != "--dx" && flag != "--dy" {
			continue
		}

		target := idx

		if !joined {
			if idx+1 >= len(tokens) {
				continue
			}

			target, value = idx+1, tokens[idx+1]
		}

		parsed, err := strconv.Atoi(value)
		if err != nil {
			continue
		}

		scaled := strconv.Itoa(clampToInt(math.Round(float64(parsed) * multiplier)))
		if joined {
			scaled = flag + "=" + scaled
		}

		tokens[target] = scaled
	}

	return strings.Join(tokens, " ")
}

// Out-of-range float to int is implementation-defined in Go, so saturate.
func clampToInt(value float64) int {
	if value >= math.MaxInt {
		return math.MaxInt
	}

	if value <= math.MinInt {
		return math.MinInt
	}

	return int(value)
}

// ScaleAllDeltaFlags scales into a new slice, so ticks never compound onto the
// original binding.
func ScaleAllDeltaFlags(actions []string, multiplier float64) []string {
	if multiplier == 1 {
		return actions
	}

	scaled := make([]string, len(actions))
	for idx, actionStr := range actions {
		scaled[idx] = ScaleDeltaFlags(actionStr, multiplier)
	}

	return scaled
}
