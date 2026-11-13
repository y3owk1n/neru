package screentext

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/y3owk1n/fude/vim"
)

// The commands vim mode adds to vim's own, each run by the key <Plug>(name),
// which a mapping names to put the command on other keys.
const (
	PlugJump            = "jump"
	PlugReadScreen      = "read-screen"
	PlugLeftClick       = "left-click"
	PlugRightClick      = "right-click"
	PlugMiddleClick     = "middle-click"
	PlugMoveMouse       = "move-mouse"
	PlugScrollLineDown  = "scroll-line-down"
	PlugScrollLineUp    = "scroll-line-up"
	PlugScrollHalfDown  = "scroll-half-down"
	PlugScrollHalfUp    = "scroll-half-up"
	PlugScrollPageDown  = "scroll-page-down"
	PlugScrollPageUp    = "scroll-page-up"
	PlugScrollLeft      = "scroll-left"
	PlugScrollRight     = "scroll-right"
	PlugScrollHalfLeft  = "scroll-half-left"
	PlugScrollHalfRight = "scroll-half-right"
	PlugCenterLine      = "center-line"
	PlugNextIcon        = "next-icon"
	PlugPrevIcon        = "prev-icon"
)

// Plugs are the names of the commands vim mode adds, the ones a mapping may
// run.
var Plugs = []string{
	PlugJump, PlugReadScreen,
	PlugLeftClick, PlugRightClick, PlugMiddleClick, PlugMoveMouse,
	PlugScrollLineDown, PlugScrollLineUp, PlugScrollHalfDown, PlugScrollHalfUp,
	PlugScrollPageDown, PlugScrollPageUp,
	PlugScrollLeft, PlugScrollRight, PlugScrollHalfLeft, PlugScrollHalfRight,
	PlugCenterLine, PlugNextIcon, PlugPrevIcon,
}

// Plug is the key that runs the command name, as <Plug>(jump).
func Plug(name string) string { return "<Plug>(" + name + ")" }

// OperatorDrag is the operator vim mode adds, which drags across the text on
// screen rather than changing it.
const OperatorDrag = "gs"

// Keymap is the mappings of normal and visual mode, from the keys typed to
// the keys they run, both in vim notation. The keys typed are written as
// vim.Notation writes them, so one sequence has one spelling.
type Keymap map[vim.Mode]map[string]string

// DefaultKeymap is what vim mode maps before the user's mappings: the keys of
// the commands it adds, and Return in visual mode to drag.
func DefaultKeymap() Keymap {
	return Keymap{
		vim.Normal: {
			"s":     Plug(PlugJump),
			"<C-l>": Plug(PlugReadScreen),
			"<CR>":  Plug(PlugLeftClick),
			"gc":    Plug(PlugLeftClick),
			"gr":    Plug(PlugRightClick),
			"gm":    Plug(PlugMiddleClick),
			"gh":    Plug(PlugMoveMouse),
			"<C-e>": Plug(PlugScrollLineDown),
			"<C-y>": Plug(PlugScrollLineUp),
			"<C-d>": Plug(PlugScrollHalfDown),
			"<C-u>": Plug(PlugScrollHalfUp),
			"<C-f>": Plug(PlugScrollPageDown),
			"<C-b>": Plug(PlugScrollPageUp),
			"zh":    Plug(PlugScrollLeft),
			"zl":    Plug(PlugScrollRight),
			"zH":    Plug(PlugScrollHalfLeft),
			"zL":    Plug(PlugScrollHalfRight),
			// vim's own zz puts the cursor on line n for a count, then the
			// app scrolls the line to the middle.
			"zz": "zz" + Plug(PlugCenterLine),
			"]i": Plug(PlugNextIcon),
			"[i": Plug(PlugPrevIcon),
		},
		vim.Visual: {
			"<CR>": OperatorDrag,
		},
	}
}

// With returns k with over laid on it. A mapping in over replaces the one in
// k for the same keys, however either spells them.
func (k Keymap) With(over Keymap) Keymap {
	out := Keymap{}

	for _, layer := range []Keymap{k, over} {
		for mode, mappings := range layer {
			if out[mode] == nil {
				out[mode] = map[string]string{}
			}

			for from, to := range mappings {
				out[mode][vim.Notation(from)] = to
			}
		}
	}

	return out
}

// Apply maps k on the machine, over what it maps for the same keys.
func (k Keymap) Apply(machine *vim.Machine) {
	for mode, mappings := range k {
		for from, to := range mappings {
			machine.Map(mode, from, to)
		}
	}
}

// Held returns the keys a machine with these mappings holds once key is typed
// after held, in mode: the keys that start a longer mapping, or none. It
// follows the machine, which runs the longest mapping the keys start with,
// or else the first key as typed, and then takes the keys after it again.
func (k Keymap) Held(mode vim.Mode, held []vim.Key, key vim.Key) []vim.Key {
	keys := append(slices.Clone(held), key)
	if k.starts(mode, keys) {
		return keys
	}

	ran := 1

	for from := range k[mode] {
		if lhs := vim.Keys(from); len(lhs) > ran && hasPrefix(keys, lhs) {
			ran = len(lhs)
		}
	}

	var rest []vim.Key
	for _, next := range keys[ran:] {
		rest = k.Held(mode, rest, next)
	}

	return rest
}

