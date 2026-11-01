package element_test

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/element"
)

func TestNearest(t *testing.T) {
	screen := image.Rect(0, 0, 1000, 1000)

	tests := []struct {
		name      string
		bounds    []image.Rectangle
		origin    image.Point
		region    image.Rectangle
		want      image.Point
		wantFound bool
	}{
		{
			name:      "nearest edge wins",
			bounds:    []image.Rectangle{image.Rect(300, 0, 340, 40), image.Rect(120, 0, 160, 40)},
			origin:    image.Pt(100, 20),
			region:    screen,
			want:      image.Pt(140, 20),
			wantFound: true,
		},
		{
			name: "containing beats a nearer edge",
			bounds: []image.Rectangle{
				image.Rect(101, 0, 141, 40),
				image.Rect(0, 500, 1000, 600),
			},
			origin:    image.Pt(100, 550),
			region:    screen,
			want:      image.Pt(500, 550),
			wantFound: true,
		},
		{
			name: "smallest containing wins, so a button beats its row",
			bounds: []image.Rectangle{
				image.Rect(0, 100, 1000, 140),
				image.Rect(80, 105, 120, 135),
			},
			origin:    image.Pt(100, 120),
			region:    screen,
			want:      image.Pt(100, 120),
			wantFound: true,
		},
		{
			name: "equal distance goes to the smaller",
			bounds: []image.Rectangle{
				image.Rect(200, 0, 400, 200),
				image.Rect(200, 90, 210, 110),
			},
			origin:    image.Pt(100, 100),
			region:    screen,
			want:      image.Pt(205, 100),
			wantFound: true,
		},
		{
			name:      "an element outside the region is ignored however near",
			bounds:    []image.Rectangle{image.Rect(510, 0, 550, 40), image.Rect(0, 0, 40, 40)},
			origin:    image.Pt(490, 20),
			region:    image.Rect(0, 0, 500, 500),
			want:      image.Pt(20, 20),
			wantFound: true,
		},
		{
			name:      "a wide element lands on its part inside the region",
			bounds:    []image.Rectangle{image.Rect(0, 100, 1000, 140)},
			origin:    image.Pt(50, 50),
			region:    image.Rect(0, 0, 200, 200),
			want:      image.Pt(100, 120),
			wantFound: true,
		},
		{
			name: "a row covering the whole region loses to the button inside it",
			bounds: []image.Rectangle{
				image.Rect(0, 0, 1000, 300),
				image.Rect(20, 20, 60, 60),
			},
			origin:    image.Pt(100, 100),
			region:    image.Rect(0, 0, 200, 200),
			want:      image.Pt(40, 40),
			wantFound: true,
		},
		{
			name:      "a container alone in the region is still a target",
			bounds:    []image.Rectangle{image.Rect(0, 0, 1000, 300)},
			origin:    image.Pt(100, 100),
			region:    image.Rect(0, 0, 200, 200),
			want:      image.Pt(100, 100),
			wantFound: true,
		},
		{
			name: "a same-row element beats a slightly nearer one in the next row",
			bounds: []image.Rectangle{
				image.Rect(860, 440, 900, 460),
				image.Rect(790, 490, 810, 510),
			},
			origin:    image.Pt(800, 450),
			region:    image.Rect(0, 0, 1600, 900),
			want:      image.Pt(880, 450),
			wantFound: true,
		},
		{
			name: "an icon right above beats a toolbar button far to the side",
			bounds: []image.Rectangle{
				image.Rect(790, 425, 810, 435),
				image.Rect(900, 440, 940, 460),
			},
			origin:    image.Pt(800, 450),
			region:    image.Rect(0, 0, 1600, 900),
			want:      image.Pt(800, 430),
			wantFound: true,
		},
		{
			name: "a very wide cell weighs vertical drift at most three times",
			bounds: []image.Rectangle{
				image.Rect(1900, 40, 1940, 60),
				image.Rect(1490, 85, 1510, 95),
			},
			origin:    image.Pt(1500, 50),
			region:    image.Rect(0, 0, 3000, 100),
			want:      image.Pt(1500, 90),
			wantFound: true,
		},
		{
			name: "a tall cell never makes vertical drift cheaper than horizontal",
			bounds: []image.Rectangle{
				image.Rect(80, 495, 90, 505),
				image.Rect(45, 535, 55, 545),
			},
			origin:    image.Pt(50, 500),
			region:    image.Rect(0, 0, 100, 1000),
			want:      image.Pt(85, 500),
			wantFound: true,
		},
		{
			name:   "nothing overlapping the region finds nothing",
			bounds: []image.Rectangle{image.Rect(600, 600, 640, 640)},
			origin: image.Pt(50, 50),
			region: image.Rect(0, 0, 200, 200),
		},
		{
			name:   "no elements finds nothing",
			origin: image.Pt(50, 50),
			region: screen,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			elements := buildElements(t, testCase.bounds)

			got, found := element.Nearest(elements, testCase.origin, testCase.region)
			if found != testCase.wantFound || got.Point != testCase.want {
				t.Errorf("Nearest() = %v, %v, want %v, %v",
					got.Point, found, testCase.want, testCase.wantFound)
			}
		})
	}
}

