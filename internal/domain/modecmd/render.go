package modecmd

import (
	"github.com/y3owk1n/neru/internal/domain"
)

// Render writes an activation back into the argument list that carries it.
//
// The mode's own name is left out: it names the request, and repeating it
// inside the arguments is redundant. Parsing tolerates it because callers
// modeled on the CLI's traffic send it, but nothing here writes it. A custom
// activation's declared name is written, and first: it is an argument of the
// request rather than the name of it, and parsing reads it from that position.
//
// Rendering is the inverse of parsing over the whole table — parsing a
// rendering returns the same activation. That is what lets a caller holding
// typed flags hand them to the daemon as text without spelling any of them
// itself.
func Render(activation Activation) []string {
	args := make([]string, 0, len(descriptors)+1)

	if activation.Mode == domain.ModeCustom && activation.Name != "" {
		args = append(args, activation.Name)
	}

	for _, descriptor := range descriptors {
		args = append(args, descriptor.render(activation)...)
	}

	return args
}
