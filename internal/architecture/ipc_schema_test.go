package architecture_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/y3owk1n/neru/internal/ipcschema"
)

// The committed IPC schema is what a client generates its types from or checks
// a reply against. A wire type changed without `just genschema` would leave
// every such client checking against a format the daemon no longer sends.
func TestIPCSchema_IsGeneratedFromTheWireTypes(t *testing.T) {
	want, err := ipcschema.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	got, readErr := os.ReadFile(filepath.Join(findRepoRoot(t), ipcschema.Path))
	if readErr != nil {
		t.Fatalf("ReadFile(%s) error = %v; run `just genschema`", ipcschema.Path, readErr)
	}

	if string(got) != string(want) {
		t.Errorf("%s is out of date with the wire types; run `just genschema`", ipcschema.Path)
	}
}
