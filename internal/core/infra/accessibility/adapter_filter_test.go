package accessibility_test

import (
	"context"
	"image"
	"slices"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/domain/element"
	"github.com/y3owk1n/neru/internal/core/infra/accessibility"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// IDs of the fixed node set the filter cases run against.
const (
	idBigButton    = "big-button"
	idTinyButton   = "tiny-button"
	idBigLink      = "big-link"
	idWideFlatLink = "wide-flat-link"
)

// filterTestNodes is the fixed cast of AX nodes every filter case runs against.
// Each node is chosen so that at least one filter field can single it out, and
// so that role, size and text constraints are independent of one another.
func filterTestNodes() []accessibility.AXNode {
	return []accessibility.AXNode{
		&accessibility.MockNode{
			MockID:        idBigButton,
			MockBounds:    image.Rect(0, 0, 100, 40),
			MockRole:      axButtonRole,
			MockTitle:     "Save",
			MockClickable: true,
		},
		&accessibility.MockNode{
			MockID:        idTinyButton,
			MockBounds:    image.Rect(0, 0, 4, 4),
			MockRole:      axButtonRole,
			MockTitle:     "x",
			MockClickable: true,
		},
		&accessibility.MockNode{
			MockID:        idBigLink,
			MockBounds:    image.Rect(0, 0, 200, 30),
			MockRole:      axLinkRole,
			MockTitle:     "Documentation",
			MockClickable: true,
		},
		&accessibility.MockNode{
			MockID:        idWideFlatLink,
			MockBounds:    image.Rect(0, 0, 300, 4),
			MockRole:      axLinkRole,
			MockTitle:     "Divider",
			MockClickable: true,
		},
	}
}

// newFilterAdapter builds an adapter whose frontmost window yields
// filterTestNodes, so ClickableElements exercises the real collection and
// filtering path against a known, non-empty element set.
func newFilterAdapter(t *testing.T) *accessibility.Adapter {
	t.Helper()

	window := &accessibility.MockWindow{}
	client := &accessibility.MockAXClient{
		MockPermissions:     true,
		MockFrontmostWindow: window,
		MockAllWindows:      []accessibility.AXWindow{window},
		MockClickableNodes:  filterTestNodes(),
	}

	return accessibility.NewAdapter(zap.NewNop(), nil, nil, client, false)
}

// idsOf returns element IDs in the order the adapter produced them.
func idsOf(elements []*element.Element) []string {
	ids := make([]string, 0, len(elements))
	for _, el := range elements {
		ids = append(ids, string(el.ID()))
	}

	slices.Sort(ids)

	return ids
}

// TestAdapter_ClickableElements_FilterContract pins what ClickableElements
// actually returns for a given filter, not merely which roles it forwarded to
// the client. The existing role-plumbing tests discard the result slice, so
// without this a regression that collected the right nodes and then dropped or
// mis-filtered them would go unnoticed.
func TestAdapter_ClickableElements_FilterContract(t *testing.T) {
	tests := []struct {
		name    string
		filter  ports.ElementFilter
		wantIDs []string
	}{
		{
			name:    "no constraints returns every clickable node",
			filter:  ports.ElementFilter{},
			wantIDs: []string{idBigButton, idBigLink, idTinyButton, idWideFlatLink},
		},
		{
			name: "MinSize drops nodes shorter or narrower than the minimum",
			filter: ports.ElementFilter{
				MinSize: image.Point{X: 10, Y: 10},
			},
			wantIDs: []string{idBigButton, idBigLink},
		},
		{
			name: "MinSize compares both axes, not area",
			filter: ports.ElementFilter{
				// wide-flat-link is 300x4: far larger in area than the
				// 10x10 minimum, but too short. An implementation that
				// compared areas would wrongly keep it.
				MinSize: image.Point{X: 10, Y: 10},
			},
			wantIDs: []string{idBigButton, idBigLink},
		},
		{
			name: "Roles acts as an allowlist",
			filter: ports.ElementFilter{
				Roles: []element.Role{element.RoleButton},
			},
			wantIDs: []string{idBigButton, idTinyButton},
		},
		{
			name: "ExcludeRoles removes the named roles",
			filter: ports.ElementFilter{
				ExcludeRoles: []element.Role{element.RoleLink},
			},
			wantIDs: []string{idBigButton, idTinyButton},
		},
		{
			name: "ExcludeRoles wins over Roles for the same role",
			filter: ports.ElementFilter{
				Roles:        []element.Role{element.RoleButton, element.RoleLink},
				ExcludeRoles: []element.Role{element.RoleLink},
			},
			wantIDs: []string{idBigButton, idTinyButton},
		},
		{
			name: "Roles and MinSize both apply",
			filter: ports.ElementFilter{
				Roles:   []element.Role{element.RoleButton},
				MinSize: image.Point{X: 10, Y: 10},
			},
			wantIDs: []string{idBigButton},
		},
		{
			name: "a role that matches nothing yields no elements",
			filter: ports.ElementFilter{
				Roles: []element.Role{element.Role("AXCheckBox")},
			},
			wantIDs: []string{},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			adapter := newFilterAdapter(t)

			got, err := adapter.ClickableElements(context.Background(), testCase.filter)
			if err != nil {
				t.Fatalf("ClickableElements() error = %v, want nil", err)
			}

			if gotIDs := idsOf(got); !slices.Equal(gotIDs, testCase.wantIDs) {
				t.Errorf("ClickableElements() returned %v, want %v", gotIDs, testCase.wantIDs)
			}
		})
	}
}

