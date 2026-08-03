package ipcctrl

import (
	"strings"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
)

// modeArgs walks a mode command's arguments.
//
// Every flag that carries a value can be written two ways — "--flag=value" or
// "--flag value" — and reading the second form means looking at the next
// argument and skipping it on the following pass. Doing that at each flag is
// the same dozen lines repeated, including a bounds check whose absence is a
// panic rather than a refusal, so it lives here once.
type modeArgs struct {
	args []string
	// index is the argument being read. take advances it when it consumes a
	// following argument as a value.
	index int
}

// newModeArgs starts a walk over a mode command's arguments, skipping the mode
// name when the caller included it.
//
// The CLI sends it — `neru grid --action left_click` arrives as
// ["grid", "--action", "left_click"] — and the hotkey path does not, so both
// shapes have to reach the same parse.
func newModeArgs(cmd ipc.Command) *modeArgs {
	if len(cmd.Args) > 0 && cmd.Args[0] == cmd.Action {
		return &modeArgs{args: cmd.Args[1:]}
	}

	return &modeArgs{args: cmd.Args}
}

// more reports whether an argument remains.
func (m *modeArgs) more() bool { return m.index < len(m.args) }

// arg returns the argument being read.
func (m *modeArgs) arg() string { return m.args[m.index] }

// next moves to the following argument.
func (m *modeArgs) next() { m.index++ }

// is reports whether the current argument is the named flag, in any of the
// spellings given — its long form, its short form, or the "--flag=" prefix.
func (m *modeArgs) is(names ...string) bool {
	arg := m.arg()

	for _, name := range names {
		if arg == name || strings.HasPrefix(arg, name+"=") {
			return true
		}
	}

	return false
}

// take reads the value belonging to the flag currently being read, in whichever
// form it was written, and consumes a following argument if that is where the
// value was.
//
// missing is the whole message a user sees when the flag was given with no
// value at all, so a flag with a constrained vocabulary can name it.
func (m *modeArgs) take(missing string) (string, *ipc.Response) {
	arg := m.arg()

	if _, after, ok := strings.Cut(arg, "="); ok {
		return after, nil
	}

	if m.index+1 >= len(m.args) {
		return "", &ipc.Response{
			Success: false,
			Message: missing,
			Code:    ipc.CodeInvalidInput,
		}
	}

	m.index++

	return m.args[m.index], nil
}

// refuse builds the refusal a flag returns when its value is unusable.
func refuse(message string) *ipc.Response {
	return &ipc.Response{
		Success: false,
		Message: message,
		Code:    ipc.CodeInvalidInput,
	}
}
