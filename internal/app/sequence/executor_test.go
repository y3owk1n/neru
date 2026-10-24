package sequence_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/app/sequence"
	"github.com/y3owk1n/neru/internal/config"
)

const goosWindows = "windows"

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

// TestExecutor_Run_ExecStepReturnsWithoutWaitingOnBackgroundChild pins that a
// step ends when its shell exits. A background child inherits the step's
// output. Reading that through a pipe blocked the step until the child exited
// and kept every byte the child wrote in memory.
func TestExecutor_Run_ExecStepReturnsWithoutWaitingOnBackgroundChild(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == goosWindows {
		t.Skip("cmd.exe has no & to leave a child running in the background")
	}

	executor := sequence.NewExecutor(sequence.ExecutorDeps{
		Config:      config.DefaultConfig,
		BaseContext: context.Background,
		Enabled:     func() bool { return true },
	})

	started := time.Now()
	outcome := executor.Run(context.Background(), "test", []string{"exec sleep 10 &"})

	if outcome.Err != nil {
		t.Fatalf("exec step failed: %v", outcome.Err)
	}

	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Fatalf("exec step waited %s for its background child", elapsed)
	}
}

// writeEnvStep is an exec step that writes the variable name to the file the
// variable outVar names. The step reads the path from the environment rather
// than the command, so a space in it needs no quoting. Windows runs the step in
// PowerShell, which useEnvShell sets, because cmd.exe cannot read the quotes Go
// escapes around an argument.
func writeEnvStep(name, outVar string) string {
	if runtime.GOOS == goosWindows {
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
	if runtime.GOOS == goosWindows {
		cfg.General.Shell.Windows = []string{
			filepath.Join(
				os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe",
			),
			"-NoProfile", "-NonInteractive", "-Command",
		}
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

// TestExecutor_Run_EnvReachesExecStepsButNotWhileStopped pins what a mode's
// --on-exit gets: the selection reaches its exec steps, directly and through a
// macro, but the environment is not a hook's, so Neru stopped still stops them.
func TestExecutor_Run_EnvReachesExecStepsButNotWhileStopped(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		enabled   bool
		wantValue string
	}{
		{"running", true, "120"},
		{"stopped", false, ""},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			direct := filepath.Join(dir, "direct")
			nested := filepath.Join(dir, "nested")

			cfg := config.DefaultConfig()
			useEnvShell(cfg)
			cfg.Macros = map[string]config.StringOrStringArray{
				"record": {writeEnvStep("NERU_X", "NERU_TEST_NESTED")},
			}

			executor := sequence.NewExecutor(sequence.ExecutorDeps{
				Config:      func() *config.Config { return cfg },
				BaseContext: context.Background,
				Enabled:     func() bool { return testCase.enabled },
			})

			ctx := sequence.WithEnv(context.Background(), []string{
				"NERU_X=120",
				"NERU_TEST_DIRECT=" + direct,
				"NERU_TEST_NESTED=" + nested,
			})

			executor.Run(ctx, "on-exit", []string{
				writeEnvStep("NERU_X", "NERU_TEST_DIRECT"),
				"macro record",
			})

			for _, path := range []string{direct, nested} {
				got, readErr := os.ReadFile(path)
				if testCase.wantValue == "" {
					if readErr == nil {
						t.Errorf("%s ran while Neru was stopped", filepath.Base(path))
					}

					continue
				}

				if readErr != nil {
					t.Fatalf("exec step never wrote %s: %v", filepath.Base(path), readErr)
				}

				if strings.TrimSpace(string(got)) != testCase.wantValue {
					t.Errorf(
						"%s saw NERU_X=%q, want %s",
						filepath.Base(path),
						got,
						testCase.wantValue,
					)
				}
			}
		})
	}
}
