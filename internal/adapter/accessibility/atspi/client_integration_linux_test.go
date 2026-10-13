//go:build integration && linux

package atspi

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"image"
	"maps"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/accessibility/ax"
	"github.com/y3owk1n/neru/internal/config"
)

// Both scan paths run against a fake application exported on a private
// dbus-daemon, so the state reads and the editable-text rule are checked in
// the walk and the Collection path themselves.

// errStateUnavailable is what a fake element whose state read fails returns.
var errStateUnavailable = errors.New("state unavailable")

// fakeAccessible answers the AT-SPI calls a scan makes for one element.
type fakeAccessible struct {
	ref             accRef
	role            string
	states          []uint32
	stateFails      bool
	children        []*fakeAccessible
	name            string
	description     string
	identifier      string
	text            string
	textReads       atomic.Int32
	identifierReads atomic.Int32
	// screenOrigin and windowOrigin are where the element sits in screen and
	// in window coordinates.
	screenOrigin image.Point
	windowOrigin image.Point
}

func (f *fakeAccessible) GetRoleName() (string, *dbus.Error) { return f.role, nil }

func (f *fakeAccessible) GetState() ([]uint32, *dbus.Error) {
	if f.stateFails {
		return nil, dbus.MakeFailedError(errStateUnavailable)
	}

	return f.states, nil
}

func (f *fakeAccessible) GetChildren() ([]accRef, *dbus.Error) {
	refs := make([]accRef, 0, len(f.children))
	for _, child := range f.children {
		refs = append(refs, child.ref)
	}

	return refs, nil
}

// Get answers org.freedesktop.DBus.Properties.Get for the properties a scan
// reads.
func (f *fakeAccessible) Get(_, property string) (dbus.Variant, *dbus.Error) {
	switch property {
	case "Description":
		return dbus.MakeVariant(f.description), nil
	case "AccessibleId":
		f.identifierReads.Add(1)

		return dbus.MakeVariant(f.identifier), nil
	default:
		return dbus.MakeVariant(f.name), nil
	}
}

func (f *fakeAccessible) GetText(int32, int32) (string, *dbus.Error) {
	f.textReads.Add(1)

	return f.text, nil
}

func (f *fakeAccessible) GetExtents(coordType uint32) (atspiExtents, *dbus.Error) {
	origin := f.screenOrigin
	if coordType == atspiCoordWindow {
		origin = f.windowOrigin
	}

	return atspiExtents{X: int32(origin.X), Y: int32(origin.Y), W: 10, H: 10}, nil
}

// GetMatches answers the Collection query as a toolkit does: the descendants
// at any depth whose role is in the rule and that have every state the rule
// names.
func (f *fakeAccessible) GetMatches(
	rule atspiMatchRule,
	count uint32,
	traverse int32,
	_ bool,
) ([]accRef, *dbus.Error) {
	var refs []accRef

	for _, child := range f.children {
		roleID := atspiRoleNameToID[child.role]
		inRule := rule.Roles[roleID/atspiBitsPerWord]&(1<<(roleID%atspiBitsPerWord)) != 0

		if inRule && hasStates(child.states, rule.States) {
			refs = append(refs, child.ref)
		}

		deeper, _ := child.GetMatches(rule, count, traverse, false)
		refs = append(refs, deeper...)
	}

	return refs, nil
}

