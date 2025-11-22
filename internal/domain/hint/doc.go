// Package hint provides hint generation algorithms for UI elements.
//
// This package implements the core domain logic for generating keyboard hints
// that enable navigation to UI elements. Hints are generated using a configurable
// alphabet and are optimized to minimize typing distance.
//
// # Usage
//
//	gen, err := hint.NewAlphabetGenerator("asdfghjkl")
//	if err != nil {
//		return err
//	}
//
//	hints, err := gen.Generate(ctx, elements)
//	if err != nil {
//		return err
//	}
//
//	collection := hint.NewCollection(hints)
//	hint := collection.FindByLabel("AS")
//
// # Label Generation Strategy
//
// Labels are generated using a prefix-avoidance strategy to ensure no
// single-character label conflicts with the start of a multi-character label.
// This allows for immediate activation of single-character hints without
// waiting for additional input.
//
// For example, with alphabet "asdf":
//   - Two-char labels: AA, AS, AD, AF, SA, SS, SD, SF, ...
//   - Single-char labels: A, S, D, F
//
// # Performance
//
// The generator is optimized for low latency:
//   - Label generation: O(n) where n is the number of elements
//   - Collection lookup by label: O(1) using hash map
//   - Collection filter by prefix: O(1) for 1-2 char prefixes using indexes
//
// # Design Principles
//
//   - Immutability: Hints are immutable after creation
//   - No Dependencies: This package only depends on domain/element
//   - Context-Aware: All operations accept context for cancellation
//   - Validation: All inputs are validated at construction time
package hint
