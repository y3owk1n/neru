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
	TrainingResultAdvanceDepth
	TrainingResultCorrect
	TrainingResultWrong
	TrainingResultCompleted
)

// TrainingSession stores the lightweight state needed for recursive-grid
// memorization drills without changing the normal recursive-grid manager.
//
// When second-depth training is enabled, each drill is a pair:
// 1. choose the highlighted top-level cell
// 2. choose the highlighted second-depth cell inside it
//
// Progress is tracked per pair. A top-level label is hidden only after all
// second-depth targets inside that cell are learned.
type TrainingSession struct {
	firstKeys                 []rune
	firstNormalizedKeys       []string
	secondKeys                []rune
	secondNormalizedKeys      []string
	hits                      [][]int
	hitsToHide                int
	penaltyOnError            int
	targetFirstIndex          int
	targetSecondIndex         int
	lastFirstIndex            int
	lastSecondIndex           int
	secondDepthTrainingActive bool
	inSecondDepth             bool
	rng                       *rand.Rand
	active                    bool
}

// NewTrainingSession creates a new training session from the configured recursive-grid layouts.
func NewTrainingSession(
	firstKeys string,
	secondKeys string,
	secondDepthTrainingActive bool,
	hitsToHide int,
	penaltyOnError int,
) *TrainingSession {
	firstRunes := []rune(firstKeys)
	firstNormalized := normalizeTrainingKeys(firstRunes)

	secondRunes := []rune(secondKeys)
	secondNormalized := normalizeTrainingKeys(secondRunes)

	if !secondDepthTrainingActive || len(secondRunes) == 0 {
		secondDepthTrainingActive = false
		secondRunes = []rune{' '}
		secondNormalized = []string{""}
	}

	hits := make([][]int, len(firstRunes))
	for idx := range hits {
		hits[idx] = make([]int, len(secondRunes))
	}

	session := &TrainingSession{
		firstKeys:                 firstRunes,
		firstNormalizedKeys:       firstNormalized,
		secondKeys:                secondRunes,
		secondNormalizedKeys:      secondNormalized,
		hits:                      hits,
		hitsToHide:                hitsToHide,
		penaltyOnError:            penaltyOnError,
		targetFirstIndex:          -1,
		targetSecondIndex:         -1,
		lastFirstIndex:            -1,
		lastSecondIndex:           -1,
		secondDepthTrainingActive: secondDepthTrainingActive,
		rng:                       rand.New(rand.NewSource(time.Now().UnixNano())), //nolint:gosec
		active:                    true,
	}

	session.selectNextTargetPair()

	return session
}

func normalizeTrainingKeys(keys []rune) []string {
	normalized := make([]string, len(keys))
	for idx, keyRune := range keys {
		normalized[idx] = strings.ToLower(
			configpkg.NormalizeKeyForComparison(string(keyRune)),
		)
	}

	return normalized
}

// Active reports whether the session is still in progress.
func (s *TrainingSession) Active() bool {
	return s != nil && s.active
}

// InSecondDepth reports whether the session is currently waiting for the
// second step of the current drill.
func (s *TrainingSession) InSecondDepth() bool {
	return s != nil && s.inSecondDepth
}

// Reset restarts the session while preserving the configured training rules.
func (s *TrainingSession) Reset() {
	if s == nil {
		return
	}

	for firstIdx := range s.hits {
		for secondIdx := range s.hits[firstIdx] {
			s.hits[firstIdx][secondIdx] = 0
		}
	}

	s.active = true
	s.inSecondDepth = false
	s.targetFirstIndex = -1
	s.targetSecondIndex = -1
	s.lastFirstIndex = -1
	s.lastSecondIndex = -1
	s.selectNextTargetPair()
}

// HandleKey validates a key press against the active training target.
func (s *TrainingSession) HandleKey(key string) TrainingResult {
	if s == nil || !s.active {
		return TrainingResultIgnored
	}

	normalized := strings.ToLower(configpkg.NormalizeKeyForComparison(key))
	if normalized == "" {
		return TrainingResultIgnored
	}

	if s.inSecondDepth {
		if s.targetSecondIndex < 0 || s.targetSecondIndex >= len(s.secondNormalizedKeys) {
			return TrainingResultIgnored
		}

		if normalized != s.secondNormalizedKeys[s.targetSecondIndex] {
			s.applyPenalty()

			return TrainingResultWrong
		}

		return s.completeCurrentTarget()
	}

	if s.targetFirstIndex < 0 || s.targetFirstIndex >= len(s.firstNormalizedKeys) {
		return TrainingResultIgnored
	}

	if normalized != s.firstNormalizedKeys[s.targetFirstIndex] {
		s.applyPenalty()

		return TrainingResultWrong
	}

	if s.secondDepthTrainingActive {
		s.inSecondDepth = true

		return TrainingResultAdvanceDepth
	}

	return s.completeCurrentTarget()
}

