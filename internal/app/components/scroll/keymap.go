package scroll

import (
	"fmt"
	"strings"
	"time"

	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
)

// SequenceTimeout is the maximum time allowed between keys in a multi-key sequence.
const SequenceTimeout = 500 * time.Millisecond

// Arrow key names for normalized key mapping.
const (
	ArrowUp    = "Up"
	ArrowDown  = "Down"
	ArrowLeft  = "Left"
	ArrowRight = "Right"
	PageUp     = "PageUp"
	PageDown   = "PageDown"
)

// Scroll action names used in configuration.
const (
	ActionScrollUp    = "scroll_up"
	ActionScrollDown  = "scroll_down"
	ActionScrollLeft  = "scroll_left"
	ActionScrollRight = "scroll_right"
	ActionGoTop       = "go_top"
	ActionGoBottom    = "go_bottom"
	ActionPageUp      = "page_up"
	ActionPageDown    = "page_down"
)

// Action represents a scroll action with direction and amount.
type Action struct {
	Direction services.ScrollDirection
	Amount    services.ScrollAmount
}

// KeyMap maps key bindings to scroll actions and handles key sequence detection.
type KeyMap struct {
	actions      map[string]Action
	keyToAction  map[string]string
	sequences    map[string]string
	sequenceKeys map[string]bool
}

// NewKeyMap creates a new KeyMap from the given bindings.
// Bindings is a map from action names to lists of key strings.
func NewKeyMap(bindings map[string][]string) *KeyMap {
	keyMap := &KeyMap{
		actions: map[string]Action{
			ActionScrollUp: {
				Direction: services.ScrollDirectionUp,
				Amount:    services.ScrollAmountChar,
			},
			ActionScrollDown: {
				Direction: services.ScrollDirectionDown,
				Amount:    services.ScrollAmountChar,
			},
			ActionScrollLeft: {
				Direction: services.ScrollDirectionLeft,
				Amount:    services.ScrollAmountChar,
			},
			ActionScrollRight: {
				Direction: services.ScrollDirectionRight,
				Amount:    services.ScrollAmountChar,
			},
			ActionGoTop: {
				Direction: services.ScrollDirectionUp,
				Amount:    services.ScrollAmountEnd,
			},
			ActionGoBottom: {
				Direction: services.ScrollDirectionDown,
				Amount:    services.ScrollAmountEnd,
			},
			ActionPageUp: {
				Direction: services.ScrollDirectionUp,
				Amount:    services.ScrollAmountHalfPage,
			},
			ActionPageDown: {
				Direction: services.ScrollDirectionDown,
				Amount:    services.ScrollAmountHalfPage,
			},
		},
		keyToAction:  make(map[string]string),
		sequences:    make(map[string]string),
		sequenceKeys: make(map[string]bool),
	}

	for action, keys := range bindings {
		for _, key := range keys {
			normalized := keyMap.normalizeKey(key)
			if normalized == "" {
				continue
			}

			if len(normalized) == 2 && config.IsAllLetters(normalized) && !isNamedKey(normalized) {
				keyMap.sequences[normalized] = action
				keyMap.sequenceKeys[keyMap.sequenceFirstKey(normalized)] = true
			} else {
				keyMap.keyToAction[normalized] = action
			}
		}
	}

	return keyMap
}

func isNamedKey(key string) bool {
	namedKeys := map[string]bool{
		"Space":     true,
		"Return":    true,
		"Enter":     true,
		"Escape":    true,
		"Tab":       true,
		"Delete":    true,
		"Backspace": true,
		"Home":      true,
		"End":       true,
		"PageUp":    true,
		"PageDown":  true,
		"Up":        true,
		"Down":      true,
		"Left":      true,
		"Right":     true,
		"Cmd+Up":    true,
		"Cmd+Down":  true,
		"Cmd+Left":  true,
		"Cmd+Right": true,
		"Cmd+Home":  true,
		"Cmd+End":   true,
		"Alt+Up":    true,
		"Alt+Down":  true,
		"Alt+Left":  true,
		"Alt+Right": true,
		"Ctrl+Z":    true,
		"Ctrl+U":    true,
		"Ctrl+D":    true,
		"Ctrl+A":    true,
		"Ctrl+E":    true,
	}

	return namedKeys[key]
}

// Action returns the Action struct for a given action name and whether it was found.
func (m *KeyMap) Action(name string) (Action, bool) {
	action, found := m.actions[name]

	return action, found
}

// Lookup returns the action name for a given key and whether it was found.
func (m *KeyMap) Lookup(key string) (string, bool) {
	normalized := m.normalizeKey(key)
	if normalized == "" {
		return "", false
	}

	action, found := m.keyToAction[normalized]

	return action, found
}

// Normalize converts a key string to its canonical form.
func (m *KeyMap) Normalize(key string) string {
	return m.normalizeKey(key)
}

// IsSequenceStart returns true if the given key can start a multi-key sequence.
func (m *KeyMap) IsSequenceStart(key string) bool {
	normalized := m.normalizeKey(key)

	return m.sequenceKeys[normalized]
}

// LookupSequence returns the action name for a complete key sequence and whether it was found.
func (m *KeyMap) LookupSequence(seq string) (string, bool) {
	normalized := strings.ToLower(seq)
	action, found := m.sequences[normalized]

	return action, found
}

// CanCompleteSequence returns true if the given two keys form a complete sequence.
func (m *KeyMap) CanCompleteSequence(first, second string) bool {
	expected := strings.ToLower(first + second)
	_, found := m.sequences[expected]

	return found
}

// KeyToAction returns the map of single keys to action names.
func (m *KeyMap) KeyToAction() map[string]string {
	return m.keyToAction
}

// Sequences returns the map of key sequences to action names.
func (m *KeyMap) Sequences() map[string]string {
	return m.sequences
}

func (m *KeyMap) normalizeKey(key string) string {
	switch key {
	case ArrowUp, "\x1f":
		return ArrowUp
	case ArrowDown, "\x1e":
		return ArrowDown
	case ArrowLeft, "\x1d":
		return ArrowLeft
	case ArrowRight, "\x1c":
		return ArrowRight
	case PageUp:
		return PageUp
	case PageDown:
		return PageDown
	case "Home":
		return "Home"
	case "End":
		return "End"
	default:
		lower := strings.ToLower(key)
		if strings.HasPrefix(lower, "ctrl+") || strings.HasPrefix(lower, "alt+") ||
			strings.HasPrefix(lower, "cmd+") ||
			strings.HasPrefix(lower, "option+") {
			return lower
		}

		if len(key) == 1 {
			r := rune(key[0])
			if r >= 0x01 && r <= 0x1a {
				return fmt.Sprintf("ctrl+%c", 'a'+r-1)
			}
		}

		return key
	}
}

func (m *KeyMap) sequenceFirstKey(seq string) string {
	if len(seq) > 0 {
		return seq[:1]
	}

	return ""
}

// SequenceState tracks the state of an incomplete key sequence.
type SequenceState struct {
	FirstKey   string
	ReceivedAt int64
}

// NewSequenceState creates a new SequenceState for the given first key and timestamp.
func NewSequenceState(key string, receivedAt int64) *SequenceState {
	return &SequenceState{
		FirstKey:   key,
		ReceivedAt: receivedAt,
	}
}

// Expired returns true if the sequence has exceeded the timeout window.
func (s *SequenceState) Expired() bool {
	return time.Since(time.Unix(0, s.ReceivedAt)) > SequenceTimeout
}
