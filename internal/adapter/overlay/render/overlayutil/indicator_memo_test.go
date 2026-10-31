package overlayutil_test

import (
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/overlayutil"
)

var start = time.Unix(1_000, 0)

func TestIndicatorMemo_BeginDraw(t *testing.T) {
	t.Parallel()

	draw := func(memo *overlayutil.IndicatorMemo[int], key int) {
		_, epoch := memo.BeginDraw(key)
		memo.EndDraw(key, epoch)
	}

	tests := []struct {
		name          string
		steps         func(memo *overlayutil.IndicatorMemo[int])
		wantUnchanged bool
	}{
		{
			name:          "repeat draw on a shown window is skipped",
			steps:         func(memo *overlayutil.IndicatorMemo[int]) { memo.Show(start); draw(memo, 1) },
			wantUnchanged: true,
		},
		{
			name:  "a different key is drawn",
			steps: func(memo *overlayutil.IndicatorMemo[int]) { memo.Show(start); draw(memo, 2) },
		},
		{
			name:  "a hidden window is drawn",
			steps: func(memo *overlayutil.IndicatorMemo[int]) { draw(memo, 1) },
		},
		{
			name: "hide forgets the draw",
			steps: func(memo *overlayutil.IndicatorMemo[int]) {
				memo.Show(start)
				draw(memo, 1)
				memo.Hide()
				memo.Show(start)
			},
		},
		{
			name: "clear forgets the draw",
			steps: func(memo *overlayutil.IndicatorMemo[int]) {
				memo.Show(start)
				draw(memo, 1)
				memo.Clear()
			},
		},
		{
			name: "invalidate forgets the draw",
			steps: func(memo *overlayutil.IndicatorMemo[int]) {
				memo.Show(start)
				draw(memo, 1)
				memo.Invalidate()
			},
		},
		{
			name: "a draw that raced an invalidation is not remembered",
			steps: func(memo *overlayutil.IndicatorMemo[int]) {
				memo.Show(start)

				_, epoch := memo.BeginDraw(1)
				memo.Invalidate()
				memo.EndDraw(1, epoch)
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var memo overlayutil.IndicatorMemo[int]
			testCase.steps(&memo)

			if unchanged, _ := memo.BeginDraw(1); unchanged != testCase.wantUnchanged {
				t.Errorf("BeginDraw(1) unchanged = %v, want %v", unchanged, testCase.wantUnchanged)
			}
		})
	}
}

func TestIndicatorMemo_HideAndClear(t *testing.T) {
	t.Parallel()

	var memo overlayutil.IndicatorMemo[int]

	if memo.Hide() || memo.Clear() {
		t.Fatal("a window that was never shown needs no hide or clear")
	}

	memo.Show(start)

	if !memo.Clear() {
		t.Error("Clear() on a shown window = false, want true")
	}

	if !memo.Hide() {
		t.Error("Hide() on a shown window = false, want true")
	}

	if memo.Hide() || memo.Clear() {
		t.Error("a hidden window needs no second hide or clear")
	}
}

func TestIndicatorMemo_Show(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		steps func(memo *overlayutil.IndicatorMemo[int])
		at    time.Time
		want  bool
	}{
		{
			name:  "a window that was never shown needs a show",
			steps: func(*overlayutil.IndicatorMemo[int]) {},
			at:    start,
			want:  true,
		},
		{
			name:  "a shown window needs none within the interval",
			steps: func(memo *overlayutil.IndicatorMemo[int]) { memo.Show(start) },
			at:    start.Add(overlayutil.IndicatorReassertInterval - time.Millisecond),
		},
		{
			name:  "a shown window needs one once the interval passes",
			steps: func(memo *overlayutil.IndicatorMemo[int]) { memo.Show(start) },
			at:    start.Add(overlayutil.IndicatorReassertInterval),
			want:  true,
		},
		{
			name: "a hidden window needs one at once",
			steps: func(memo *overlayutil.IndicatorMemo[int]) {
				memo.Show(start)
				memo.Hide()
			},
			at:   start.Add(time.Millisecond),
			want: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var memo overlayutil.IndicatorMemo[int]
			testCase.steps(&memo)

			if got := memo.Show(testCase.at); got != testCase.want {
				t.Errorf("Show() = %v, want %v", got, testCase.want)
			}
		})
	}
}
