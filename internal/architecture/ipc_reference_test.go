package architecture_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/app/ipcctrl"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain/state"
)

const ipcReferenceDoc = "docs/reference/ipc.md"

// backtickedWord matches one `word` in a table cell.
var backtickedWord = regexp.MustCompile("`([^`]+)`")

// The IPC reference is the only list of commands a third-party client has, so
// a command the daemon registers and the table leaves out is one nobody outside
// the source can find, and a row the daemon no longer answers is a promise it
// broke without a protocol bump.
func TestEveryIPCCommandIsInTheIPCReference(t *testing.T) {
	controller := ipcctrl.New(ipcctrl.Deps{
		AppState: state.NewAppState(),
		Config:   config.DefaultConfig(),
	})

	registered := make([]string, 0, len(controller.Handlers)+len(controller.Streams))
	for name := range controller.Handlers {
		registered = append(registered, name)
	}

	for name := range controller.Streams {
		registered = append(registered, name)
	}

	documented := documentedIPCCommands(t)

	for _, name := range registered {
		if !slices.Contains(documented, name) {
			t.Errorf("%s: the daemon answers %q, but the Commands table does not list it",
				ipcReferenceDoc, name)
		}
	}

	for _, name := range documented {
		if !slices.Contains(registered, name) {
			t.Errorf("%s: the Commands table lists %q, but the daemon does not answer it",
				ipcReferenceDoc, name)
		}
	}
}

// A client checks the protocol it was written against with the number the
// reference states, so a bump the reference does not repeat misleads it.
func TestTheIPCReferenceStatesTheProtocolVersion(t *testing.T) {
	want := fmt.Sprintf("speaks. It is `%d`.", ipc.ProtocolVersion)
	if !strings.Contains(readIPCReference(t), want) {
		t.Errorf("%s: the Compatibility section must say %q, matching ipc.ProtocolVersion",
			ipcReferenceDoc, want)
	}
}

// readIPCReference returns the IPC reference's text.
func readIPCReference(t *testing.T) string {
	t.Helper()

	content, readErr := os.ReadFile(filepath.Join(findRepoRoot(t), ipcReferenceDoc))
	if readErr != nil {
		t.Fatalf("ReadFile(%s) error = %v", ipcReferenceDoc, readErr)
	}

	return string(content)
}

// documentedIPCCommands returns every command named in the first column of the
// Commands table.
func documentedIPCCommands(t *testing.T) []string {
	t.Helper()

	_, section, found := strings.Cut(readIPCReference(t), "\n## Commands\n")
	if !found {
		t.Fatalf("%s has no Commands section", ipcReferenceDoc)
	}

	section, _, _ = strings.Cut(section, "\n## ")

	var names []string

	for line := range strings.SplitSeq(section, "\n") {
		cells := strings.Split(line, "|")
		if len(cells) < 3 || !strings.HasPrefix(strings.TrimSpace(cells[1]), "`") {
			continue
		}

		for _, match := range backtickedWord.FindAllStringSubmatch(cells[1], -1) {
			names = append(names, match[1])
		}
	}

	if len(names) == 0 {
		t.Fatalf("%s: the Commands table names no commands", ipcReferenceDoc)
	}

	return names
}
