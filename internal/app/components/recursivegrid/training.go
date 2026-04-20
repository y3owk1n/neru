package recursivegrid

import (
	"math/rand"
	"strings"
	"time"

	configpkg "github.com/y3owk1n/neru/internal/config"
)

// TrainingResult describes the outcome of a training key press.
type TrainingResult int

const (
	TrainingResultIgnored TrainingResult = iota
	TrainingResultCorrect
	TrainingResultWrong
	TrainingResultCompleted
)

// TrainingSession stores the lightweight state needed for recursive-grid
// memorization drills without changing the normal recursive-grid manager.
type TrainingSession struct {
	keys           []rune
	normalizedKeys []string
	hits           []int
	hitsToHide     int
	penaltyOnError int
	targetIndex    int
	lastTarget     int
	rng            *rand.Rand
	active         bool
}

// NewTrainingSession creates a new training session from the configured top-level key layout.
func NewTrainingSession(keys string, hitsToHide, penaltyOnError int) *TrainingSession {
	keyRunes := []rune(keys)
	normalizedKeys := make([]string, len(keyRunes))
	for idx, keyRune := range keyRunes {
		normalizedKeys[idx] = strings.ToLower(
			configpkg.NormalizeKeyForComparison(string(keyRune)),
		)
	}

	session := &TrainingSession{
		keys:           keyRunes,
		normalizedKeys: normalizedKeys,
		hits:           make([]int, len(keyRunes)),
		hitsToHide:     hitsToHide,
		penaltyOnError: penaltyOnError,
		targetIndex:    -1,
		lastTarget:     -1,
		rng:            rand.New(rand.NewSource(time.Now().UnixNano())), //nolint:gosec
		active:         true,
	}

	session.selectNextTarget()

	return session
}

// Active reports whether the session is still in progress.
func (s *TrainingSession) Active() bool {
	return s != nil && s.active
}

// Reset restarts the session while preserving the configured training rules.
func (s *TrainingSession) Reset() {
	if s == nil {
		return
	}

	for idx := range s.hits {
		s.hits[idx] = 0
	}

	s.active = true
	s.targetIndex = -1
	s.lastTarget = -1
	s.selectNextTarget()
}

// HandleKey validates a key press against the active training target.
func (s *TrainingSession) HandleKey(key string) TrainingResult {
	if s == nil || !s.active {
		return TrainingResultIgnored
	}

	normalized := strings.ToLower(configpkg.NormalizeKeyForComparison(key))
	if normalized == "" || s.targetIndex < 0 || s.targetIndex >= len(s.normalizedKeys) {
		return TrainingResultIgnored
	}

	if normalized == s.normalizedKeys[s.targetIndex] {
		s.hits[s.targetIndex]++
		s.lastTarget = s.targetIndex
		s.selectNextTarget()
		if !s.active {
			return TrainingResultCompleted
		}

		return TrainingResultCorrect
	}

	if s.penaltyOnError > 0 {
		s.hits[s.targetIndex] -= s.penaltyOnError
		if s.hits[s.targetIndex] < 0 {
			s.hits[s.targetIndex] = 0
		}
	}

	return TrainingResultWrong
}

// TargetIndex returns the active target index, or -1 if training is complete.
func (s *TrainingSession) TargetIndex() int {
	if s == nil {
		return -1
	}

	return s.targetIndex
}

// OverlayKeys returns the configured keys with learned cells replaced by spaces
// so the existing recursive-grid overlay can render them as hidden labels.
func (s *TrainingSession) OverlayKeys() string {
	if s == nil {
		return ""
	}

	overlayKeys := make([]rune, len(s.keys))
	copy(overlayKeys, s.keys)

	for idx := range overlayKeys {
		if s.hits[idx] >= s.hitsToHide {
			overlayKeys[idx] = ' '
		}
	}

	return string(overlayKeys)
}

// Progress returns the number of learned cells and total cells.
func (s *TrainingSession) Progress() (int, int) {
	if s == nil {
		return 0, 0
	}

	learned := 0
	for _, hits := range s.hits {
		if hits >= s.hitsToHide {
			learned++
		}
	}

	return learned, len(s.hits)
}

func (s *TrainingSession) selectNextTarget() {
	if s == nil {
		return
	}

	candidates := make([]int, 0, len(s.hits))
	for idx, hits := range s.hits {
		if hits < s.hitsToHide {
			candidates = append(candidates, idx)
		}
	}

	if len(candidates) == 0 {
		s.targetIndex = -1
		s.active = false

		return
	}

	if len(candidates) > 1 && s.lastTarget >= 0 {
		filtered := candidates[:0]
		for _, idx := range candidates {
			if idx != s.lastTarget {
				filtered = append(filtered, idx)
			}
		}

		if len(filtered) > 0 {
			candidates = filtered
		}
	}

	s.targetIndex = candidates[s.rng.Intn(len(candidates))]
}