func TestClient_ScanPaths_ReadEditableTextAsEntryAndSkipDisabled(t *testing.T) {
	address := startPrivateBus(t)

	app := connectBus(t, address)
	scanner := connectBus(t, address)

	child := fixtureChild(app.Names()[0])

	stateFails := child("state_fails", atspiRoleText,
		atspiStateShowing, atspiStateEnabled, atspiStateEditable)
	stateFails.stateFails = true

	frame := child("frame", "frame", atspiStateShowing)
	frame.children = []*fakeAccessible{
		child("editable", atspiRoleText, atspiStateShowing, atspiStateEnabled, atspiStateEditable),
		child("read_only", atspiRoleText, atspiStateShowing, atspiStateEnabled),
		child("hidden", atspiRoleText, atspiStateEnabled, atspiStateEditable),
		child("disabled", "push button", atspiStateShowing),
		stateFails,
		child("button", "push button", atspiStateShowing, atspiStateEnabled),
	}

	exportFixture(t, app, frame)
	scans := scanPaths(t, scanner, frame, false)

	tests := []struct {
		roles []string
		want  map[string]string
	}{
		{
			roles: []string{"text_field", "button"},
			want:  map[string]string{"editable": atspiRoleEntry, "button": "push button"},
		},
		{
			roles: []string{"static_text"},
			want:  map[string]string{"read_only": atspiRoleText},
		},
	}

	for path, scan := range scans {
		for _, test := range tests {
			t.Run(path+" "+strings.Join(test.roles, " "), func(t *testing.T) {
				roles := rolesSet(resolveLinuxRoles(t, test.roles...))

				got := make(map[string]string)
				for _, node := range scan(New(zap.NewNop(), nil), roles) {
					got[fixtureName(node)] = node.Role()
				}

				if !maps.Equal(got, test.want) {
					t.Errorf("hinted %v, want %v", got, test.want)
				}
			})
		}
	}
}

func TestClient_ScanPaths_ReadNameDescriptionAndValue(t *testing.T) {
	address := startPrivateBus(t)

	app := connectBus(t, address)
	scanner := connectBus(t, address)

	child := fixtureChild(app.Names()[0])

	entry := child(
		"search",
		atspiRoleText,
		atspiStateShowing,
		atspiStateEnabled,
		atspiStateEditable,
	)
	entry.name, entry.description, entry.text = "Search", "Find in page", "typed text"

	password := child("password", "password text",
		atspiStateShowing, atspiStateEnabled, atspiStateEditable)
	password.text = "hunter2"

	button := child("save", "push button", atspiStateShowing, atspiStateEnabled)
	button.name, button.description, button.text = "Save", "Save the file", "button text"

	frame := child("frame", "frame", atspiStateShowing)
	frame.children = []*fakeAccessible{entry, password, button}

	exportFixture(t, app, frame)

	want := map[string][3]string{
		"search":   {"Search", "Find in page", "typed text"},
		"password": {"", "", ""},
		"save":     {"Save", "Save the file", ""},
	}

	roles := rolesSet(resolveLinuxRoles(t, "text_field", "button"))

	for path, scan := range scanPaths(t, scanner, frame, false) {
		t.Run(path, func(t *testing.T) {
			got := make(map[string][3]string)
			for _, node := range scan(New(zap.NewNop(), nil), roles) {
				got[fixtureName(node)] = [3]string{node.Title(), node.Description(), node.Value()}
			}

			if !maps.Equal(got, want) {
				t.Errorf("read %v, want %v", got, want)
			}
		})
	}

	if reads := password.textReads.Load(); reads != 0 {
		t.Errorf("the password field's text was read %d times, want never", reads)
	}
}

func TestClient_Walk_HintsGTK4ElementsThatAreOnlySensitive(t *testing.T) {
	address := startPrivateBus(t)

	app := connectBus(t, address)
	scanner := connectBus(t, address)

	child := fixtureChild(app.Names()[0])

	// GTK 4 marks an element that takes input SENSITIVE and never ENABLED.
	frame := child("frame", "frame", atspiStateShowing)
	frame.children = []*fakeAccessible{
		child("sensitive", "button", atspiStateShowing, atspiStateSensitive),
		child("insensitive", "button", atspiStateShowing),
	}

	exportFixture(t, app, frame)

	var out []ax.Node

	visited := 0
	New(zap.NewNop(), nil).walk(t.Context(), scanner, frame.ref,
		rolesSet(resolveLinuxRoles(t, "button")), 0, atspiWalkDepthCeiling, &out, &visited,
		placement{}, false)

	got := make([]string, 0, len(out))
	for _, node := range out {
		got = append(got, fixtureName(node))
	}

	if !slices.Equal(got, []string{"sensitive"}) {
		t.Errorf("hinted %v, want [sensitive]", got)
	}
}

