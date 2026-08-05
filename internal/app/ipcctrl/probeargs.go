package ipcctrl

import (
	"strings"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
)

// probeArgs walks a hints probe's arguments.
//
// Every flag that carries a value can be written two ways — "--flag=value" or
// "--flag value" — and reading the second form means looking at the next
// argument and skipping it on the following pass. Doing that at each flag is
// the same dozen lines repeated, including a bounds check whose absence is a
// panic rather than a refusal, so it lives here once.
//
// A mode command is read by the grammar in internal/domain/modecmd instead. A
// probe has its own short vocabulary and no rules between its flags, so it
// stops here.
type probeArgs struct {
	args []string
	// index is the argument being read. take advances it when it consumes a
	// following argument as a value.
	index int
}

// newProbeArgs starts a walk over a request's arguments, skipping the request's
// own name when the caller included it, as anything modeled on the CLI's
// traffic does.
func newProbeArgs(cmd ipc.Command) *probeArgs {
	if len(cmd.Args) > 0 && cmd.Args[0] == cmd.Action {
		return &probeArgs{args: cmd.Args[1:]}
	}

	return &probeArgs{args: cmd.Args}
}

// more reports whether an argument remains.
func (m *probeArgs) more() bool { return m.index < len(m.args) }

// arg returns the argument being read.
func (m *probeArgs) arg() string { return m.args[m.index] }

// next moves to the following argument.
func (m *probeArgs) next() { m.index++ }

// is reports whether the current argument is the named flag, in any spelling
// that flag accepts.
func (m *probeArgs) is(name modecmd.Flag) bool {
	descriptor, known := modecmd.Lookup(name)
	if !known {
		return false
	}

	return descriptor.Match(m.arg())
}

// take reads the value belonging to the flag currently being read, in whichever
// form it was written, and consumes a following argument if that is where the
// value was.
//
// missing is the whole message a user sees when the flag was given with no
// value at all, so a flag with a constrained vocabulary can name it.
func (m *probeArgs) take(missing string) (string, *ipc.Response) {
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
