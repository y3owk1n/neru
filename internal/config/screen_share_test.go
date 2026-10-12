package config_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain/parity"
)

// TestHidesFromScreenShare pins where hiding the overlay from a screen share
// does anything, which decides where toggle-screen-share answers and where
// the hidden state, its event and the on_screen_share_hide hook can change.
func TestHidesFromScreenShare(t *testing.T) {
	for platform, want := range map[parity.Platform]bool{
		parity.Darwin:  true,
		parity.Linux:   false,
		parity.Windows: false,
	} {
		if got := config.HidesFromScreenShare(platform); got != want {
			t.Errorf("HidesFromScreenShare(%s) = %t, want %t", platform, got, want)
		}
	}
}