// Leads reports whether keys, typed in mode, are a mapping or the start of
// one whose keys run match.
func (k Keymap) Leads(mode vim.Mode, keys []vim.Key, match func(to []vim.Key) bool) bool {
	for from, to := range k[mode] {
		if hasPrefix(vim.Keys(from), keys) && match(vim.Keys(to)) {
			return true
		}
	}

	return false
}

// Runs returns the keys a mapping of exactly keys runs in mode.
func (k Keymap) Runs(mode vim.Mode, keys []vim.Key) ([]vim.Key, bool) {
	for from, to := range k[mode] {
		if slices.Equal(vim.Keys(from), keys) {
			return vim.Keys(to), true
		}
	}

	return nil, false
}

// Chords returns the Ctrl chords the mappings are typed with, as the event
// taps name them, such as Ctrl+J for <C-j>.
func (k Keymap) Chords() []string {
	var chords []string

	for _, mappings := range k {
		for from := range mappings {
			for _, key := range vim.Keys(from) {
				if letter, ok := ctrlLetter(key); ok {
					chords = append(chords, "Ctrl+"+strings.ToUpper(letter))
				}
			}
		}
	}

	slices.Sort(chords)

	return slices.Compact(chords)
}

// starts reports whether keys begin a longer mapping in mode.
func (k Keymap) starts(mode vim.Mode, keys []vim.Key) bool {
	for from := range k[mode] {
		if lhs := vim.Keys(from); len(lhs) > len(keys) && hasPrefix(lhs, keys) {
			return true
		}
	}

	return false
}

// hasPrefix reports whether keys begin with prefix.
func hasPrefix(keys, prefix []vim.Key) bool {
	return len(keys) >= len(prefix) && slices.Equal(keys[:len(prefix)], prefix)
}

// ctrlLetter returns the letter of a Ctrl chord such as <C-j>.
func ctrlLetter(key vim.Key) (string, bool) {
	s := string(key)
	if len(s) == len("<C-x>") && strings.HasPrefix(s, "<C-") && s[4] == '>' &&
		s[3] >= 'a' && s[3] <= 'z' {
		return s[3:4], true
	}

	return "", false
}

// CheckMapping says why the keys from cannot be mapped to the keys runs, or
// returns nil. Every key typed has to be one the event taps name, and
// Escape is never mapped, since it always cancels and leaves. Every
// <Plug>(name) run has to be a command vim mode adds. A <name> on either
// side has to be one vim knows, since vim reads one it does not as its
// characters, so <M-j> would map < and then M.
func CheckMapping(from, runs string) error {
	for _, side := range []string{from, runs} {
		if name, ok := unknownName(side); ok {
			return fmt.Errorf("%w: %s, write <lt> for a <", errNotAKey, name)
		}
	}

	keys := vim.Keys(from)
	if len(keys) == 0 {
		return errNoKeys
	}

	for _, key := range keys {
		if key == keyEscape {
			return errMapsEscape
		}

		if !typable(key) {
			return fmt.Errorf("%w: %s", errNotAKey, key)
		}
	}

	if runs == "" {
		return errMapsNothing
	}

	for _, key := range vim.Keys(runs) {
		name, ok := strings.CutPrefix(string(key), "<Plug>(")
		if !ok {
			continue
		}

		name = strings.TrimSuffix(name, ")")
		if !slices.Contains(Plugs, name) {
			return fmt.Errorf("%w: <Plug>(%s)", errNotACommand, name)
		}
	}

	return nil
}

var (
	errNoKeys      = errors.New("maps no keys")
	errMapsEscape  = errors.New("escape always cancels and leaves, so no mapping may use it")
	errMapsNothing = errors.New("maps to nothing, write <Nop> to make the keys do nothing")
	errNotAKey     = errors.New("not a key vim mode reads")
	errNotACommand = errors.New("not a command vim mode has")
)

// unknownName returns the first <name> in keys that vim does not read as one
// key, which it reads as a < key, the name's characters and a > key.
func unknownName(keys string) (string, bool) {
	parsed := vim.Keys(keys)

	for start, key := range parsed {
		if key != "<" {
			continue
		}

		for end := start + 1; end < len(parsed); end++ {
			char := parsed[end]
			if char == ">" && end > start+1 {
				var name strings.Builder
				for _, k := range parsed[start : end+1] {
					name.WriteString(string(k))
				}

				return name.String(), true
			}

			if utf8.RuneCountInString(string(char)) != 1 || char == "<" || char == " " ||
				char == ">" {
				break
			}
		}
	}

	return "", false
}

// typable reports whether Key can turn a key the taps name into key.
func typable(key vim.Key) bool {
	if key == "<S-Tab>" || slices.Contains(slices.Collect(maps.Values(namedKeys)), key) {
		return true
	}

	if _, ok := ctrlLetter(key); ok {
		return true
	}

	return utf8.RuneCountInString(string(key)) == 1
}
