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

// writeEnvStep is an exec step that writes the variable name to the file the
// variable outVar names. The step reads the path from the environment rather
// than the command, so a space in it needs no quoting. Windows runs the step in
// PowerShell, which useEnvShell sets, because cmd.exe cannot read the quotes Go
// escapes around an argument.
func writeEnvStep(name, outVar string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf(
			"exec Set-Content -NoNewline -LiteralPath $env:%s -Value $env:%s",
			outVar,
			name,
		)
	}

	return fmt.Sprintf("exec printf '%%s' \"$%s\" > \"$%s\"", name, outVar)
}

// useEnvShell sets the shell writeEnvStep's steps are written for.
func useEnvShell(cfg *config.Config) {
	if runtime.GOOS == "windows" {
		cfg.General.ExecShell = filepath.Join(
			os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe",
		)
		cfg.General.ExecShellArgs = []string{"-NoProfile", "-NonInteractive", "-Command"}
	}
}

// TestExecutor_Run_HookEnvReachesExecSteps pins that a hook's payload reaches
// its exec steps, directly and through a macro, and that a hook's exec step
// runs while Neru is stopped, since the hooks that fire then report the pause.
func TestExecutor_Run_HookEnvReachesExecSteps(t *testing.T) {
	t.Parallel()

	// A space in the path is the case a shell is most likely to get wrong.
	dir := filepath.Join(t.TempDir(), "with space")

	mkdirErr := os.Mkdir(dir, 0o700)
	if mkdirErr != nil {
		t.Fatalf("creating %s: %v", dir, mkdirErr)
	}

	direct := filepath.Join(dir, "direct")
	nested := filepath.Join(dir, "nested")

	cfg := config.DefaultConfig()
	useEnvShell(cfg)
	cfg.Macros = map[string]config.StringOrStringArray{
		"record": {writeEnvStep("NERU_MODE", "NERU_TEST_NESTED")},
	}

	executor := sequence.NewExecutor(sequence.ExecutorDeps{
		Config:      func() *config.Config { return cfg },
		BaseContext: context.Background,
		Enabled:     func() bool { return false },
	})

	ctx := sequence.WithHook(context.Background(), []string{
		"NERU_MODE=hints",
		"NERU_TEST_DIRECT=" + direct,
		"NERU_TEST_NESTED=" + nested,
	})

	outcome := executor.Run(ctx, "hooks.on_mode_enter", []string{
		writeEnvStep("NERU_MODE", "NERU_TEST_DIRECT"),
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
