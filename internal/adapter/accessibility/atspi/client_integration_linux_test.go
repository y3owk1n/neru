//go:build integration && linux

package atspi

import (
	"bufio"
	"context"
	"errors"
	"maps"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/accessibility/ax"
)

// Both scan paths run against a fake application exported on a private
// dbus-daemon, so the state reads and the editable-text rule are checked in
// the walk and the Collection path themselves.

// errStateUnavailable is what a fake element whose state read fails returns.
var errStateUnavailable = errors.New("state unavailable")

// fakeAccessible answers the AT-SPI calls a scan makes for one element.
type fakeAccessible struct {
	ref         accRef
	role        string
	states      []uint32
	stateFails  bool
	children    []*fakeAccessible
	name        string
	description string
	text        string
	textReads   atomic.Int32
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

// Get answers org.freedesktop.DBus.Properties.Get for the two properties a
// scan reads.
func (f *fakeAccessible) Get(_, property string) (dbus.Variant, *dbus.Error) {
	if property == "Description" {
		return dbus.MakeVariant(f.description), nil
	}

	return dbus.MakeVariant(f.name), nil
}

func (f *fakeAccessible) GetText(int32, int32) (string, *dbus.Error) {
	f.textReads.Add(1)

	return f.text, nil
}

func (f *fakeAccessible) GetExtents(uint32) (atspiExtents, *dbus.Error) {
	return atspiExtents{W: 10, H: 10}, nil
}

// GetMatches answers the Collection query as a toolkit does: the children
// whose role is in the rule and that are showing.
func (f *fakeAccessible) GetMatches(
	rule atspiMatchRule,
	_ uint32,
	_ int32,
	_ bool,
) ([]accRef, *dbus.Error) {
	var refs []accRef

	for _, child := range f.children {
		roleID := atspiRoleNameToID[child.role]
		inRule := rule.Roles[roleID/atspiBitsPerWord]&(1<<(roleID%atspiBitsPerWord)) != 0

		if inRule && atspiStateBit(child.states, atspiStateShowing) {
			refs = append(refs, child.ref)
		}
	}

	return refs, nil
}

func TestClient_ScanPaths_ReadEditableTextAsEntry(t *testing.T) {
	address := startPrivateBus(t)

	app := connectBus(t, address)
	scanner := connectBus(t, address)

	child := fixtureChild(app.Names()[0])

	stateFails := child("state_fails", atspiRoleText, atspiStateShowing, atspiStateEditable)
	stateFails.stateFails = true

	frame := child("frame", "frame", atspiStateShowing)
	frame.children = []*fakeAccessible{
		child("editable", atspiRoleText, atspiStateShowing, atspiStateEditable),
		child("read_only", atspiRoleText, atspiStateShowing),
		child("hidden", atspiRoleText, atspiStateEditable),
		stateFails,
		child("button", "push button", atspiStateShowing),
	}

	exportFixture(t, app, frame)
	scans := scanPaths(t, scanner, frame)

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

	entry := child("entry", atspiRoleText, atspiStateShowing, atspiStateEditable)
	entry.name, entry.description, entry.text = "Search", "Find in page", "typed text"

	password := child("password", "password text", atspiStateShowing, atspiStateEditable)
	password.text = "hunter2"

	button := child("save", "push button", atspiStateShowing)
	button.name, button.description, button.text = "Save", "Save the file", "button text"

	frame := child("frame", "frame", atspiStateShowing)
	frame.children = []*fakeAccessible{entry, password, button}

	exportFixture(t, app, frame)

	want := map[string][3]string{
		"entry":    {"Search", "Find in page", "typed text"},
		"password": {"", "", ""},
		"save":     {"Save", "Save the file", ""},
	}

	roles := rolesSet(resolveLinuxRoles(t, "text_field", "button"))

	for path, scan := range scanPaths(t, scanner, frame) {
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

// exportFixture exports frame and its children on app under the interfaces a
// scan calls.
func exportFixture(t *testing.T, app *dbus.Conn, frame *fakeAccessible) {
	t.Helper()

	ifaces := []string{
		atspiAccessibleIfc, atspiComponentIfc, atspiCollectionIfc, atspiTextIfc,
		"org.freedesktop.DBus.Properties",
	}

	for _, fake := range append([]*fakeAccessible{frame}, frame.children...) {
		for _, iface := range ifaces {
			err := app.Export(fake, fake.ref.Path, iface)
			if err != nil {
				t.Fatalf("export %s on %s: %v", iface, fake.ref.Path, err)
			}
		}
	}
}

// scanPaths returns the walk and the Collection path, each scanning frame
// for roles.
func scanPaths(
	t *testing.T,
	scanner *dbus.Conn,
	frame *fakeAccessible,
) map[string]func(*Client, map[string]struct{}) []ax.Node {
	t.Helper()

	return map[string]func(*Client, map[string]struct{}) []ax.Node{
		"walk": func(client *Client, roles map[string]struct{}) []ax.Node {
			var out []ax.Node

			visited := 0
			client.walk(t.Context(), scanner, frame.ref, roles, 0, &out, &visited, 0, 0)

			return out
		},
		"collection": func(client *Client, roles map[string]struct{}) []ax.Node {
			out, ok := client.collectViaCollection(t.Context(), scanner, frame.ref, roles, 0, 0)
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
