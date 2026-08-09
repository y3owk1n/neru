package architecture_test

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/overlayutil"
)

// The two declarations of one struct. The resize completion path allocates it
// on the C heap, native code retains it across two dispatch_async hops, and the
// //export-ed Go side casts the raw pointer straight to the Go type and
// dereferences it — so the layouts are not merely similar, they are the same
// bytes read twice.
const (
	callbackContextHeader = "internal/adapter/platform/darwin/callback_context.h"

	// callbackContextTypedef is the C typedef's name. Addressing it by name is
	// what makes a rename fail here rather than pass over nothing.
	callbackContextTypedef = "callbackContext"

	// nativeIntSuffix is what C spells a fixed-width integer with and Go does
	// not, so a failure message can name a width in either vocabulary.
	nativeIntSuffix = "_t"
)

// callbackContextGoType names the Go side in failure messages, so a reader is
// sent to the declaration rather than to this test. It is asked of the linked
// type rather than written down, because a hand-written copy of a name is the
// kind of thing this file exists to distrust.
var callbackContextGoType = reflect.TypeFor[overlayutil.CallbackContext]().String()

// callbackContextField is one field of either declaration: the name it is
// written with, and its width spelled the way Go spells it.
//
// Both sides are reduced to this before anything is compared, so the comparison
// is over layout rather than over two vocabularies.
type callbackContextField struct {
	name string

	// goType is the fixed-width integer type the field is, in Go's spelling —
	// "uint64" for a Go uint64 and for a C uint64_t alike.
	goType string
}

// TestCallbackContextLayoutIsPinnedAcrossTheLanguageBoundary holds
// callback_context.h's struct to overlayutil.CallbackContext.
//
// This is ADR 0007's deliberate exception to the one-implementation rule: Go
// cannot be the implementation of a C struct that native code allocates and
// retains, so the copies get a pin instead of a deletion
// (docs/adr/0007-a-shared-derivation-has-one-implementation.md). What makes
// this copy worth pinning is that a disagreement is silent — nothing fails to
// compile, and no cast is checked. A callback ID and a generation would be read
// from the wrong offsets, validated against the registry in
// internal/adapter/overlay/render/overlayutil/util.go, and then dropped as
// stale or dispatched on garbage.
//
// It fails on the mistakes that happen: a field added to one side, the two
// fields reordered, a type widened or narrowed, and a field renamed on one
// side. The Go side is reflected over rather than read as text, because this
// package can link it — overlayutil carries no build tag, and reflection is
// what the //export-ed cast actually depends on. The C side has no such
// counterpart off macOS and is read through readNativeSource, the entry point
// every language-boundary pin here shares.
//
// What is left for it to catch on macOS is narrower than what it catches
// elsewhere, and worth being clear about. MallocCallbackContext
// (internal/adapter/platform/darwin/cstring.go) names the struct and both
// members through cgo, so on a macOS build a renamed struct, a renamed member
// and a changed width are already compile errors — verified by making each of
// those three edits. The reorder is not: both members are uint64 and cgo
// assigns them by name, so swapping them compiles and every other leg has no
// cgo at all. That is the case this pin is the only reader of.
func TestCallbackContextLayoutIsPinnedAcrossTheLanguageBoundary(t *testing.T) {
	t.Parallel()

	native := readCallbackContextHeader(t)
	goSide := goCallbackContextFields(t, reflect.TypeFor[overlayutil.CallbackContext]())

	for _, problem := range callbackContextProblems(native, goSide) {
		t.Error(problem)
	}
}

// TestCallbackContextLayoutPinCatchesDrift is the ticket's acceptance check,
// kept rather than performed once: each way the two declarations could move
// apart is applied to a copy of what the pin read, and the pin has to tell it
// from the tree.
//
// The Go drifts are whole struct types declared here and reflected over the way
// the real one is, so what they exercise is the reader as well as the
// comparison. The C drifts are applied to the fields parsed out of the header,
// which keeps this honest across a reformat of it.
func TestCallbackContextLayoutPinCatchesDrift(t *testing.T) {
	t.Parallel()

	native := readCallbackContextHeader(t)
	goSide := goCallbackContextFields(t, reflect.TypeFor[overlayutil.CallbackContext]())

	for _, drift := range callbackContextDrifts(t) {
		drifted := drift.apply(native, goSide)

		if callbackContextProblems(drifted.native, drifted.goSide) == nil {
			t.Errorf(
				"no check tells %s apart from the declarations in the tree: %s would pass the pin",
				drift.name, drift.where,
			)
		}
	}
}

