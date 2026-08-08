//go:build race

package app_test

// simRaceDetectorEnabled reports whether this test binary was built with
// -race. There is no runtime query for it, so it is answered by the build
// constraint the toolchain sets — the only honest way to know, and the reason
// the wait budgets never try to infer it from wall-clock measurements, which
// on a loaded runner say "slow" for reasons that have nothing to do with the
// race detector.
const simRaceDetectorEnabled = true
