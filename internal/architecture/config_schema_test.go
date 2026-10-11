package architecture_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/y3owk1n/neru/internal/configschema"
)

// The committed schema is what every editor pointed at it reads. A field added
// to the config structs without `just genschema` would be flagged as unknown in
// every user's editor while the daemon accepts it.
func TestConfigSchema_IsGeneratedFromTheConfigStructs(t *testing.T) {
	want, err := configschema.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	got, readErr := os.ReadFile(filepath.Join(findRepoRoot(t), configschema.Path))
	if readErr != nil {
		t.Fatalf("ReadFile(%s) error = %v; run `just genschema`", configschema.Path, readErr)
	}

	if string(got) != string(want) {
		t.Errorf(
			"%s is out of date with the config structs; run `just genschema`",
			configschema.Path,
		)
	}
}