func (s *TrainingSession) completeCurrentTarget() TrainingResult {
	s.hits[s.targetFirstIndex][s.targetSecondIndex]++
	s.inSecondDepth = false
	s.lastFirstIndex = s.targetFirstIndex
	s.lastSecondIndex = s.targetSecondIndex
	s.selectNextTargetPair()
	if !s.active {
		return TrainingResultCompleted
	}

	return TrainingResultCorrect
}

func (s *TrainingSession) applyPenalty() {
	if s.penaltyOnError <= 0 || s.targetFirstIndex < 0 || s.targetSecondIndex < 0 {
		return
	}

	s.hits[s.targetFirstIndex][s.targetSecondIndex] -= s.penaltyOnError
	if s.hits[s.targetFirstIndex][s.targetSecondIndex] < 0 {
		s.hits[s.targetFirstIndex][s.targetSecondIndex] = 0
	}
}

// TargetIndex returns the active target index for the current step, or -1 if training is complete.
func (s *TrainingSession) TargetIndex() int {
	if s == nil {
		return -1
	}

	if s.inSecondDepth {
		return s.targetSecondIndex
	}

	return s.targetFirstIndex
}

// OverlayKeys returns the configured keys for the current training step with
// learned labels replaced by spaces so the existing recursive-grid overlay can
// render them as hidden labels.
func (s *TrainingSession) OverlayKeys() string {
	if s == nil {
		return ""
	}

	if s.inSecondDepth {
		return s.secondDepthOverlayKeys()
	}

	return s.firstDepthOverlayKeys()
}

func (s *TrainingSession) firstDepthOverlayKeys() string {
	overlayKeys := make([]rune, len(s.firstKeys))
	copy(overlayKeys, s.firstKeys)

	for firstIdx := range overlayKeys {
		if s.firstDepthLearned(firstIdx) {
			overlayKeys[firstIdx] = ' '
		}
	}

	return string(overlayKeys)
}

func (s *TrainingSession) secondDepthOverlayKeys() string {
	overlayKeys := make([]rune, len(s.secondKeys))
	copy(overlayKeys, s.secondKeys)

	for secondIdx := range overlayKeys {
		if s.hits[s.targetFirstIndex][secondIdx] >= s.hitsToHide {
			overlayKeys[secondIdx] = ' '
		}
	}

	return string(overlayKeys)
}

func (s *TrainingSession) firstDepthLearned(firstIdx int) bool {
	if firstIdx < 0 || firstIdx >= len(s.hits) {
		return false
	}

	for _, hits := range s.hits[firstIdx] {
		if hits < s.hitsToHide {
			return false
		}
	}

	return true
}

// Progress returns the number of learned drills and total drills.
func (s *TrainingSession) Progress() (int, int) {
	if s == nil {
		return 0, 0
	}

	learned := 0
	total := 0
	for firstIdx := range s.hits {
		for secondIdx := range s.hits[firstIdx] {
			total++
			if s.hits[firstIdx][secondIdx] >= s.hitsToHide {
				learned++
			}
		}
	}

	return learned, total
}

func (s *TrainingSession) selectNextTargetPair() {
	if s == nil {
		return
	}

	firstCandidates := make([]int, 0, len(s.hits))
	for firstIdx := range s.hits {
		if !s.firstDepthLearned(firstIdx) {
			firstCandidates = append(firstCandidates, firstIdx)
		}
	}

	if len(firstCandidates) == 0 {
		s.targetFirstIndex = -1
		s.targetSecondIndex = -1
		s.active = false
		s.inSecondDepth = false

		return
	}

	if len(firstCandidates) > 1 && s.lastFirstIndex >= 0 {
		filtered := firstCandidates[:0]
		for _, idx := range firstCandidates {
			if idx != s.lastFirstIndex {
				filtered = append(filtered, idx)
			}
		}

		if len(filtered) > 0 {
			firstCandidates = filtered
		}
	}

	s.targetFirstIndex = firstCandidates[s.rng.Intn(len(firstCandidates))]
	s.targetSecondIndex = s.selectSecondTarget(s.targetFirstIndex)
	s.inSecondDepth = false
}

func (s *TrainingSession) selectSecondTarget(firstIdx int) int {
	secondCandidates := make([]int, 0, len(s.hits[firstIdx]))
	for secondIdx, hits := range s.hits[firstIdx] {
		if hits < s.hitsToHide {
			secondCandidates = append(secondCandidates, secondIdx)
		}
	}

	if len(secondCandidates) == 0 {
		return -1
	}

	if len(secondCandidates) > 1 && firstIdx == s.lastFirstIndex && s.lastSecondIndex >= 0 {
		filtered := secondCandidates[:0]
		for _, idx := range secondCandidates {
			if idx != s.lastSecondIndex {
				filtered = append(filtered, idx)
			}
		}

		if len(filtered) > 0 {
			secondCandidates = filtered
		}
	}

	return secondCandidates[s.rng.Intn(len(secondCandidates))]
}
