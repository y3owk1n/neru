package hint_test

import (
	"context"
	"fmt"
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/element"
	"github.com/y3owk1n/neru/internal/domain/hint"
)

func BenchmarkAlphabetGenerator_Generate_Alloc(b *testing.B) {
	generator, _ := hint.NewAlphabetGenerator("asdfghjkl")
	ctx := context.Background()
	elements := make([]*element.Element, 1000)

	for index := range 1000 {
		element, _ := element.NewElement(
			element.ID(fmt.Sprintf("test-id-%d", index)),
			image.Rect(index, index, index+50, index+50),
			element.RoleButton,
		)
		elements[index] = element
	}

	b.ResetTimer()

	for range 100 {
		_, _ = generator.Generate(ctx, elements)
	}
}

func BenchmarkAlphabetGenerator_Generate_Small(b *testing.B) {
	generator, _ := hint.NewAlphabetGenerator("abcdefghijklmnopqrstuvwxyz")
	ctx := context.Background()
	elements := make([]*element.Element, 10)

	for index := range 10 {
		element, _ := element.NewElement(
			element.ID(fmt.Sprintf("test-id-%d", index)),
			image.Rect(index*10, index*10, index*10+50, index*10+50),
			element.RoleButton,
		)
		elements[index] = element
	}

	b.ResetTimer()

	for b.Loop() {
		_, _ = generator.Generate(ctx, elements)
	}
}

func BenchmarkAlphabetGenerator_Generate_Small_WithElements(b *testing.B) {
	generator, _ := hint.NewAlphabetGenerator("abcdefghijklmnopqrstuvwxyz")
	ctx := context.Background()
	elements := make([]*element.Element, 10)

	for index := range 10 {
		element, _ := element.NewElement(
			element.ID("test-id"),
			image.Rect(index*10, index*10, index*10+50, index*10+50),
			element.RoleButton,
		)
		elements[index] = element
	}

	b.ResetTimer()

	for b.Loop() {
		_, _ = generator.Generate(ctx, elements)
	}
}

func BenchmarkAlphabetGenerator_Generate_Large_Alloc(b *testing.B) {
	generator, _ := hint.NewAlphabetGenerator("abcdefghijklmnopqrstuvwxyz")
	ctx := context.Background()
	elements := make([]*element.Element, 1000)

	for index := range 1000 {
		element, _ := element.NewElement(
			element.ID(fmt.Sprintf("test-id-%d", index)),
			image.Rect(index, index, index+50, index+50),
			element.RoleButton,
		)
		elements[index] = element
	}

	b.ResetTimer()

	for range 1000 {
		_, _ = generator.Generate(ctx, elements)
	}
}

func BenchmarkAlphabetGenerator_Generate_Large(b *testing.B) {
	generator, _ := hint.NewAlphabetGenerator("abcdefghijklmnopqrstuvwxyz")
	ctx := context.Background()
	elements := make([]*element.Element, 1000)

	for index := range 1000 {
		element, _ := element.NewElement(
			element.ID(fmt.Sprintf("test-id-%d", index)),
			image.Rect(index, index, index+50, index+50),
			element.RoleButton,
		)
		elements[index] = element
	}

	b.ResetTimer()

	for b.Loop() {
		_, _ = generator.Generate(ctx, elements)
	}
}

// BenchmarkAlphabetGenerator_Generate_Small benchmarks with a small number of elements.
func BenchmarkAlphabetGenerator_Generate_Small_New(b *testing.B) {
	generator, _ := hint.NewAlphabetGenerator("abcdefghijklmnopqrstuvwxyz")
	ctx := context.Background()

	elements := make([]*element.Element, 10)

	for index := range 10 {
		element, _ := element.NewElement(
			element.ID("test-id"),
			image.Rect(index*10, index*10, index*10+50, index*10+50),
			element.RoleButton,
		)
		elements[index] = element
	}

	b.ResetTimer()

	for b.Loop() {
		_, _ = generator.Generate(ctx, elements)
	}
}

// BenchmarkAlphabetGenerator_Generate_Large benchmarks with many elements.
func BenchmarkAlphabetGenerator_Generate_Large_New(b *testing.B) {
	// Use full alphabet to support >1000 elements (26^2 = 676, 26^3 = 17576)
	generator, _ := hint.NewAlphabetGenerator("abcdefghijklmnopqrstuvwxyz")
	ctx := context.Background()

	// Create 1000 dummy elements
	elements := make([]*element.Element, 1000)

	for index := range 1000 {
		element, _ := element.NewElement(
			element.ID(fmt.Sprintf("test-id-%d", index)),
			image.Rect(index, index, index+50, index+50),
			element.RoleButton,
		)
		elements[index] = element
	}

	b.ResetTimer()

	for range 100 {
		_, _ = generator.Generate(ctx, elements)
	}
}

// BenchmarkCollection_FilterByPrefix benchmarks hint filtering.
func BenchmarkCollection_FilterByPrefix(b *testing.B) {
	generator, _ := hint.NewAlphabetGenerator("asdfghjkl")
	ctx := context.Background()

	// Create test elements
	elements := make([]*element.Element, 100)

	for index := range 100 {
		element, _ := element.NewElement(
			element.ID("test-id"),
			image.Rect(index*10, index*10, index*10+50, index*10+50),
			element.RoleButton,
		)
		elements[index] = element
	}

	hints, _ := generator.Generate(ctx, elements)
	collection := hint.NewCollection(hints)

	b.ResetTimer()

	for b.Loop() {
		_ = collection.FilterByPrefix("a")
	}
}