func TestToward(t *testing.T) {
	screen := image.Rect(0, 0, 1000, 1000)
	// A row of three buttons inside a toolbar, and a field below the first.
	toolbar := image.Rect(0, 0, 1000, 60)
	first := image.Rect(10, 10, 50, 50)
	second := image.Rect(60, 10, 100, 50)
	third := image.Rect(400, 10, 440, 50)
	field := image.Rect(10, 80, 300, 110)
	row := []image.Rectangle{toolbar, first, second, third, field}

	tests := []struct {
		name      string
		bounds    []image.Rectangle
		origin    image.Point
		region    image.Rectangle
		dir       domain.Direction
		want      image.Point
		wantFound bool
	}{
		{
			name:      "right steps to the next button, not the toolbar around it",
			bounds:    row,
			origin:    image.Pt(30, 30),
			region:    screen,
			dir:       domain.DirectionRight,
			want:      image.Pt(80, 30),
			wantFound: true,
		},
		{
			name:      "right from the second button skips the gap to the third",
			bounds:    row,
			origin:    image.Pt(80, 30),
			region:    screen,
			dir:       domain.DirectionRight,
			want:      image.Pt(420, 30),
			wantFound: true,
		},
		{
			name:      "left steps back",
			bounds:    row,
			origin:    image.Pt(80, 30),
			region:    screen,
			dir:       domain.DirectionLeft,
			want:      image.Pt(30, 30),
			wantFound: true,
		},
		{
			name:      "right from the toolbar's empty space steps to the button that way",
			bounds:    row,
			origin:    image.Pt(200, 30),
			region:    screen,
			dir:       domain.DirectionRight,
			want:      image.Pt(420, 30),
			wantFound: true,
		},
		{
			name:      "left from the toolbar's empty space steps to the nearest button back",
			bounds:    row,
			origin:    image.Pt(200, 30),
			region:    screen,
			dir:       domain.DirectionLeft,
			want:      image.Pt(80, 30),
			wantFound: true,
		},
		{
			name:      "down leaves the toolbar for the field below",
			bounds:    row,
			origin:    image.Pt(30, 30),
			region:    screen,
			dir:       domain.DirectionDown,
			want:      image.Pt(155, 95),
			wantFound: true,
		},
		{
			name: "right keeps to the row over a nearer element below it",
			bounds: []image.Rectangle{
				image.Rect(10, 10, 50, 50),
				image.Rect(300, 10, 340, 50),
				image.Rect(60, 70, 100, 110),
			},
			origin:    image.Pt(30, 30),
			region:    screen,
			dir:       domain.DirectionRight,
			want:      image.Pt(320, 30),
			wantFound: true,
		},
		{
			name:      "from empty space it steps from the point itself",
			bounds:    []image.Rectangle{image.Rect(200, 480, 240, 520)},
			origin:    image.Pt(100, 500),
			region:    screen,
			dir:       domain.DirectionRight,
			want:      image.Pt(220, 500),
			wantFound: true,
		},
		{
			name: "down lands on the list's first item, not the list",
			bounds: []image.Rectangle{
				image.Rect(10, 10, 50, 50),
				image.Rect(0, 100, 300, 400),
				image.Rect(10, 110, 290, 140),
				image.Rect(10, 150, 290, 180),
			},
			origin:    image.Pt(30, 30),
			region:    screen,
			dir:       domain.DirectionDown,
			want:      image.Pt(150, 125),
			wantFound: true,
		},
		{
			name:   "nothing that way finds nothing",
			bounds: row,
			origin: image.Pt(420, 30),
			region: screen,
			dir:    domain.DirectionUp,
		},
		{
			name:   "an element past the region edge is not a step",
			bounds: []image.Rectangle{image.Rect(10, 10, 50, 50), image.Rect(210, 10, 250, 50)},
			origin: image.Pt(30, 30),
			region: image.Rect(0, 0, 200, 200),
			dir:    domain.DirectionRight,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			elements := buildElements(t, testCase.bounds)

			got, found := element.Toward(elements, testCase.origin, testCase.region, testCase.dir)
			if found != testCase.wantFound || got.Point != testCase.want {
				t.Errorf("Toward() = %v, %v, want %v, %v",
					got.Point, found, testCase.want, testCase.wantFound)
			}
		})
	}
}

func buildElements(t *testing.T, bounds []image.Rectangle) []*element.Element {
	t.Helper()

	elements := make([]*element.Element, len(bounds))
	for idx, rect := range bounds {
		elem, err := element.NewElement(element.ID(rune('a'+idx)), rect, element.RoleButton)
		if err != nil {
			t.Fatalf("build element %v: %v", rect, err)
		}

		elements[idx] = elem
	}

	return elements
}
