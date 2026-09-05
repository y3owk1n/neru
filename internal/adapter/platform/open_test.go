package platform_test

import (
	"context"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/platform"
	"github.com/y3owk1n/neru/internal/derrors"
)

// An already-canceled context makes the launch fail before anything is
// spawned, so this runs on each shipped platform without opening a browser.
func TestOpenExternalReportsALaunchFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := platform.OpenExternal(ctx, "https://example.invalid")
	if err == nil {
		t.Fatal("OpenExternal() = nil, want an error when the handler cannot launch")
	}

	if !derrors.IsCode(err, derrors.CodeExecFailed) {
		t.Errorf("OpenExternal() error = %v, want code %s", err, derrors.CodeExecFailed)
	}
}
