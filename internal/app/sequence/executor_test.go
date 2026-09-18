package sequence_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/y3owk1n/neru/internal/app/sequence"
	"github.com/y3owk1n/neru/internal/config"
)

// TestExecutor_Run_SkipsExecStepWhileStopped pins that `neru stop` reaches a
// sequence already under way: an exec step never passes through the command
// handler that refuses everything else, so the executor asks for itself.
func TestExecutor_Run_SkipsExecStepWhileStopped(t *testing.T) {
	t.Parallel()

	marker := filepath.Join(t.TempDir(), "ran")

	executor := sequence.NewExecutor(sequence.ExecutorDeps{
		Config:      config.DefaultConfig,
		BaseContext: context.Background,
		Enabled:     func() bool { return false },
	})

	outcome := executor.Run(context.Background(), "test", []string{"exec touch " + marker})

	if outcome.Err == nil {
		t.Fatal("exec step while stopped reported no error")
	}

	_, statErr := os.Stat(marker)
	if statErr == nil {
		t.Fatal("exec step ran while Neru was stopped")
	}
}
