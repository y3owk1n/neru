package hint

import (
	"context"
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/element"
)

// BenchmarkAlphabetGenerator_Generate benchmarks hint label generation.
func BenchmarkAlphabetGenerator_Generate(b *testing.B) {
	gen, _ := NewAlphabetGenerator("asdfghjkl")
	ctx := context.Background()

	// Create test elements
	elements := make([]*element.Element, 100)
	for i := 0; i < 100; i++ {
		elem, _ := element.NewElement(
			element.ID("test-id"),
			image.Rect(i*10, i*10, i*10+50, i*10+50),
			element.RoleButton,
		)
		elements[i] = elem
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = gen.Generate(ctx, elements)
	}
}

// BenchmarkAlphabetGenerator_Generate_Large benchmarks with many elements.
func BenchmarkAlphabetGenerator_Generate_Large(b *testing.B) {
	gen, _ := NewAlphabetGenerator("asdfghjkl")
	ctx := context.Background()

	// Create 1000 test elements
	elements := make([]*element.Element, 1000)
	for i := 0; i < 1000; i++ {
		elem, _ := element.NewElement(
			element.ID("test-id"),
			image.Rect(i*10, i*10, i*10+50, i*10+50),
			element.RoleButton,
		)
		elements[i] = elem
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = gen.Generate(ctx, elements)
	}
}

// BenchmarkCollection_FilterByPrefix benchmarks hint filtering.
func BenchmarkCollection_FilterByPrefix(b *testing.B) {
	gen, _ := NewAlphabetGenerator("asdfghjkl")
	ctx := context.Background()

	// Create test elements
	elements := make([]*element.Element, 100)
	for i := 0; i < 100; i++ {
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
	for i := 0; i < b.N; i++ {
		_ = collection.FilterByPrefix("a")
	}
}