// TestCallbackContextLayoutPinReportsADeclarationItCannotRead pins the other
// half of the guardrail: a declaration this pin cannot read must be reported,
// never skipped. A pin that reads nothing and passes is worse than no pin,
// because it reads as coverage.
//
// The Go side needs no case here — it is linked rather than read, so renaming
// or moving overlayutil.CallbackContext fails to compile, which is the loudest
// form of this failure. The C side is text, and the cost of reading text is
// that the pin reads one spelling: rewriting the struct in an equivalent shape
// fails here rather than being understood, and the failure names the shape it
// expected.
func TestCallbackContextLayoutPinReportsADeclarationItCannotRead(t *testing.T) {
	t.Parallel()

	header := readNativeSource(t, callbackContextHeader)

	for _, unreadable := range callbackContextUnreadableHeaders(t, header) {
		if _, problem := parseCallbackContextHeader(unreadable.header); problem == "" {
			t.Errorf(
				"parsing accepted a header with %s; the pin would then hold a struct it never read",
				unreadable.name,
			)
		}
	}
}

// callbackContextDrift is one way the two declarations could plausibly move
// apart.
type callbackContextDrift struct {
	name  string
	where string
	apply func(native, goSide []callbackContextField) callbackContextLayouts
}

// callbackContextLayouts is what the pin compares: the C fields and the Go
// fields, each in declaration order.
type callbackContextLayouts struct {
	native []callbackContextField
	goSide []callbackContextField
}

// callbackContextDrifts are the drifts the pin has to catch. The Go ones are
// read out of a deliberately mismatched struct type, which is the ticket's
// "add a field to one side only" and "reorder the two fields" written down.
func callbackContextDrifts(t *testing.T) []callbackContextDrift {
	t.Helper()

	goDrift := func(name string, value any) callbackContextDrift {
		return callbackContextDrift{
			name:  name,
			where: callbackContextGoType,
			apply: func(native, _ []callbackContextField) callbackContextLayouts {
				return callbackContextLayouts{
					native: native,
					goSide: goCallbackContextFields(t, reflect.TypeOf(value)),
				}
			},
		}
	}

	return append(
		[]callbackContextDrift{
			goDrift("a third field added to the Go struct", struct {
				CallbackID uint64
				Generation uint64
				Deadline   uint64
			}{}),
			goDrift("the two Go fields reordered", struct {
				Generation uint64
				CallbackID uint64
			}{}),
			goDrift("a Go field narrowed", struct {
				CallbackID uint32
				Generation uint64
			}{}),
			goDrift("a Go field renamed", struct {
				CallbackHandle uint64
				Generation     uint64
			}{}),
			goDrift("a Go field dropped", struct {
				CallbackID uint64
			}{}),
		},
		callbackContextNativeDrifts()...,
	)
}

// callbackContextNativeDrifts are the same drifts on the C side, applied to the
// fields parsed out of the header.
func callbackContextNativeDrifts() []callbackContextDrift {
	nativeDrift := func(name string, mutate func([]callbackContextField) []callbackContextField) callbackContextDrift {
		return callbackContextDrift{
			name:  name,
			where: callbackContextHeader,
			apply: func(native, goSide []callbackContextField) callbackContextLayouts {
				return callbackContextLayouts{
					native: mutate(append([]callbackContextField(nil), native...)),
					goSide: goSide,
				}
			},
		}
	}

	return []callbackContextDrift{
		nativeDrift(
			"a third field added to the C struct",
			func(fields []callbackContextField) []callbackContextField {
				return append(fields, callbackContextField{name: "deadline", goType: "uint64"})
			},
		),
		nativeDrift(
			"the two C fields reordered",
			func(fields []callbackContextField) []callbackContextField {
				return []callbackContextField{fields[1], fields[0]}
			},
		),
		nativeDrift(
			"a C field narrowed",
			func(fields []callbackContextField) []callbackContextField {
				fields[1].goType = "uint32"

				return fields
			},
		),
		nativeDrift(
			"a C field renamed",
			func(fields []callbackContextField) []callbackContextField {
				fields[0].name = "callbackHandle"

				return fields
			},
		),
		nativeDrift(
			"a C field dropped",
			func(fields []callbackContextField) []callbackContextField {
				return fields[:1]
			},
		),
	}
}

// callbackContextUnreadableHeader is one rewrite of the header this pin must
// report rather than read past.
type callbackContextUnreadableHeader struct {
	name   string
	header string
}

