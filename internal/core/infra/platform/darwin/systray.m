// systray.m — this file is intentionally empty.
//
// The systray Objective-C implementation lives in internal/core/infra/systray/systray.m
// because CGo only compiles .c/.m files co-located with the Go package that
// contains `import "C"`. Only the header (systray.h) is kept here to
// consolidate all macOS headers in platform/darwin/.
//
// Do NOT add implementation here — it would cause duplicate symbol errors
// with the copy in the systray package.
