package sequence_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

// writeEnvStep is an exec step that writes one environment variable to path,
// in the default shell of the platform the test runs on. The Windows path is
// left unquoted: cmd.exe does not read the escaped quotes Go puts around an
// argument, and a temp directory path has no spaces to protect.
func writeEnvStep(name, path string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("exec echo %%%s%%> %s", name, path)
	}

	return fmt.Sprintf("exec printf '%%s' \"$%s\" > %q", name, path)
}

// TestExecutor_Run_HookEnvReachesExecSteps pins that a hook's payload reaches
// its exec steps, directly and through a macro, and that a hook's exec step
// runs while Neru is stopped, since the hooks that fire then report the pause.
func TestExecutor_Run_HookEnvReachesExecSteps(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	direct := filepath.Join(dir, "direct")
	nested := filepath.Join(dir, "nested")

	cfg := config.DefaultConfig()
	cfg.Macros = map[string]config.StringOrStringArray{
		"record": {writeEnvStep("NERU_MODE", nested)},
	}

	executor := sequence.NewExecutor(sequence.ExecutorDeps{
		Config:      func() *config.Config { return cfg },
		BaseContext: context.Background,
		Enabled:     func() bool { return false },
	})

	ctx := sequence.WithHook(context.Background(), []string{"NERU_MODE=hints"})

	outcome := executor.Run(ctx, "hooks.on_mode_enter", []string{
		writeEnvStep("NERU_MODE", direct),
		"macro record",
	})
	if outcome.Err != nil {
		t.Fatalf("hook sequence failed: %v", outcome.Err)
	}

	for _, path := range []string{direct, nested} {
		got, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("exec step never wrote %s: %v", filepath.Base(path), readErr)
		}

		if strings.TrimSpace(string(got)) != "hints" {
			t.Errorf("%s saw NERU_MODE=%q, want hints", filepath.Base(path), got)
		}
	}
}
