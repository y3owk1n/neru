package modecmd

// Render writes an activation back into the argument list that carries it.
//
// The mode's own name is left out: it names the request, and repeating it
// inside the arguments is redundant. Parsing tolerates it because callers
// modeled on the CLI's traffic send it, but nothing here writes it.
//
// Rendering is the inverse of parsing over the whole table — parsing a
// rendering returns the same activation. That is what lets a caller holding
// typed flags hand them to the daemon as text without spelling any of them
// itself.
func Render(activation Activation) []string {
	args := make([]string, 0, len(descriptors))

	for _, descriptor := range descriptors {
		args = append(args, descriptor.render(activation)...)
	}

	return args
}
