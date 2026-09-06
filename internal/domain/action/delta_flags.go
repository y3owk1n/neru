package action

import "strconv"

// ParseDeltaFlags reads the --dx/--dy values from the tokens of a
// move_mouse_relative action, split by the same quote-aware grammar the
// executor uses. The bool is false when either flag is missing or not an
// integer.
func ParseDeltaFlags(tokens []string) (int, int, bool) {
	found := map[string]int{}

	for idx, token := range tokens {
		flag, value, joined := cutFlag(token)
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

// cutFlag splits "--flag=value" into its halves; a bare token is returned
// whole with joined == false.
func cutFlag(token string) (string, string, bool) {
	for idx := range len(token) {
		if token[idx] == '=' {
			return token[:idx], token[idx+1:], true
		}
	}

	return token, "", false
}