// TestAdapter_ClickableElements_PreservesNodeAttributes checks that the
// AXNode -> element.Element conversion carries every attribute across. A
// dropped bounds or role here would silently misplace every hint.
func TestAdapter_ClickableElements_PreservesNodeAttributes(t *testing.T) {
	adapter := newFilterAdapter(t)

	got, err := adapter.ClickableElements(context.Background(), ports.ElementFilter{
		Roles: []element.Role{element.RoleButton},
		// Single out exactly one node so the assertions below are unambiguous.
		MinSize: image.Point{X: 10, Y: 10},
	})
	if err != nil {
		t.Fatalf("ClickableElements() error = %v, want nil", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected exactly one element, got %d (%v)", len(got), idsOf(got))
	}

	found := got[0]

	if found.ID() != element.ID(idBigButton) {
		t.Errorf("ID() = %q, want %q", found.ID(), idBigButton)
	}

	if want := image.Rect(0, 0, 100, 40); found.Bounds() != want {
		t.Errorf("Bounds() = %v, want %v", found.Bounds(), want)
	}

	if found.Role() != element.RoleButton {
		t.Errorf("Role() = %q, want %q", found.Role(), element.RoleButton)
	}

	if found.Title() != "Save" {
		t.Errorf("Title() = %q, want %q", found.Title(), "Save")
	}

	// Center is what every click action ultimately targets.
	if want := (image.Point{X: 50, Y: 20}); found.Center() != want {
		t.Errorf("Center() = %v, want %v", found.Center(), want)
	}
}

// TestAdapter_ClickableElements_PropagatesClientError makes sure a failure from
// the underlying AX client surfaces instead of being reported as an empty but
// successful result — which would look to a caller like "nothing on screen".
func TestAdapter_ClickableElements_PropagatesClientError(t *testing.T) {
	window := &accessibility.MockWindow{}
	client := &accessibility.MockAXClient{
		MockPermissions:       true,
		MockFrontmostWindow:   window,
		MockAllWindows:        []accessibility.AXWindow{window},
		MockClickableNodesErr: errTestAccessibility,
	}

	adapter := accessibility.NewAdapter(zap.NewNop(), nil, nil, client, false)

	got, err := adapter.ClickableElements(context.Background(), ports.ElementFilter{})
	if err == nil {
		t.Fatalf(
			"ClickableElements() error = nil, want the client error; got %d elements",
			len(got),
		)
	}
}
