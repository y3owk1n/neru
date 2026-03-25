package config_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

const invalidRecursiveGridKeys = "abc"

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
