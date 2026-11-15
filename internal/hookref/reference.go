package hookref

import (
	"fmt"
	"strings"

	"github.com/y3owk1n/neru/internal/docsregion"
	"github.com/y3owk1n/neru/internal/domain/event"
)

// The comments delimiting each generated region.
const (
	HooksBeginMarker     = "<!-- BEGIN GENERATED HOOKS: edit internal/domain/event/catalog.go, then run `just genhookref` -->"
	HooksEndMarker       = "<!-- END GENERATED HOOKS -->"
	VariablesBeginMarker = "<!-- BEGIN GENERATED HOOK VARIABLES: edit internal/domain/event/catalog.go, then run `just genhookref` -->"
	VariablesEndMarker   = "<!-- END GENERATED HOOK VARIABLES -->"
)

var (
	hooksMarkers = docsregion.Markers{
		Begin: HooksBeginMarker,
		End:   HooksEndMarker,
		What:  "hooks",
	}
	variablesMarkers = docsregion.Markers{
		Begin: VariablesBeginMarker,
		End:   VariablesEndMarker,
		What:  "hook-variables",
	}
)

// everyHook is what the variables table says for NERU_EVENT, which no event
// lists because every event sets it.
const everyHook = "every hook"

// HooksTable renders one row per event, in catalog order.
func HooksTable() string {
	var out strings.Builder

	out.WriteString("| Option | Type | Default | Runs when |\n")
	out.WriteString("| ------ | ---- | ------- | --------- |\n")

	for _, spec := range event.Catalog() {
		fmt.Fprintf(&out, "| `%s` | string/array | none | %s |\n", spec.Hook(), spec.Description)
	}

	return out.String()
}

// VariablesTable renders one row per variable, naming the hooks whose events
// carry it.
func VariablesTable() string {
	var out strings.Builder

	out.WriteString("| Variable | Set for | Value |\n")
	out.WriteString("| -------- | ------- | ----- |\n")

	for _, variable := range event.Variables() {
		fmt.Fprintf(
			&out,
			"| `%s` | %s | %s |\n",
			variable.Name,
			setFor(variable.Name),
			variable.Value,
		)
	}

	return out.String()
}

// setFor names the hooks whose events list the variable.
func setFor(name string) string {
	var hooks []string

	for _, spec := range event.Catalog() {
		for _, variable := range spec.Variables {
			if variable.Name == name {
				hooks = append(hooks, "`"+spec.Hook()+"`")
			}
		}
	}

	if len(hooks) == 0 {
		return everyHook
	}

	return strings.Join(hooks, ", ")
}

// Rewrite returns the document with both regions rewritten, and reports a
// document missing either.
func Rewrite(document string) (string, error) {
	rewritten, err := hooksMarkers.Rewrite(document, HooksTable())
	if err != nil {
		return "", err
	}

	return variablesMarkers.Rewrite(rewritten, VariablesTable())
}

// HooksRegion returns what a document holds between the hooks markers.
func HooksRegion(document string) (string, error) {
	return hooksMarkers.Region(document)
}

// VariablesRegion returns what a document holds between the variables markers.
func VariablesRegion(document string) (string, error) {
	return variablesMarkers.Region(document)
}
