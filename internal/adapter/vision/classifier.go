package vision

import (
	"github.com/y3owk1n/neru/internal/config"
)

// TestClassifier is a pure-Go classifier used in testing and as a fallback
// when no Core ML model is available. It classifies regions using geometric
// heuristics only (see regionClassifier).
//
// A real Core ML classifier can be swapped in later by implementing the
// same interface and loading a .mlpackage from disk.
type TestClassifier struct {
	classifier regionClassifier
}

// NewTestClassifier creates a new test/fallback classifier. It answers in the
// running platform's native role vocabulary, as the real one does.
func NewTestClassifier() *TestClassifier {
	return &TestClassifier{classifier: newRegionClassifier(config.HintsVisionConfig{})}
}

// Classify delegates to the heuristic region classifier.
func (tc *TestClassifier) Classify(region DetectedRegion) (string, bool) {
	return tc.classifier.Classify(region)
}
