package platform_test

import (
	"context"
	"runtime"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/platform"
	"github.com/y3owk1n/neru/internal/derrors"
)

// An already-canceled context makes the launch fail before anything is
// spawned, so this runs everywhere without opening a browser. The shipped
// platforms report the failed launch, and the fallback slot refuses.
func TestOpenExternalReportsALaunchFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := platform.OpenExternal(ctx, "https://example.invalid")
	if err == nil {
		t.Fatal("OpenExternal() = nil, want an error when the handler cannot launch")
	}

	wantCode := derrors.CodeNotSupported

	switch runtime.GOOS {
	case string(platform.Darwin), string(platform.Linux), string(platform.Windows):
		wantCode = derrors.CodeExecFailed
	}

	if !derrors.IsCode(err, wantCode) {
		t.Errorf("OpenExternal() error = %v, want code %s", err, wantCode)
	}
}
