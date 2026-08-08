//go:build !race

package app_test

// simRaceDetectorEnabled reports whether this test binary was built with
// -race. See the //go:build race half of this pair for why it is a build
// constraint rather than a measurement.
const simRaceDetectorEnabled = false