// callbackContextUnreadableHeaders doctors the header into shapes this pin does
// not read.
func callbackContextUnreadableHeaders(
	t *testing.T,
	header string,
) []callbackContextUnreadableHeader {
	t.Helper()

	return []callbackContextUnreadableHeader{
		{
			name: "the struct renamed",
			header: mustRewrite(t, header,
				"} "+callbackContextTypedef+";",
				"} overlayCallbackContext;",
			),
		},
		{
			name: "the struct given a tag instead of a typedef name",
			header: mustRewrite(t, header,
				"typedef struct {",
				"struct callbackContext {",
			),
		},
		{
			name: "a field of a width this pin does not read",
			header: mustRewrite(t, header,
				"uint64_t callbackID;",
				"unsigned long callbackID;",
			),
		},
		{
			name: "a field whose width is hidden behind an alias",
			header: mustRewrite(t, header,
				"uint64_t generation;",
				"neru_generation_t generation;",
			),
		},
		{
			name: "the struct emptied",
			header: mustRewrite(t, header,
				"\tuint64_t callbackID;\n\tuint64_t generation;\n",
				"",
			),
		},
	}
}

// callbackContextProblems is every disagreement between the two declarations.
//
// Fields are compared by position, because position is what the cast reads: two
// uint64 fields swapped is a callback ID validated as a generation, and no
// count or name check would see it.
func callbackContextProblems(native, goSide []callbackContextField) []string {
	var problems []string

	if len(native) != len(goSide) {
		problems = append(problems, fmt.Sprintf(
			"%s declares %d field(s) (%s) and %s has %d (%s); "+
				"the C struct is cast straight to the Go one, so a field on one side only "+
				"shifts every field after it",
			callbackContextHeader, len(native), callbackContextFieldList(native, nativeIntSuffix),
			callbackContextGoType, len(goSide), callbackContextFieldList(goSide, ""),
		))
	}

	for index := range min(len(native), len(goSide)) {
		problems = append(
			problems,
			callbackContextFieldProblems(index, native[index], goSide[index])...)
	}

	return problems
}

// callbackContextFieldProblems compares the two declarations' field at one
// position.
func callbackContextFieldProblems(index int, native, goSide callbackContextField) []string {
	var problems []string

	if want := nativeFieldSpelling(goSide.name); native.name != want {
		problems = append(problems, fmt.Sprintf(
			"field %d is %s.%s in Go and %s in %s; a renamed or reordered field is read "+
				"from the offset the other one is written to",
			index, callbackContextGoType, goSide.name, native.name, callbackContextHeader,
		))
	}

	if native.goType != goSide.goType {
		problems = append(problems, fmt.Sprintf(
			"field %d is %s in %s and %s%s in %s; the widths have to agree or every "+
				"field after it is read from the wrong offset",
			index, goSide.goType, callbackContextGoType,
			native.goType, nativeIntSuffix, callbackContextHeader,
		))
	}

	return problems
}

// callbackContextFieldList spells a declaration for a failure message, in the
// vocabulary that declaration is written in: suffix is what its language spells
// a fixed-width integer with, so the C side reads uint64_t and the Go side
// uint64 rather than both reading whichever this pin compares them as.
func callbackContextFieldList(fields []callbackContextField, suffix string) string {
	if len(fields) == 0 {
		return "none"
	}

	spelled := make([]string, 0, len(fields))
	for _, field := range fields {
		spelled = append(spelled, field.goType+suffix+" "+field.name)
	}

	return strings.Join(spelled, ", ")
}

// nativeFieldSpelling spells a Go field name the way the C struct writes it:
// "CallbackID" becomes "callbackID". The C side lowercases the initial and
// keeps the rest, which is what both fields there do today. A Go field whose
// name does not survive that — a leading initialism like "ID" — would fail
// here, and the fix is to spell the rule for it rather than to loosen it.
//
// This is not nativeEnumMemberName (native_constants_test.go) wearing a
// different name: that one asks how Objective-C spells a value a *person*
// writes in the config, and answers by capitalising. This asks how C spells a
// Go struct field, and answers by lowercasing. One question each, opposite
// directions.
func nativeFieldSpelling(goName string) string {
	if goName == "" {
		return ""
	}

	return strings.ToLower(goName[:1]) + goName[1:]
}

