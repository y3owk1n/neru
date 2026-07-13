package electron_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/core/infra/electron"
)

const (
	testNameUnknownBundle                   = "unknown bundle"
	bundleIDSafari                          = "com.apple.Safari"
	testNameEmptyBundle                     = "empty bundle"
	testNameAdditionalBundle                = "additional bundle"
	bundleIDMyApp                           = "com.example.MyApp"
	testNameCaseInsensitiveAdditionalBundle = "case insensitive additional bundle"
	bundleIDMyAppUpper                      = "COM.EXAMPLE.MYAPP"
)

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
			name:              testNameAdditionalBundle,
			bundleID:          bundleIDMyApp,
			additionalBundles: []string{bundleIDMyApp},
			want:              true,
		},
		{
			name:              testNameUnknownBundle,
			bundleID:          bundleIDSafari,
			additionalBundles: []string{},
			want:              false,
		},
		{
			name:              testNameEmptyBundle,
			bundleID:          "",
			additionalBundles: []string{},
			want:              false,
		},
		{
			name:              testNameCaseInsensitiveAdditionalBundle,
			bundleID:          bundleIDMyApp,
			additionalBundles: []string{bundleIDMyAppUpper},
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
