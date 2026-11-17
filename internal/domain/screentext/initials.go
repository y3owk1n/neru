package screentext

import "slices"

// WordStarts marks each letter or digit of folded, text as Fold spells it,
// that starts a word. That is the first one after a space or symbol, a
// capital after a small letter, as the A of saveAs, and the last capital of
// a run that a small letter follows, as the P of XMLParser. It is also a
// digit after a letter or a letter after a digit, as the 2 of File2. Fold
// spells each Chinese, Japanese or Korean syllable with a capital, so the Z
// of ShenZhen starts a word too.
func WordStarts(folded string) []bool {
	starts := make([]bool, len(folded))

	for index := range len(folded) {
		char := folded[index]
		if !isAlphanumeric(char) {
			continue
		}

		if index == 0 {
			starts[index] = true

			continue
		}

		previous := folded[index-1]
		nextIsLower := index+1 < len(folded) && isLower(folded[index+1])

		switch {
		case !isAlphanumeric(previous):
			starts[index] = true
		case isUpper(char):
			starts[index] = isLower(previous) || isDigit(previous) ||
				isUpper(previous) && nextIsLower
		default:
			starts[index] = isDigit(char) != isDigit(previous)
		}
	}

	return starts
}

// Initials is where query's letters can land on text from the word start at,
// each right after the letter before it or on the next word start on the
// line, so sas finds Save As and sz finds ShenZhen. It returns each byte the
// last letter can land on, in order, or none when query does not land. text
// is as Fold spells it, lowered when query should ignore case, and starts is
// WordStarts of the same spelling.
func Initials(text string, starts []bool, query string, at int) []int {
	if query == "" || at >= len(text) || !starts[at] || text[at] != query[0] {
		return nil
	}

	landed := []int{at}

	for index := 1; index < len(query) && len(landed) > 0; index++ {
		next := query[index]

		var reached []int

		for _, from := range landed {
			if from+1 < len(text) && text[from+1] == next {
				reached = append(reached, from+1)
			}

			if to := NextWordStart(text, starts, from); to > from+1 && text[to] == next {
				reached = append(reached, to)
			}
		}

		slices.Sort(reached)
		landed = slices.Compact(reached)
	}

	return landed
}

// NextWordStart is the first word start after from on its line, or -1.
func NextWordStart(text string, starts []bool, from int) int {
	for to := from + 1; to < len(text) && text[to] != '\n'; to++ {
		if starts[to] {
			return to
		}
	}

	return -1
}

func isLower(char byte) bool { return char >= 'a' && char <= 'z' }

func isUpper(char byte) bool { return char >= 'A' && char <= 'Z' }

func isDigit(char byte) bool { return char >= '0' && char <= '9' }

func isAlphanumeric(char byte) bool { return isLower(char) || isUpper(char) || isDigit(char) }
