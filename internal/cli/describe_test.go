package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/client"
)

func TestSchemaCmd_PrintsThePublishedSchema(t *testing.T) {
	for _, name := range []string{"ipc", "config"} {
		t.Run(name, func(t *testing.T) {
			stdout, stderr, err := executeRoot(t, "schema", name)
			if err != nil {
				t.Fatalf("neru schema %s failed: %v\n%s", name, err, stderr)
			}

			published, readErr := os.ReadFile(
				filepath.Join("..", "..", "schemas", name+".schema.json"),
			)
			if readErr != nil {
				t.Fatalf("read the published schema: %v", readErr)
			}

			if stdout != string(published) {
				t.Errorf("neru schema %s differs from schemas/%s.schema.json; run `just genschema`",
					name, name)
			}
		})
	}
}

func TestDescribeCmd_JSON_DescribesThisPlatformWithoutADaemon(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("TMPDIR", t.TempDir())

	stdout, stderr, err := executeRoot(t, "describe", "--json")
	if err != nil {
		t.Fatalf("neru describe --json failed: %v\n%s", err, stderr)
	}

	var manifest client.DescribeData

	decodeErr := json.Unmarshal([]byte(stdout), &manifest)
	if decodeErr != nil {
		t.Fatalf("neru describe --json printed something other than the manifest: %v", decodeErr)
	}

	if manifest.Platform != runtime.GOOS {
		t.Errorf("platform = %q, want %q", manifest.Platform, runtime.GOOS)
	}

	if manifest.Protocol != client.ProtocolVersion {
		t.Errorf("protocol = %d, want %d", manifest.Protocol, client.ProtocolVersion)
	}

	if len(manifest.Actions) == 0 || len(manifest.Events) == 0 || len(manifest.Options) == 0 {
		t.Errorf("the manifest is missing a vocabulary: %d actions, %d events, %d options",
			len(manifest.Actions), len(manifest.Events), len(manifest.Options))
	}
}

func TestDescribeCmd_Summary_NamesThePlatform(t *testing.T) {
	stdout, stderr, err := executeRoot(t, "describe")
	if err != nil {
		t.Fatalf("neru describe failed: %v\n%s", err, stderr)
	}

	if !strings.Contains(stdout, "on "+runtime.GOOS) {
		t.Errorf("neru describe does not name the platform %q:\n%s", runtime.GOOS, stdout)
	}
}

func TestAgentGuideCmd_NamesThisBuild(t *testing.T) {
	stdout, stderr, err := executeRoot(t, "agent-guide")
	if err != nil {
		t.Fatalf("neru agent-guide failed: %v\n%s", err, stderr)
	}

	if strings.Contains(stdout, "{{") {
		t.Errorf("neru agent-guide left a placeholder unfilled:\n%s", stdout)
	}

	if !strings.Contains(stdout, "protocol "+strconv.Itoa(client.ProtocolVersion)) {
		t.Errorf("neru agent-guide does not state the protocol")
	}
}
