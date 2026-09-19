package fontcache_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/platform/fontcache"
	"github.com/y3owk1n/neru/internal/ports"
)

// menlo is the family the measurer tests ask about. Which family it is does not
// matter, because the wrapped measurement is a fake.
const menlo = "Menlo Regular"

// errTextLayerUnavailable is what the fake measurement fails with.
var errTextLayerUnavailable = errors.New("text layer unavailable")

func TestMeasurer_MeasuresOncePerStringAndFont(t *testing.T) {
	calls := 0
	measurer := fontcache.NewMeasurer(
		func(text, _ string, size float64, _ bool) (ports.TextMetrics, error) {
			calls++

			return ports.TextMetrics{Width: float64(len(text)) * size, Height: size}, nil
		},
	)

	for range 3 {
		got, err := measurer.Measure("AB", menlo, 10, false)
		if err != nil {
			t.Fatalf("Measure() error = %v", err)
		}

		if want := (ports.TextMetrics{Width: 20, Height: 10}); got != want {
			t.Fatalf("Measure() = %+v, want %+v", got, want)
		}
	}

	if calls != 1 {
		t.Fatalf("measured %d times, want 1", calls)
	}

	// Every part of the key names a different measurement.
	for _, other := range []struct {
		text, family string
		size         float64
		bold         bool
	}{
		{"ABC", menlo, 10, false},
		{"AB", "Monaco", 10, false},
		{"AB", menlo, 11, false},
		{"AB", menlo, 10, true},
	} {
		_, err := measurer.Measure(other.text, other.family, other.size, other.bold)
		if err != nil {
			t.Fatalf("Measure() error = %v", err)
		}
	}

	if calls != 5 {
		t.Fatalf("measured %d times, want 5", calls)
	}
}

func TestMeasurer_FailureIsNotRemembered(t *testing.T) {
	failing := true
	measurer := fontcache.NewMeasurer(
		func(string, string, float64, bool) (ports.TextMetrics, error) {
			if failing {
				return ports.TextMetrics{}, errTextLayerUnavailable
			}

			return ports.TextMetrics{Width: 7, Height: 9}, nil
		},
	)

	_, err := measurer.Measure("A", menlo, 10, false)
	if !errors.Is(err, errTextLayerUnavailable) {
		t.Fatalf("Measure() error = %v, want %v", err, errTextLayerUnavailable)
	}

	failing = false

	got, err := measurer.Measure("A", menlo, 10, false)
	if err != nil {
		t.Fatalf("Measure() after recovery error = %v", err)
	}

	if want := (ports.TextMetrics{Width: 7, Height: 9}); got != want {
		t.Fatalf("Measure() after recovery = %+v, want %+v", got, want)
	}
}

func TestMeasurer_ConcurrentMeasureIsSafe(t *testing.T) {
	measurer := fontcache.NewMeasurer(
		func(_, _ string, size float64, _ bool) (ports.TextMetrics, error) {
			return ports.TextMetrics{Width: size, Height: size}, nil
		},
	)

	var group sync.WaitGroup

	for range 16 {
		group.Go(func() {
			for size := 1; size < 50; size++ {
				got, err := measurer.Measure("A", menlo, float64(size), false)
				if err != nil || got.Width != float64(size) {
					t.Errorf("Measure() = %+v, %v", got, err)

					return
				}
			}
		})
	}

	group.Wait()
}

func TestMeasurer_WhatItRemembersIsBounded(t *testing.T) {
	calls := 0
	measurer := fontcache.NewMeasurer(
		func(_, _ string, size float64, _ bool) (ports.TextMetrics, error) {
			calls++

			return ports.TextMetrics{Width: size, Height: size}, nil
		},
	)

	// Far more distinct measurements than it holds: somebody trying one font
	// size after another for the daemon's lifetime.
	for size := 1; size <= 10000; size++ {
		_, err := measurer.Measure("A", menlo, float64(size), false)
		if err != nil {
			t.Fatalf("Measure() error = %v", err)
		}
	}

	before := calls

	_, err := measurer.Measure("A", menlo, 1, false)
	if err != nil {
		t.Fatalf("Measure() error = %v", err)
	}

	if calls == before {
		t.Fatal("a measurement from 10000 measurements ago was still held")
	}
}
