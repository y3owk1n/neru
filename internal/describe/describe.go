package describe

import (
	"slices"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/buildinfo"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/event"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
	"github.com/y3owk1n/neru/internal/domain/parity"
	"github.com/y3owk1n/neru/internal/supportref"
)

// hooksSection is the config table a hook key sits in.
const hooksSection = "hooks."

// Manifest describes this binary on the platform it runs on.
func Manifest() ipc.DescribeData {
	platform, _ := parity.Current()

	return Build(platform)
}

// Build describes this binary as seen from platform. An empty platform, a
// build Neru declares nothing about, reports every word as not working here.
func Build(platform parity.Platform) ipc.DescribeData {
	words := newIndex(supportref.Declaration(), platform)

	return ipc.DescribeData{
		Version:   buildinfo.Version,
		Protocol:  ipc.ProtocolVersion,
		Platform:  string(platform),
		Modes:     builtInModes(),
		ModeFlags: modeFlags(words),
		Actions:   actions(words),
		Options:   options(words),
		Events:    events(words),
		Variables: variables(),
	}
}

// support is one word's platform column, as the manifest writes it.
type support struct {
	platforms []string
	here      bool
	note      string
}

// index is the platform declarations keyed by kind and name, with the narrow
// values of each word gathered under it.
type index struct {
	words  map[parity.Kind]map[string]support
	values map[parity.Kind]map[string][]ipc.DescribeValue
	names  map[parity.Kind][]string
}

func newIndex(declaration parity.Declaration, platform parity.Platform) index {
	idx := index{
		words:  map[parity.Kind]map[string]support{},
		values: map[parity.Kind]map[string][]ipc.DescribeValue{},
		names:  map[parity.Kind][]string{},
	}

	for _, word := range declaration {
		column := support{
			platforms: platformNames(word.Platforms),
			here:      platform != "" && word.Platforms.Supports(platform),
			note:      word.Note,
		}

		if word.Value != "" {
			if idx.values[word.Kind] == nil {
				idx.values[word.Kind] = map[string][]ipc.DescribeValue{}
			}

			idx.values[word.Kind][word.Name] = append(idx.values[word.Kind][word.Name],
				ipc.DescribeValue{
					Value:     word.Value,
					Platforms: column.platforms,
					Here:      column.here,
					Note:      column.note,
				})

			continue
		}

		if idx.words[word.Kind] == nil {
			idx.words[word.Kind] = map[string]support{}
		}

		idx.words[word.Kind][word.Name] = column
		idx.names[word.Kind] = append(idx.names[word.Kind], word.Name)
	}

	return idx
}

// word returns the column declared for a word. A word nothing declares works
// nowhere, which is what the guardrails on each vocabulary refuse to let ship.
func (idx index) word(kind parity.Kind, name string) support {
	column, ok := idx.words[kind][name]
	if !ok {
		return support{platforms: []string{}}
	}

	return column
}

// valuesOf returns the narrow values declared for a word, never nil.
func (idx index) valuesOf(kind parity.Kind, name string) []ipc.DescribeValue {
	values := idx.values[kind][name]
	if values == nil {
		return []ipc.DescribeValue{}
	}

	return values
}

// sortedNames returns every word of a kind, sorted.
func (idx index) sortedNames(kind parity.Kind) []string {
	names := slices.Clone(idx.names[kind])
	slices.Sort(names)

	return names
}

func modeFlags(idx index) []ipc.DescribeModeFlag {
	descriptors := modecmd.All()
	flags := make([]ipc.DescribeModeFlag, 0, len(descriptors))

	for _, descriptor := range descriptors {
		name := descriptor.Name().String()
		column := idx.word(parity.KindModeFlag, name)

		flags = append(flags, ipc.DescribeModeFlag{
			Name:      name,
			Short:     descriptor.Short(),
			Shape:     shape(descriptor.Kind()),
			Modes:     modeNames(descriptor.AcceptedModes()),
			Usage:     descriptor.Usage(),
			Platforms: column.platforms,
			Here:      column.here,
			Note:      column.note,
			Values:    idx.valuesOf(parity.KindModeFlag, name),
		})
	}

	return flags
}

func actions(idx index) []ipc.DescribeAction {
	names := idx.sortedNames(parity.KindAction)
	described := make([]ipc.DescribeAction, 0, len(names))

	for _, name := range names {
		column := idx.word(parity.KindAction, name)

		described = append(described, ipc.DescribeAction{
			Name:      name,
			Modes:     actionModes(name),
			Platforms: column.platforms,
			Here:      column.here,
			Note:      column.note,
		})
	}

	return described
}

func options(idx index) []ipc.DescribeOption {
	names := idx.sortedNames(parity.KindOption)
	described := make([]ipc.DescribeOption, 0, len(names))

	for _, name := range names {
		column := idx.word(parity.KindOption, name)

		described = append(described, ipc.DescribeOption{
			Name:      name,
			Platforms: column.platforms,
			Here:      column.here,
			Note:      column.note,
			Values:    idx.valuesOf(parity.KindOption, name),
		})
	}

	return described
}

// events takes each event's platforms from its hook key, which is where the
// platform column for an event is declared.
func events(idx index) []ipc.DescribeEvent {
	specs := event.Catalog()
	described := make([]ipc.DescribeEvent, 0, len(specs))

	for _, spec := range specs {
		column := idx.word(parity.KindOption, hooksSection+spec.Hook())

		seen := spec.HookVariables()

		names := make([]string, 0, len(seen))
		for _, variable := range seen {
			names = append(names, variable.Name)
		}

		described = append(described, ipc.DescribeEvent{
			Name:        string(spec.Name),
			Hook:        spec.Hook(),
			Description: spec.Description,
			Variables:   names,
			Platforms:   column.platforms,
			Here:        column.here,
			Note:        column.note,
		})
	}

	return described
}

func variables() []ipc.DescribeVariable {
	all := event.Variables()
	described := make([]ipc.DescribeVariable, 0, len(all))

	for _, variable := range all {
		described = append(
			described,
			ipc.DescribeVariable{Name: variable.Name, Value: variable.Value},
		)
	}

	return described
}

// actionModes returns the modes whose --action accepts name, asked of the
// grammar that validates a binding, so the manifest cannot accept what a
// binding refuses.
func actionModes(name string) []string {
	modes := []string{}

	flag, _ := modecmd.Lookup(modecmd.FlagAction)
	for _, mode := range flag.AcceptedModes() {
		_, err := modecmd.Parse(mode, []string{modecmd.FlagAction.Assign(name)})
		if err == nil {
			modes = append(modes, domain.ModeString(mode))
		}
	}

	return modes
}

func shape(kind modecmd.Kind) string {
	switch kind {
	case modecmd.KindPresence:
		return "presence"
	case modecmd.KindValue:
		return "value"
	case modecmd.KindList:
		return "list"
	}

	return "unknown"
}

// builtInModes names every mode a mode command can open, as status and
// NERU_MODE name them. It leaves out the declared-mode command, since its
// word is not the name of any mode.
func builtInModes() []string {
	modes := modecmd.Modes()

	return modeNames(slices.DeleteFunc(modes, func(mode domain.Mode) bool {
		return mode == domain.ModeCustom
	}))
}

func modeNames(modes []domain.Mode) []string {
	names := make([]string, 0, len(modes))
	for _, mode := range modes {
		names = append(names, domain.ModeString(mode))
	}

	return names
}

func platformNames(platforms parity.Platforms) []string {
	names := make([]string, 0, len(platforms))
	for _, platform := range platforms {
		names = append(names, string(platform))
	}

	return names
}