// fixedOrigin is a window-origin source that always gives the same answer.
type fixedOrigin struct {
	origin image.Point
	known  bool
}

func (fixedOrigin) start() {}

func (f fixedOrigin) originFor(windowFrame) (image.Point, bool, error) {
	return f.origin, f.known, nil
}

func TestClient_PlacementFor_PutsHintsOnTheScreen(t *testing.T) {
	address := startPrivateBus(t)

	app := connectBus(t, address)
	scanner := connectBus(t, address)

	child := fixtureChild(app.Names()[0])

	// A window at (646, 52) with a button 39 by 3 pixels into it. GTK 4
	// reports a zero origin in screen coordinates. An XWayland client reports
	// real screen positions there.
	window := image.Pt(646, 52)
	inside := image.Pt(39, 3)

	gtk4Frame := child("gtk4", "frame", atspiStateShowing)
	gtk4Button := child("gtk4_button", "button", atspiStateShowing, atspiStateSensitive)
	gtk4Button.windowOrigin = inside
	gtk4Frame.children = []*fakeAccessible{gtk4Button}

	xwaylandFrame := child("xwayland", "frame", atspiStateShowing)
	xwaylandFrame.screenOrigin = window
	xwaylandButton := child("xwayland_button", "button", atspiStateShowing, atspiStateEnabled)
	xwaylandButton.screenOrigin = window.Add(inside)
	xwaylandButton.windowOrigin = inside
	xwaylandFrame.children = []*fakeAccessible{xwaylandButton}

	// A toolkit that draws a shadow around its window puts the frame, and
	// every element with it, that far into its own window coordinates.
	shadow := image.Pt(26, 23)

	shadowFrame := child("shadow", "frame", atspiStateShowing)
	shadowFrame.windowOrigin = shadow
	shadowButton := child("shadow_button", "button", atspiStateShowing, atspiStateEnabled)
	shadowButton.windowOrigin = shadow.Add(inside)
	shadowFrame.children = []*fakeAccessible{shadowButton}

	exportFixture(t, app, gtk4Frame)
	exportFixture(t, app, xwaylandFrame)
	exportFixture(t, app, shadowFrame)

	tests := []struct {
		name   string
		frame  *fakeAccessible
		origin fixedOrigin
		want   image.Point
	}{
		{"gtk 4 with the window origin", gtk4Frame, fixedOrigin{window, true}, window.Add(inside)},
		{
			"xwayland with the window origin",
			xwaylandFrame,
			fixedOrigin{window, true},
			window.Add(inside),
		},
		{"xwayland without the window origin", xwaylandFrame, fixedOrigin{}, window.Add(inside)},
		{"a frame inset by a shadow", shadowFrame, fixedOrigin{window, true}, window.Add(inside)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := New(zap.NewNop(), nil)
			client.windowOrigin = test.origin

			place, _ := client.placementFor(t.Context(), scanner, &atspiWindow{ref: test.frame.ref})

			var out []ax.Node

			visited := 0
			client.walk(t.Context(), scanner, test.frame.ref,
				rolesSet(resolveLinuxRoles(t, "button")), 0, atspiWalkDepthCeiling, &out, &visited,
				place, false)

			got := make([]image.Point, 0, len(out))
			for _, node := range out {
				got = append(got, node.Bounds().Min)
			}

			if !slices.Equal(got, []image.Point{test.want}) {
				t.Errorf("hinted at %v, want one hint at %v", got, test.want)
			}
		})
	}
}

// hasStates reports whether states has every bit set in want.
func hasStates(states []uint32, want []int32) bool {
	for word, bits := range want {
		if word >= len(states) || states[word]&uint32(bits) != uint32(bits) {
			return false
		}
	}

	return true
}

// fixtureChild returns a constructor for fake elements owned by busName.
func fixtureChild(busName string) func(string, string, ...uint) *fakeAccessible {
	return func(name, role string, states ...uint) *fakeAccessible {
		return &fakeAccessible{
			ref:    accRef{Name: busName, Path: dbus.ObjectPath("/fixture/" + name)},
			role:   role,
			states: stateBitsFor(states...),
		}
	}
}

