package recursivegrid_test

import (
	"image"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/recursivegrid"
)

// The two callback names a selectionEvent can carry.
const (
	updateCallback   = "update"
	completeCallback = "complete"
)

// selectionEvent is one callback firing, recorded with the callback that fired
// it so that a transposition shows up as the wrong name rather than as a
// missing call.
type selectionEvent struct {
	callback string
	point    image.Point
}

// recordSelections returns callbacks that append to log in the order they fire.
func recordSelections(log *[]selectionEvent) recursivegrid.SelectionCallbacks {
	return recursivegrid.SelectionCallbacks{
		OnUpdate: func(point image.Point) {
			*log = append(*log, selectionEvent{callback: updateCallback, point: point})
		},
		OnComplete: func(point image.Point) {
			*log = append(*log, selectionEvent{callback: completeCallback, point: point})
		},
	}
}

// TestManager_HandleInput_SelectionCallbacksAreNotInterchangeable pins which of
// the two selection callbacks fires for which event. Both take a point and
// neither returns anything, so wiring the update handler where the complete
// handler belongs still compiles once they are inside a struct; what separates
// them is *when* they fire. This records both in one ordered log so a
// transposition is a failure rather than a silently different overlay.
func TestManager_HandleInput_SelectionCallbacksAreNotInterchangeable(t *testing.T) {
	var log []selectionEvent

	manager := recursivegrid.NewManagerWithLayers(
		image.Rect(0, 0, 100, 100),
		"uijk",
		50, 50, 10, // min size 50 leaves exactly one refinement before the resolve
		domain.GridDimensions{Rows: 2, Cols: 2},
		nil, nil,
		recordSelections(&log),
		zap.NewNop(),
	)

	// A refinement: the 100x100 root narrows to its top-left 50x50 cell.
	_, completed := manager.HandleInput("u")
	require.False(t, completed, "the first selection refines rather than resolves")

	// That 50x50 cell cannot divide (halving it falls under the 50px minimum),
	// so the next keystroke resolves the selection.
	_, completed = manager.HandleInput("k")
	require.True(t, completed, "the selection at the final depth resolves")

	assert.Equal(t, []selectionEvent{
		{callback: updateCallback, point: image.Point{X: 25, Y: 25}},
		{callback: completeCallback, point: image.Point{X: 38, Y: 38}},
	}, log, "a refinement is an update and a resolution is a completion; neither is the other")
}

// TestManager_ZoomToPoint_SelectionCallbacksAreNotInterchangeable pins the same
// distinction on the manager's other caller of the pair, which chooses between
// the two on the same isComplete answer.
func TestManager_ZoomToPoint_SelectionCallbacksAreNotInterchangeable(t *testing.T) {
	newManager := func(log *[]selectionEvent) *recursivegrid.Manager {
		return recursivegrid.NewManagerWithLayers(
			image.Rect(0, 0, 100, 100),
			"uijk",
			50, 50, 10,
			domain.GridDimensions{Rows: 2, Cols: 2},
			nil, nil,
			recordSelections(log),
			zap.NewNop(),
		)
	}

	t.Run("stopping short of the target depth is an update", func(t *testing.T) {
		var log []selectionEvent

		_, completed := newManager(&log).ZoomToPoint(image.Point{X: 10, Y: 10}, 1)

		require.False(t, completed, "depth 1 is reachable, so the zoom has not resolved")
		assert.Equal(t, []selectionEvent{
			{callback: updateCallback, point: image.Point{X: 25, Y: 25}},
		}, log)
	})

	t.Run("running out of divisions is a completion", func(t *testing.T) {
		var log []selectionEvent

		// Depth 2 is unreachable: the 50x50 cell at depth 1 cannot divide, so
		// the zoom stops there and resolves.
		_, completed := newManager(&log).ZoomToPoint(image.Point{X: 10, Y: 10}, 2)

		require.True(t, completed, "the zoom ran out of divisions before the target depth")
		assert.Equal(t, []selectionEvent{
			{callback: completeCallback, point: image.Point{X: 25, Y: 25}},
		}, log)
	})
}
