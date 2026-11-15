package client

// DescribeData is the payload of `describe`, and what `neru describe` prints:
// every word a binding, a mode command or config.toml can carry, with the
// platforms writing it does something on.
//
// Platforms lists the platforms a word works on, Here says whether it works
// on the one answering, and Note says why the list is short, empty when the
// word works everywhere. Values lists only the values of a word whose
// platforms differ from the word's own, such as one detection strategy.
//
// The prose fields (Usage, Description, Value and Note) are for reading and
// may be reworded in any release. The names, lists and booleans keep their
// meaning within a protocol.
type DescribeData struct {
	Version   string             `json:"version"`
	Protocol  int                `json:"protocol"`
	Platform  string             `json:"platform"`
	Modes     []string           `json:"modes"`
	ModeFlags []DescribeModeFlag `json:"mode_flags"` //nolint:tagliatelle // snake_case like the other describe keys.
	Actions   []DescribeAction   `json:"actions"`
	Options   []DescribeOption   `json:"options"`
	Events    []DescribeEvent    `json:"events"`
	Variables []DescribeVariable `json:"variables"`
}

// DescribeModeFlag is one flag a mode command takes. Shape is "presence" for
// a flag written alone, "value" for one written with a value, and "list" for
// one whose repeats add up.
type DescribeModeFlag struct {
	Name      string          `json:"name"`
	Short     string          `json:"short"`
	Shape     string          `json:"shape"`
	Modes     []string        `json:"modes"`
	Usage     string          `json:"usage"`
	Platforms []string        `json:"platforms"`
	Here      bool            `json:"here"`
	Note      string          `json:"note"`
	Values    []DescribeValue `json:"values"`
}

// DescribeAction is one action name. Modes lists the modes whose --action
// can run it on a selection, and is empty for an action that runs only on its
// own.
type DescribeAction struct {
	Name      string   `json:"name"`
	Modes     []string `json:"modes"`
	Platforms []string `json:"platforms"`
	Here      bool     `json:"here"`
	Note      string   `json:"note"`
}

// DescribeOption is one config.toml key, written as its full TOML path. Its
// type and default are in the config schema, which `neru schema config`
// prints.
type DescribeOption struct {
	Name      string          `json:"name"`
	Platforms []string        `json:"platforms"`
	Here      bool            `json:"here"`
	Note      string          `json:"note"`
	Values    []DescribeValue `json:"values"`
}

// DescribeValue is one value of a flag or an option that works on fewer
// platforms than the word itself.
type DescribeValue struct {
	Value     string   `json:"value"`
	Platforms []string `json:"platforms"`
	Here      bool     `json:"here"`
	Note      string   `json:"note"`
}

// DescribeEvent is one event, as `watch` names it, with the [hooks] key that
// runs on it and the variables that hook's exec steps can see.
type DescribeEvent struct {
	Name        string   `json:"name"`
	Hook        string   `json:"hook"`
	Description string   `json:"description"`
	Variables   []string `json:"variables"`
	Platforms   []string `json:"platforms"`
	Here        bool     `json:"here"`
	Note        string   `json:"note"`
}

// DescribeVariable is one environment variable a hook's exec steps can see.
type DescribeVariable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
