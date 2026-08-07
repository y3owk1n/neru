package app_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/y3owk1n/neru/internal/domain"
)

// TestSimulation_ConfigReloadRacingGridActivation is the regression for #1277.
//
// A reload rebuilds the grid mode's domain state — the grid itself and the
// subgrid keys — and the manager it rebuilds is the one the handler assigns on
// activation and reads on every keystroke, under `h.mu`. The manager has no
// lock of its own, so the write has to happen under that same lock or it is a
// plain data race on state a keystroke reads.
//
// The reload exits the active mode before it reconfigures, which narrows the
// window but does not close it: an activation arriving over IPC between the
// exit and the component update runs `ActivateMode` on one goroutine and the
// rebuild on another. This drives exactly that pair, so `-race` reaches what no
// single-goroutine journey can.
func TestSimulation_ConfigReloadRacingGridActivation(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)

	// Two files the reload alternates between. The character sets differ, so a
	// reload rebuilds the grid rather than recognizing the set already in use
	// and leaving the manager alone — the rebuild is the write under test.
	dir := t.TempDir()
	paths := [2]string{filepath.Join(dir, "a.toml"), filepath.Join(dir, "b.toml")}
	characters := [2]string{"asdfghjkl", "qwertyuip"}
	sublayerKeys := [2]string{"uiopjklnm", "asdfghzxc"}

	for index, path := range paths {
		writeErr := os.WriteFile(path, fmt.Appendf(nil, `
[hotkeys]
%q = "grid"

[grid]
enabled = true
characters = %q
sublayer_keys = %q
`, gridHotkey, characters[index], sublayerKeys[index]), 0o600)
		if writeErr != nil {
			t.Fatalf("failed to write the reloaded config: %v", writeErr)
		}
	}

	const reloads = 20

	done := make(chan struct{})

	var racing sync.WaitGroup

	racing.Add(2)

	go func() {
		defer racing.Done()
		defer close(done)

		for round := range reloads {
			reloadErr := sim.app.ReloadConfig(context.Background(), paths[round%len(paths)])
			if reloadErr != nil {
				t.Errorf("ReloadConfig() error = %v", reloadErr)

				return
			}
		}
	}()

	// Activations run for as long as reloads do, rather than a fixed count: a
	// reload reads and writes a file, so a counted loop here would finish long
	// before the first one and never overlap the window.
	//
	// They go in through ActivateMode rather than pressHotkey, which is what a
	// journey would use: pressHotkey waits on the hotkey registration and fails
	// the test from whichever goroutine it is on, and t.Fatalf off the test
	// goroutine is not allowed. ActivateMode is the entry point an IPC
	// activation — `neru grid` — reaches anyway, which is the racing pair the
	// issue names.
	go func() {
		defer racing.Done()

		for {
			select {
			case <-done:
				return
			default:
			}

			sim.app.ActivateMode(domain.ModeGrid)

			// One key from each alphabet, so a keystroke reads the manager
			// whichever of the two configurations is in force.
			sim.press("a", "q")
			sim.app.ExitMode()
		}
	}()

	racing.Wait()

	sim.app.ExitMode()
}
