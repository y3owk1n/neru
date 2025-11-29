//go:build unit

package electron_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/core/infra/electron"
)

func TestIsLikelyElectronBundle(t *testing.T) {
	tests := []struct {
		name     string
		bundleID string
		want     bool
	}{
		{
			name:     "known electron bundle",
			bundleID: "com.microsoft.VSCode",
			want:     true,
		},
		{
			name:     "case insensitive match",
			bundleID: "COM.MICROSOFT.VSCODE",
			want:     true,
		},
		{
			name:     "unknown bundle",
			bundleID: "com.apple.Safari",
			want:     false,
		},
		{
			name:     "empty bundle",
			bundleID: "",
			want:     false,
		},
		{
			name:     "whitespace bundle",
			bundleID: "  ",
			want:     false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := electron.IsLikelyElectronBundle(testCase.bundleID)
			if got != testCase.want {
				t.Errorf(
					"IsLikelyElectronBundle(%q) = %v, want %v",
					testCase.bundleID,
					got,
					testCase.want,
				)
			}
		})
	}
}

func TestIsLikelyChromiumBundle(t *testing.T) {
	tests := []struct {
		name     string
		bundleID string
		want     bool
	}{
		{
			name:     "known chromium bundle",
			bundleID: "com.google.Chrome",
			want:     true,
		},
		{
			name:     "case insensitive match",
			bundleID: "COM.GOOGLE.CHROME",
			want:     true,
		},
		{
			name:     "unknown bundle",
			bundleID: "com.apple.Safari",
			want:     false,
		},
		{
			name:     "empty bundle",
			bundleID: "",
			want:     false,
		},
		{
			name:     "whitespace bundle",
			bundleID: "  ",
			want:     false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := electron.IsLikelyChromiumBundle(testCase.bundleID)
			if got != testCase.want {
				t.Errorf(
					"IsLikelyChromiumBundle(%q) = %v, want %v",
					testCase.bundleID,
					got,
					testCase.want,
				)
			}
		})
	}
}

func TestShouldEnableElectronSupport(t *testing.T) {
	tests := []struct {
		name              string
		bundleID          string
		additionalBundles []string
		want              bool
	}{
		{
			name:              "known electron bundle",
			bundleID:          "com.microsoft.VSCode",
			additionalBundles: []string{},
			want:              true,
		},
		{
			name:              "additional bundle",
			bundleID:          "com.example.MyApp",
			additionalBundles: []string{"com.example.MyApp"},
			want:              true,
		},
		{
			name:              "unknown bundle",
			bundleID:          "com.apple.Safari",
			additionalBundles: []string{},
			want:              false,
		},
		{
			name:              "empty bundle",
			bundleID:          "",
			additionalBundles: []string{},
			want:              false,
		},
		{
			name:              "case insensitive additional bundle",
			bundleID:          "com.example.MyApp",
			additionalBundles: []string{"COM.EXAMPLE.MYAPP"},
			want:              true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := electron.ShouldEnableElectronSupport(
				testCase.bundleID,
				testCase.additionalBundles,
			)
			if got != testCase.want {
				t.Errorf(
					"ShouldEnableElectronSupport(%q, %v) = %v, want %v",
					testCase.bundleID,
					testCase.additionalBundles,
					got,
					testCase.want,
				)
			}
		})
	}
}

func TestShouldEnableChromiumSupport(t *testing.T) {
	tests := []struct {
		name              string
		bundleID          string
		additionalBundles []string
		want              bool
	}{
		{
			name:              "known chromium bundle",
			bundleID:          "com.google.Chrome",
			additionalBundles: []string{},
			want:              true,
		},
		{
			name:              "additional bundle",
			bundleID:          "com.example.MyApp",
			additionalBundles: []string{"com.example.MyApp"},
			want:              true,
		},
		{
			name:              "unknown bundle",
			bundleID:          "com.apple.Safari",
			additionalBundles: []string{},
			want:              false,
		},
		{
			name:              "empty bundle",
			bundleID:          "",
			additionalBundles: []string{},
			want:              false,
		},
		{
			name:              "case insensitive additional bundle",
			bundleID:          "com.example.MyApp",
			additionalBundles: []string{"COM.EXAMPLE.MYAPP"},
			want:              true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := electron.ShouldEnableChromiumSupport(
				testCase.bundleID,
				testCase.additionalBundles,
			)
			if got != testCase.want {
				t.Errorf(
					"ShouldEnableChromiumSupport(%q, %v) = %v, want %v",
					testCase.bundleID,
					testCase.additionalBundles,
					got,
					testCase.want,
				)
			}
		})
	}
}

func TestShouldEnableFirefoxSupport(t *testing.T) {
	tests := []struct {
		name              string
		bundleID          string
		additionalBundles []string
		want              bool
	}{
		{
			name:              "known firefox bundle",
			bundleID:          "org.mozilla.firefox",
			additionalBundles: []string{},
			want:              true,
		},
		{
			name:              "additional bundle",
			bundleID:          "com.example.MyApp",
			additionalBundles: []string{"com.example.MyApp"},
			want:              true,
		},
		{
			name:              "unknown bundle",
			bundleID:          "com.apple.Safari",
			additionalBundles: []string{},
			want:              false,
		},
		{
			name:              "empty bundle",
			bundleID:          "",
			additionalBundles: []string{},
			want:              false,
		},
		{
			name:              "case insensitive additional bundle",
			bundleID:          "com.example.MyApp",
			additionalBundles: []string{"COM.EXAMPLE.MYAPP"},
			want:              true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := electron.ShouldEnableFirefoxSupport(
				testCase.bundleID,
				testCase.additionalBundles,
			)
			if got != testCase.want {
				t.Errorf(
					"ShouldEnableFirefoxSupport(%q, %v) = %v, want %v",
					testCase.bundleID,
					testCase.additionalBundles,
					got,
					testCase.want,
				)
			}
		})
	}
}
