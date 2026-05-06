package config_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

const (
	invalidRecursiveGridKeys           = "abc"
	validRecursiveGridSingleColumnKeys = "abc"
)

func TestConfigValidateRecursiveGrid_Valid(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.RecursiveGrid.Enabled = true
	cfg.RecursiveGrid.GridCols = 2
	cfg.RecursiveGrid.GridRows = 2
	cfg.RecursiveGrid.Keys = "uijk"

	err := cfg.ValidateRecursiveGrid()
	if err != nil {
		t.Fatalf("ValidateRecursiveGrid() unexpected error: %v", err)
	}
}

func TestConfigValidateRecursiveGrid_SingleDimensionLayoutsValid(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.RecursiveGrid.Enabled = true
	cfg.RecursiveGrid.GridCols = 1
	cfg.RecursiveGrid.GridRows = 3
	cfg.RecursiveGrid.Keys = validRecursiveGridSingleColumnKeys
	cfg.RecursiveGrid.Layers = []config.RecursiveGridLayerConfig{
		{
			Depth:    1,
			GridCols: 4,
			GridRows: 1,
			Keys:     "hjkl",
		},
	}

	err := cfg.ValidateRecursiveGrid()
	if err != nil {
		t.Fatalf("ValidateRecursiveGrid() unexpected error for 1xN/Nx1 layouts: %v", err)
	}
}

func TestConfigValidateRecursiveGrid_1x1_Invalid(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.RecursiveGrid.Enabled = true
	cfg.RecursiveGrid.GridCols = 1
	cfg.RecursiveGrid.GridRows = 1
	cfg.RecursiveGrid.Keys = "a"

	err := cfg.ValidateRecursiveGrid()
	if err == nil {
		t.Fatal("ValidateRecursiveGrid() expected error for degenerate 1x1 grid")
	}
}

func TestConfigValidateRecursiveGrid_InvalidKeyLength(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.RecursiveGrid.Enabled = true
	cfg.RecursiveGrid.GridCols = 2
	cfg.RecursiveGrid.GridRows = 2
	cfg.RecursiveGrid.Keys = invalidRecursiveGridKeys

	err := cfg.ValidateRecursiveGrid()
	if err == nil {
		t.Fatal("ValidateRecursiveGrid() expected error for invalid key length")
	}
}

func TestConfigValidateRecursiveGrid_SmallMinSizeAllowed(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.RecursiveGrid.Enabled = true
	cfg.RecursiveGrid.MinSizeWidth = 5
	cfg.RecursiveGrid.MinSizeHeight = 5

	err := cfg.ValidateRecursiveGrid()
	if err != nil {
		t.Fatalf("ValidateRecursiveGrid() unexpected error for small min sizes: %v", err)
	}
}

func TestDefaultConfigRecursiveGridAnimationDisabled(t *testing.T) {
	cfg := config.DefaultConfig()

	if cfg.RecursiveGrid.Animation.Enabled {
		t.Fatal("DefaultConfig() recursive_grid.animation.enabled should default to false")
	}

	if cfg.RecursiveGrid.Animation.DurationMS != config.DefaultRecursiveGridAnimationDurationMS {
		t.Fatalf(
			"DefaultConfig() recursive_grid.animation.duration_ms = %d, want %d",
			cfg.RecursiveGrid.Animation.DurationMS,
			config.DefaultRecursiveGridAnimationDurationMS,
		)
	}
}

func TestConfigValidateRecursiveGrid_InvalidAnimationDuration(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.RecursiveGrid.Enabled = true
	cfg.RecursiveGrid.Animation.DurationMS = -1

	err := cfg.ValidateRecursiveGrid()
	if err == nil {
		t.Fatal("ValidateRecursiveGrid() expected error for negative animation duration")
	}
}

func TestConfigValidateRecursiveGrid_InvalidTrainingHitsToHide(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.RecursiveGrid.Enabled = true
	cfg.RecursiveGrid.Training.HitsToHide = 0

	err := cfg.ValidateRecursiveGrid()
	if err == nil {
		t.Fatal("ValidateRecursiveGrid() expected error for hits_to_hide < 1")
	}
}

func TestConfigValidateRecursiveGrid_InvalidTrainingPenaltyOnError(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.RecursiveGrid.Enabled = true
	cfg.RecursiveGrid.Training.PenaltyOnError = -1

	err := cfg.ValidateRecursiveGrid()
	if err == nil {
		t.Fatal("ValidateRecursiveGrid() expected error for penalty_on_error < 0")
	}
}
