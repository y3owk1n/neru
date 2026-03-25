//nolint:testpackage // Tests private mode handler methods.
package modes

import (
	"image"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/domain/state"
)

func TestExecuteActionAtPoint_NilActionNoop(t *testing.T) {
	handler := &Handler{
		logger:      zap.NewNop(),
		cursorState: state.NewCursorState(),
	}

	handler.executeActionAtPoint(nil, point(10, 10), false, nil)

	if handler.cursorState.WasActionPerformed() {
		t.Fatal("expected no action state change for nil action")
	}
}

func point(x, y int) image.Point {
	return image.Point{X: x, Y: y}
}
