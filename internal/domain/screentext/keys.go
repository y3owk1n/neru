package screentext

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/y3owk1n/fude/vim"
)

// The vim keys the taps have more than one name for.
const (
	keyEscape vim.Key = "<Esc>"
	keyReturn vim.Key = "<CR>"
)

// namedKeys maps the keys the event taps name to vim notation.
var namedKeys = map[string]vim.Key{
	"escape": keyEscape, "esc": keyEscape, "\x1b": keyEscape,
	"return": keyReturn, "enter": keyReturn, "\r": keyReturn,
	"delete": "<BS>", "backspace": "<BS>",
	"space": " ", "tab": "\t",
	"up": "<Up>", "down": "<Down>", "left": "<Left>", "right": "<Right>",
	"home": "<Home>", "end": "<End>", "pageup": "<PageUp>", "pagedown": "<PageDown>",
}

// shifted is what Shift makes of a digit or symbol on a US layout. The taps
// name a shifted symbol by its unshifted key, as "Shift+4" for "$".
var shifted = map[string]string{
	"1": "!", "2": "@", "3": "#", "4": "$", "5": "%", "6": "^", "7": "&", "8": "*",
	"9": "(", "0": ")", "-": "_", "=": "+", "[": "{", "]": "}", "\\": "|",
	";": ":", "'": "\"", ",": "<", ".": ">", "/": "?", "`": "~",
}

// Key turns a key as the event taps name it into vim notation, or reports
// false for a key vim has no use for, such as a Cmd chord.
func Key(key string) (vim.Key, bool) {
	if k, ok := namedKeys[strings.ToLower(key)]; ok {
		return k, true
	}

	if base, ok := cutModifier(key, "Ctrl+"); ok {
		r, size := utf8.DecodeRuneInString(base)
		if size == len(base) && unicode.IsLetter(r) {
			return vim.Key("<C-" + string(unicode.ToLower(r)) + ">"), true
		}

		if base == "[" {
			return keyEscape, true
		}

		return "", false
	}

	if base, ok := cutModifier(key, "Shift+"); ok {
		switch lower := strings.ToLower(base); {
		case lower == "tab":
			return "<S-Tab>", true
		case lower == "space":
			return " ", true
		case shifted[base] != "":
			return vim.Key(shifted[base]), true
		}

		r, size := utf8.DecodeRuneInString(base)
		if size == len(base) && unicode.IsLetter(r) {
			return vim.Key(string(unicode.ToUpper(r))), true
		}

		return "", false
	}

	if utf8.RuneCountInString(key) == 1 {
		return vim.Key(key), true
	}

	return "", false
}

// cutModifier returns key without its leading modifier, matched in any case,
// since Windows names a chord "shift+v" where the other taps say "Shift+v".
func cutModifier(key, modifier string) (string, bool) {
	if len(key) < len(modifier) || !strings.EqualFold(key[:len(modifier)], modifier) {
		return key, false
	}

	return key[len(modifier):], true
}

// unshifted is the key Shift turns into each symbol on a US layout, the
// inverse of shifted.
var unshifted = func() map[rune]string {
	out := make(map[rune]string, len(shifted))
	for key, symbol := range shifted {
		r, _ := utf8.DecodeRuneInString(symbol)
		out[r] = key
	}

	return out
}()

// FeedKeys spells text as the keys that type it, in the names the key feed
// takes, or reports false when a rune has no key on a US layout.
func FeedKeys(text string) ([]string, bool) {
	keys := make([]string, 0, len(text))

	for _, char := range text {
		switch {
		case char == '\n':
			keys = append(keys, "Return")
		case char == '\t':
			keys = append(keys, "Tab")
		case char == ' ':
			keys = append(keys, "Space")
		case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z', char >= '0' && char <= '9':
			keys = append(keys, string(char))
		case unshifted[char] != "":
			keys = append(keys, "Shift+"+unshifted[char])
		case char < utf8.RuneSelf && strings.ContainsRune("-=[]\\;',./`", char):
			keys = append(keys, string(char))
		default:
			return nil, false
		}
	}

	return keys, true
}