func TestClient_ScanPaths_ReadElementStateOnlyWhenAsked(t *testing.T) {
	address := startPrivateBus(t)

	app := connectBus(t, address)
	scanner := connectBus(t, address)

	child := fixtureChild(app.Names()[0])

	entry := child("field", atspiRoleText,
		atspiStateShowing, atspiStateEditable, atspiStateEnabled, atspiStateFocused)
	entry.identifier = "search-field"

	button := child("action", "push button", atspiStateShowing, atspiStateEnabled)
	button.identifier = "save-button"

	frame := child("frame", "frame", atspiStateShowing)
	frame.children = []*fakeAccessible{entry, button}

	exportFixture(t, app, frame)

	type state struct {
		identifier string
		focused    bool
	}

	tests := []struct {
		readState bool
		want      map[string]state
	}{
		{
			readState: true,
			want: map[string]state{
				"field":  {"search-field", true},
				"action": {"save-button", false},
			},
		},
		{
			readState: false,
			want: map[string]state{
				"field":  {"", false},
				"action": {"", false},
			},
		},
	}

	roles := rolesSet(resolveLinuxRoles(t, "text_field", "button"))

	for _, test := range tests {
		for path, scan := range scanPaths(t, scanner, frame, test.readState) {
			t.Run(fmt.Sprintf("%s read state %v", path, test.readState), func(t *testing.T) {
				reads := entry.identifierReads.Load() + button.identifierReads.Load()

				got := make(map[string]state)
				for _, node := range scan(New(zap.NewNop(), nil), roles) {
					got[fixtureName(node)] = state{node.Identifier(), node.IsFocused()}
				}

				if !maps.Equal(got, test.want) {
					t.Errorf("read %v, want %v", got, test.want)
				}

				reads = entry.identifierReads.Load() + button.identifierReads.Load() - reads
				if !test.readState && reads != 0 {
					t.Errorf("the scan read %d identifiers, want none without read state", reads)
				}
			})
		}
	}
}

func TestClient_Scan_ReadsNoDeeperThanMaxDepth(t *testing.T) {
	const depth1, depth2, depth3 = "depth1", "depth2", "depth3"

	address := startPrivateBus(t)

	app := connectBus(t, address)
	scanner := connectBus(t, address)

	child := fixtureChild(app.Names()[0])

	// The buttons sit at depth 1, 2 and 3 under the frame. Darwin also counts
	// the window's children as depth 1.
	inner := child("inner", "panel", atspiStateShowing)
	inner.children = []*fakeAccessible{
		child(depth3, "push button", atspiStateShowing, atspiStateEnabled),
	}

	outer := child("outer", "panel", atspiStateShowing)
	outer.children = []*fakeAccessible{
		child(depth2, "push button", atspiStateShowing, atspiStateEnabled),
		inner,
	}

	frame := child("frame", "frame", atspiStateShowing)
	frame.children = []*fakeAccessible{
		child(depth1, "push button", atspiStateShowing, atspiStateEnabled),
		outer,
	}

	exportFixture(t, app, frame)

	roles := rolesSet(resolveLinuxRoles(t, "button"))
	all := []string{depth1, depth2, depth3}

	tests := []struct {
		name     string
		maxDepth int
		want     []string
		wantPath string
	}{
		{"one level", 1, []string{depth1}, scanPathWalk},
		{"two levels", 2, []string{depth1, depth2}, scanPathWalk},
		{"unlimited", 0, all, scanPathCollection},
		{"the default", config.DefaultMaxDepth, all, scanPathCollection},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := New(zap.NewNop(), nil)

			nodes, path, _ := client.scan(t.Context(), scanner, frame.ref, roles,
				test.maxDepth, placement{}, false)

			got := make([]string, 0, len(nodes))
			for _, node := range nodes {
				got = append(got, fixtureName(node))
			}

			slices.Sort(got)

			if !slices.Equal(got, test.want) {
				t.Errorf("hinted %v, want %v", got, test.want)
			}

			if path != test.wantPath {
				t.Errorf("read through the %s path, want %s", path, test.wantPath)
			}
		})
	}
}

