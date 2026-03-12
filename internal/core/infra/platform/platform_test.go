package platform_test

import (
	"runtime"
	"testing"

	"github.com/y3owk1n/neru/internal/core/infra/platform"
)

func TestCurrentOS(t *testing.T) {
	current := platform.CurrentOS()
	switch runtime.GOOS {
	case string(platform.Darwin):
		if current != platform.Darwin {
			t.Errorf("expected Darwin, got %s", current)
		}
	case string(platform.Linux):
		if current != platform.Linux {
			t.Errorf("expected Linux, got %s", current)
		}
	case string(platform.Windows):
		if current != platform.Windows {
			t.Errorf("expected Windows, got %s", current)
		}
	default:
		if current != platform.Unknown {
			t.Errorf("expected Unknown, got %s", current)
		}
	}
}

func TestIsDarwin(t *testing.T) {
	isDarwin := platform.IsDarwin()
	if runtime.GOOS == string(platform.Darwin) && !isDarwin {
		t.Error("expected IsDarwin to be true on darwin")
	}

	if runtime.GOOS != string(platform.Darwin) && isDarwin {
		t.Error("expected IsDarwin to be false on non-darwin")
	}
}
