package modes

import (
	"image"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	configpkg "github.com/y3owk1n/neru/internal/config"
)

const (
	labelInitialCapacity = 2
)

type monitorSelectTarget struct {
	Name      string
	Bounds    image.Rectangle
	Label     string
	IsCurrent bool
}

type monitorSelectSession struct {
	characters    []rune
	input         string
	targets       []monitorSelectTarget
	current       *monitorSelectTarget
	selectedIndex int
}

func newMonitorSelectSession(
	monitors []monitorSelectTarget,
	currentBounds image.Rectangle,
	cfg configpkg.MonitorSelectConfig,
) *monitorSelectSession {
	if len(monitors) <= 1 {
		return nil
	}

	// Sort ALL monitors in a fixed spatial order so labels are deterministic
	// regardless of which monitor is current.
	sortSpatially(monitors)

	currentIndex := findCurrentMonitorIndex(monitors, currentBounds)
	if currentIndex < 0 {
		return nil
	}

	// Assign positional labels to ALL monitors based on the fixed spatial order.
	assignMonitorLabels(monitors, cfg.Characters)

	targets := make([]monitorSelectTarget, len(monitors))

	var currentPtr *monitorSelectTarget

	for idx := range monitors {
		if idx == currentIndex {
			monitors[idx].IsCurrent = true
			currentCopy := monitors[idx]
			currentPtr = &currentCopy
		}

		targets[idx] = monitors[idx]
	}

	return &monitorSelectSession{
		characters:    []rune(cfg.Characters),
		targets:       targets,
		current:       currentPtr,
		selectedIndex: 0,
	}
}

func findCurrentMonitorIndex(monitors []monitorSelectTarget, currentBounds image.Rectangle) int {
	for idx, monitor := range monitors {
		if monitor.Bounds == currentBounds {
			return idx
		}
	}

	center := image.Point{
		X: currentBounds.Min.X + currentBounds.Dx()/2,
		Y: currentBounds.Min.Y + currentBounds.Dy()/2,
	}

	for idx, monitor := range monitors {
		if center.In(monitor.Bounds) {
			return idx
		}
	}

	return -1
}

func sortSpatially(monitors []monitorSelectTarget) {
	sort.SliceStable(monitors, func(left, right int) bool {
		leftTarget := monitors[left]

		rightTarget := monitors[right]
		if leftTarget.Bounds.Min.Y != rightTarget.Bounds.Min.Y {
			return leftTarget.Bounds.Min.Y < rightTarget.Bounds.Min.Y
		}

		if leftTarget.Bounds.Min.X != rightTarget.Bounds.Min.X {
			return leftTarget.Bounds.Min.X < rightTarget.Bounds.Min.X
		}

		if leftTarget.Bounds.Dy() != rightTarget.Bounds.Dy() {
			return leftTarget.Bounds.Dy() < rightTarget.Bounds.Dy()
		}

		if leftTarget.Bounds.Dx() != rightTarget.Bounds.Dx() {
			return leftTarget.Bounds.Dx() < rightTarget.Bounds.Dx()
		}

		return strings.Compare(
			strings.ToLower(leftTarget.Name),
			strings.ToLower(rightTarget.Name),
		) < 0
	})
}

func assignMonitorLabels(targets []monitorSelectTarget, characters string) {
	alphabet := []rune(characters)
	if len(alphabet) == 0 {
		return
	}

	for idx := range targets {
		targets[idx].Label = monitorSelectLabelForIndex(alphabet, idx)
	}
}

func monitorSelectLabelForIndex(alphabet []rune, index int) string {
	if len(alphabet) == 0 {
		return ""
	}

	base := len(alphabet)
	value := index + 1
	label := make([]rune, 0, labelInitialCapacity)

	for value > 0 {
		value--
		label = append(label, alphabet[value%base])
		value /= base
	}

	for left, right := 0, len(label)-1; left < right; left, right = left+1, right-1 {
		label[left], label[right] = label[right], label[left]
	}

	return string(label)
}

func (s *monitorSelectSession) Current() *monitorSelectTarget {
	return s.current
}

func (s *monitorSelectSession) Input() string {
	if s == nil {
		return ""
	}

	return s.input
}

func (s *monitorSelectSession) Targets() []monitorSelectTarget {
	if s == nil {
		return nil
	}

	targets := make([]monitorSelectTarget, len(s.targets))
	copy(targets, s.targets)

	return targets
}

func (s *monitorSelectSession) Selected() *monitorSelectTarget {
	if s == nil || len(s.targets) == 0 {
		return nil
	}

	if s.selectedIndex < 0 || s.selectedIndex >= len(s.targets) {
		return nil
	}

	return &s.targets[s.selectedIndex]
}

func (s *monitorSelectSession) Confirm() *monitorSelectTarget {
	return s.Selected()
}

func (s *monitorSelectSession) HandleCharacter(key string) *monitorSelectTarget {
	if s == nil {
		return nil
	}

	if utf8.RuneCountInString(key) != 1 {
		return nil
	}

	r, _ := utf8.DecodeRuneInString(key)
	if !s.supportsRune(r) {
		return nil
	}

	nextInput := s.input + string(r)

	matchIndices := s.matchingIndices(nextInput)
	if len(matchIndices) == 0 {
		return nil
	}

	s.input = nextInput

	s.selectedIndex = matchIndices[0]
	if len(matchIndices) == 1 && strings.EqualFold(s.targets[s.selectedIndex].Label, s.input) {
		return &s.targets[s.selectedIndex]
	}

	return nil
}

func (s *monitorSelectSession) Backspace() {
	if s == nil || s.input == "" {
		return
	}

	_, size := utf8.DecodeLastRuneInString(s.input)
	s.input = s.input[:len(s.input)-size]
	s.reselectAfterInputChange()
}

func (s *monitorSelectSession) Cycle(backward bool) {
	if s == nil || len(s.targets) == 0 {
		return
	}

	matchIndices := s.matchingIndices(s.input)
	if len(matchIndices) == 0 {
		matchIndices = make([]int, len(s.targets))
		for idx := range s.targets {
			matchIndices[idx] = idx
		}
	}

	currentPosition := 0
	for idx, absoluteIndex := range matchIndices {
		if absoluteIndex == s.selectedIndex {
			currentPosition = idx

			break
		}
	}

	if backward {
		currentPosition = (currentPosition - 1 + len(matchIndices)) % len(matchIndices)
	} else {
		currentPosition = (currentPosition + 1) % len(matchIndices)
	}

	s.selectedIndex = matchIndices[currentPosition]
}

func (s *monitorSelectSession) reselectAfterInputChange() {
	matchIndices := s.matchingIndices(s.input)
	if len(matchIndices) == 0 {
		if len(s.targets) > 0 {
			s.selectedIndex = 0
		}

		return
	}

	if slices.Contains(matchIndices, s.selectedIndex) {
		return
	}

	s.selectedIndex = matchIndices[0]
}

func (s *monitorSelectSession) matchingIndices(input string) []int {
	if len(s.targets) == 0 {
		return nil
	}

	if input == "" {
		indices := make([]int, len(s.targets))
		for idx := range s.targets {
			indices[idx] = idx
		}

		return indices
	}

	indices := make([]int, 0, len(s.targets))
	for idx, target := range s.targets {
		if strings.HasPrefix(strings.ToLower(target.Label), strings.ToLower(input)) {
			indices = append(indices, idx)
		}
	}

	return indices
}

func (s *monitorSelectSession) supportsRune(r rune) bool {
	for _, allowed := range s.characters {
		if unicode.ToLower(allowed) == unicode.ToLower(r) {
			return true
		}
	}

	return false
}