// exportFixture exports frame and every element under it on app under the
// interfaces a scan calls.
func exportFixture(t *testing.T, app *dbus.Conn, frame *fakeAccessible) {
	t.Helper()

	ifaces := []string{
		atspiAccessibleIfc, atspiComponentIfc, atspiCollectionIfc, atspiTextIfc,
		"org.freedesktop.DBus.Properties",
	}

	for _, iface := range ifaces {
		err := app.Export(frame, frame.ref.Path, iface)
		if err != nil {
			t.Fatalf("export %s on %s: %v", iface, frame.ref.Path, err)
		}
	}

	for _, child := range frame.children {
		exportFixture(t, app, child)
	}
}

// scanPaths returns the walk and the Collection path, each scanning frame
// for roles and reading the element state when readState is set.
func scanPaths(
	t *testing.T,
	scanner *dbus.Conn,
	frame *fakeAccessible,
	readState bool,
) map[string]func(*Client, map[string]struct{}) []ax.Node {
	t.Helper()

	return map[string]func(*Client, map[string]struct{}) []ax.Node{
		"walk": func(client *Client, roles map[string]struct{}) []ax.Node {
			var out []ax.Node

			visited := 0
			client.walk(
				t.Context(),
				scanner,
				frame.ref,
				roles,
				0,
				atspiWalkDepthCeiling,
				&out,
				&visited,
				placement{},
				readState,
			)

			return out
		},
		"collection": func(client *Client, roles map[string]struct{}) []ax.Node {
			out, ok := client.collectViaCollection(
				t.Context(), scanner, frame.ref, roles, placement{}, readState)
			if !ok {
				t.Fatal("the Collection path fell back to the walk")
			}

			return out
		},
	}
}

// fixtureName returns the fixture name in a node's object path.
func fixtureName(node ax.Node) string {
	return strings.TrimPrefix(strings.Split(node.ID(), "@")[0], "/fixture/")
}

// startPrivateBus runs a dbus-daemon for this test alone and returns its
// address.
func startPrivateBus(t *testing.T) string {
	t.Helper()

	daemon, err := exec.LookPath("dbus-daemon")
	if err != nil {
		t.Skip("skipping: dbus-daemon is not installed")
	}

	ctx, cancel := context.WithCancel(context.Background())

	cmd := exec.CommandContext(ctx, daemon, "--session", "--nofork", "--print-address=1",
		"--address=unix:path="+filepath.Join(t.TempDir(), "bus"))

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("dbus-daemon stdout: %v", err)
	}

	err = cmd.Start()
	if err != nil {
		t.Fatalf("start dbus-daemon: %v", err)
	}

	t.Cleanup(func() {
		cancel()

		_ = cmd.Wait()
	})

	lines := make(chan string, 1)

	go func() {
		line, _ := bufio.NewReader(stdout).ReadString('\n')
		lines <- strings.TrimSpace(line)
	}()

	select {
	case address := <-lines:
		if address == "" {
			t.Fatal("dbus-daemon printed no address")
		}

		return address
	case <-time.After(5 * time.Second):
		t.Fatal("dbus-daemon did not print its address within 5s")
	}

	return ""
}

func connectBus(t *testing.T, address string) *dbus.Conn {
	t.Helper()

	conn, err := dbus.Connect(address)
	if err != nil {
		t.Fatalf("connect to the private bus: %v", err)
	}

	t.Cleanup(func() { _ = conn.Close() })

	return conn
}

// stateBitsFor packs AT-SPI state indices the way GetState returns them.
func stateBitsFor(states ...uint) []uint32 {
	words := make([]uint32, atspiStateWords)
	for _, state := range states {
		words[state/atspiBitsPerWord] |= 1 << (state % atspiBitsPerWord)
	}

	return words
}
