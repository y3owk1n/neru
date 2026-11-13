package manager_test

import (
	"sync"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
)

func TestVimSpin_StepsUntilStopped(t *testing.T) {
	var (
		lock  sync.Mutex
		steps []int
	)

	spin := manager.StartVimSpin(&lock, func(step int) { steps = append(steps, step) })

	time.Sleep(350 * time.Millisecond)

	lock.Lock()
	spin.Stop()

	taken := append([]int(nil), steps...)
	lock.Unlock()

	time.Sleep(250 * time.Millisecond)

	lock.Lock()
	defer lock.Unlock()

	if len(taken) < 2 || taken[0] != 1 || taken[1] != 2 {
		t.Fatalf("steps before Stop = %v, want 1, 2 and on", taken)
	}

	if len(steps) != len(taken) {
		t.Fatalf("steps after Stop = %v, want none past %v", steps, taken)
	}
}
