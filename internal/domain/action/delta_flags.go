package action

import (
	"strconv"
	"strings"
)

// ParseDeltaFlags reads the --dx/--dy values from a move_mouse_relative
// action string. The bool is false when either flag is missing or not an
// integer.
func ParseDeltaFlags(actionStr string) (int, int, bool) {
	tokens := strings.Fields(actionStr)
	found := map[string]int{}

	for idx, token := range tokens {
		flag, value, joined := strings.Cut(token, "=")
		if flag != "--dx" && flag != "--dy" {
			continue
		}

		if !joined {
			if idx+1 >= len(tokens) {
				return 0, 0, false
			}

			value = tokens[idx+1]
		}

		parsed, err := strconv.Atoi(value)
		if err != nil {
			return 0, 0, false
		}

		found[flag] = parsed
	}

	deltaX, hasX := found["--dx"]

	deltaY, hasY := found["--dy"]
	if !hasX || !hasY {
		return 0, 0, false
	}

	return deltaX, deltaY, true
}