// callbackContextStructPattern matches the typedef this pin reads, by the name
// it is typedef'd to. A struct renamed, given a tag instead, or split into
// members this pattern does not span is reported rather than passed over.
//
// The body admits no brace of its own, which is what makes the name the thing
// being addressed: were another struct to be declared above this one in the
// header, the match would begin at that one's opening brace and swallow it.
var callbackContextStructPattern = regexp.MustCompile(
	`typedef struct \{([^{}]*)\}[ \t]*` + regexp.QuoteMeta(callbackContextTypedef) + `;`,
)

// callbackContextFieldPattern matches one `uint64_t callbackID;` member. Only
// the fixed-width types <stdint.h> declares are read: those are the ones whose
// width is the same on every ABI, which is the whole reason the C side of a
// shared struct is written with them.
var callbackContextFieldPattern = regexp.MustCompile(
	`^(u?int(?:8|16|32|64))_t[ \t]+([A-Za-z_][A-Za-z0-9_]*);(?:[ \t]*//.*)?$`,
)

// readCallbackContextHeader reads the C declaration out of the tree, failing
// the test when it cannot.
func readCallbackContextHeader(t *testing.T) []callbackContextField {
	t.Helper()

	fields, problem := parseCallbackContextHeader(readNativeSource(t, callbackContextHeader))
	if problem != "" {
		t.Fatal(problem)
	}

	return fields
}

// parseCallbackContextHeader reads the struct's members in declaration order.
// The second result describes why they could not be read, and is empty when
// they could — an error value would buy nothing here, since the only callers
// turn it straight into a test failure.
func parseCallbackContextHeader(header string) ([]callbackContextField, string) {
	body := callbackContextStructPattern.FindStringSubmatch(header)
	if body == nil {
		return nil, fmt.Sprintf(
			"%s: no `typedef struct { ... } %s;` declaration (renamed or moved?); "+
				"the layout %s is cast to would be pinned by nothing",
			callbackContextHeader, callbackContextTypedef, callbackContextGoType,
		)
	}

	var fields []callbackContextField

	for line := range strings.SplitSeq(body[1], "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}

		member := callbackContextFieldPattern.FindStringSubmatch(trimmed)
		if member == nil {
			return nil, fmt.Sprintf(
				"%s: `%s` is a member this pin does not read; it reads "+
					"`<u>intN_t name;` members, whose width is the same on every ABI",
				callbackContextHeader, trimmed,
			)
		}

		fields = append(fields, callbackContextField{name: member[2], goType: member[1]})
	}

	if len(fields) == 0 {
		return nil, fmt.Sprintf(
			"%s: %s declares no members this pin can read",
			callbackContextHeader, callbackContextTypedef,
		)
	}

	return fields, ""
}

// goCallbackContextFields reads a Go struct's fields in declaration order.
//
// reflect is what the C side has no counterpart for and what the //export-ed
// cast depends on: the field order here is the order of the bytes, and the Kind
// is the real width even where the declaration spells it as a named type.
func goCallbackContextFields(t *testing.T, structType reflect.Type) []callbackContextField {
	t.Helper()

	if structType.Kind() != reflect.Struct {
		t.Fatalf("%s is a %s, not a struct", callbackContextGoType, structType.Kind())
	}

	fields := make([]callbackContextField, 0, structType.NumField())

	for field := range structType.Fields() {
		if field.Anonymous {
			t.Fatalf(
				"%s embeds %s; this pin reads named fields, because an embedded "+
					"struct's members are laid out here but written elsewhere",
				callbackContextGoType, field.Type,
			)
		}

		if !goFixedWidthKinds[field.Type.Kind()] {
			t.Fatalf(
				"%s.%s is a %s; this pin reads fixed-width integer fields, which are "+
					"the ones a C struct can be laid out to match",
				callbackContextGoType, field.Name, field.Type.Kind(),
			)
		}

		fields = append(fields, callbackContextField{
			name: field.Name, goType: field.Type.Kind().String(),
		})
	}

	return fields
}

// goFixedWidthKinds are the field kinds a struct shared with C may be laid out
// from, and each Kind names itself the way its declaration spells it. int, uint
// and uintptr are left out on purpose: their width is the platform's, which is
// exactly what such a struct must not depend on.
var goFixedWidthKinds = map[reflect.Kind]bool{
	reflect.Int8:   true,
	reflect.Int16:  true,
	reflect.Int32:  true,
	reflect.Int64:  true,
	reflect.Uint8:  true,
	reflect.Uint16: true,
	reflect.Uint32: true,
	reflect.Uint64: true,
}
