package vision

// TestClassifier is a pure-Go classifier used in testing and as a fallback
// when no Core ML model is available. It classifies regions using geometric
// heuristics only (see regionClassifier).
//
// A real Core ML classifier can be swapped in later by implementing the
// same interface and loading a .mlpackage from disk.
type TestClassifier struct {
	classifier regionClassifier
}

// NewTestClassifier creates a new test/fallback classifier.
func NewTestClassifier() *TestClassifier {
	return &TestClassifier{}
}

// Classify delegates to the heuristic region classifier.
func (tc *TestClassifier) Classify(region DetectedRegion) (string, bool) {
	return tc.classifier.Classify(region)
}
