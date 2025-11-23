package hint

import (
	"context"
	"fmt" // Added for fmt.Sprintf
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/element"
)

func BenchmarkAlphabetGenerator_Generate_Alloc(b *testing.B) {
	gen, _ := NewAlphabetGenerator("asdfghjkl")
	ctx := context.Background()
	elements := make([]*element.Element, 1000)
	for i := range 1000 {
		elem, _ := element.NewElement(
			element.ID(fmt.Sprintf("test-id-%d", i)),
			image.Rect(i, i, i+50, i+50),
			element.RoleButton,
		)
		elements[i] = elem
	}

	b.ResetTimer()
	for range 100 {
		_, _ = gen.Generate(ctx, elements)
	}
}

func BenchmarkAlphabetGenerator_Generate_Small(b *testing.B) {
	gen, _ := NewAlphabetGenerator("abcdefghijklmnopqrstuvwxyz")
	ctx := context.Background()
	elements := make([]*element.Element, 10)
	for i := range 10 {
		elem, _ := element.NewElement(
			element.ID(fmt.Sprintf("test-id-%d", i)),
			image.Rect(i*10, i*10, i*10+50, i*10+50),
			element.RoleButton,
		)
		elements[i] = elem
	}

	b.ResetTimer()
	for b.Loop() {
		_, _ = gen.Generate(ctx, elements)
	}
}

func BenchmarkAlphabetGenerator_Generate_Small_WithElements(b *testing.B) {
	gen, _ := NewAlphabetGenerator("abcdefghijklmnopqrstuvwxyz")
	ctx := context.Background()
	elements := make([]*element.Element, 10)
	for i := range 10 {
		elem, _ := element.NewElement(
			element.ID("test-id"),
			image.Rect(i*10, i*10, i*10+50, i*10+50),
			element.RoleButton,
		)
		elements[i] = elem
	}

	b.ResetTimer()
	for b.Loop() {
		_, _ = gen.Generate(ctx, elements)
	}
}

func BenchmarkAlphabetGenerator_Generate_Large_Alloc(b *testing.B) {
	gen, _ := NewAlphabetGenerator("abcdefghijklmnopqrstuvwxyz")
	ctx := context.Background()
	elements := make([]*element.Element, 1000)
	for i := range 1000 {
		elem, _ := element.NewElement(
			element.ID(fmt.Sprintf("test-id-%d", i)),
			image.Rect(i, i, i+50, i+50),
			element.RoleButton,
		)
		elements[i] = elem
	}

	b.ResetTimer()
	for range 1000 {
		_, _ = gen.Generate(ctx, elements)
	}
}

func BenchmarkAlphabetGenerator_Generate_Large(b *testing.B) {
	gen, _ := NewAlphabetGenerator("abcdefghijklmnopqrstuvwxyz")
	ctx := context.Background()
	elements := make([]*element.Element, 1000)
	for i := range 1000 {
		elem, _ := element.NewElement(
			element.ID(fmt.Sprintf("test-id-%d", i)),
			image.Rect(i, i, i+50, i+50),
			element.RoleButton,
		)
		elements[i] = elem
	}

	b.ResetTimer()
	for b.Loop() {
		_, _ = gen.Generate(ctx, elements)
	}
}

// BenchmarkAlphabetGenerator_Generate_Small benchmarks with a small number of elements.
func BenchmarkAlphabetGenerator_Generate_Small_New(b *testing.B) {
	gen, _ := NewAlphabetGenerator("abcdefghijklmnopqrstuvwxyz")
	ctx := context.Background()

	elements := make([]*element.Element, 10)
	for i := range 10 {
		elem, _ := element.NewElement(
			element.ID("test-id"),
			image.Rect(i*10, i*10, i*10+50, i*10+50),
			element.RoleButton,
		)
		elements[i] = elem
	}

	b.ResetTimer()
	for b.Loop() {
		_, _ = gen.Generate(ctx, elements)
	}
}

// BenchmarkAlphabetGenerator_Generate_Large benchmarks with many elements.
func BenchmarkAlphabetGenerator_Generate_Large_New(b *testing.B) {
	// Use full alphabet to support >1000 elements (26^2 = 676, 26^3 = 17576)
	gen, _ := NewAlphabetGenerator("abcdefghijklmnopqrstuvwxyz")
	ctx := context.Background()

	// Create 1000 dummy elements
	// Create 1000 dummy elements
	elements := make([]*element.Element, 1000)
	for i := range 1000 {
		elem, _ := element.NewElement(
			element.ID(fmt.Sprintf("test-id-%d", i)),
			image.Rect(i, i, i+50, i+50),
			element.RoleButton,
		)
		elements[i] = elem
	}

	b.ResetTimer()
	for range 100 {
		_, _ = gen.Generate(ctx, elements)
	}
}

// BenchmarkCollection_FilterByPrefix benchmarks hint filtering.
func BenchmarkCollection_FilterByPrefix(b *testing.B) {
	gen, _ := NewAlphabetGenerator("asdfghjkl")
	ctx := context.Background()

	// Create test elements
	elements := make([]*element.Element, 100)
	for i := range 100 {
		elem, _ := element.NewElement(
			element.ID("test-id"),
			image.Rect(i*10, i*10, i*10+50, i*10+50),
			element.RoleButton,
		)
		elements[i] = elem
	}

	hints, _ := gen.Generate(ctx, elements)
	collection := NewCollection(hints)

	b.ResetTimer()
	for b.Loop() {
		_ = collection.FilterByPrefix("a")
	}
}
