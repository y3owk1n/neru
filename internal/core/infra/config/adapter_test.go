package config_test

import (
	"context"
	"testing"

	configPkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/infra/config"
)

func TestNewAdapter(t *testing.T) {
	service := configPkg.NewService(configPkg.DefaultConfig(), "/test/path", nil)
	adapter := config.NewAdapter(service)

	if adapter == nil {
		t.Fatal("NewAdapter returned nil")
	}
}

func TestAdapter_Get(t *testing.T) {
	expectedConfig := configPkg.DefaultConfig()
	service := configPkg.NewService(expectedConfig, "/test/path", nil)
	adapter := config.NewAdapter(service)

	result := adapter.Get()

	if result != expectedConfig {
		t.Errorf("Expected config %v, got %v", expectedConfig, result)
	}
}

func TestAdapter_Path(t *testing.T) {
	expectedPath := "/test/config.toml"
	service := configPkg.NewService(configPkg.DefaultConfig(), expectedPath, nil)
	adapter := config.NewAdapter(service)

	result := adapter.Path()

	if result != expectedPath {
		t.Errorf("Expected path %s, got %s", expectedPath, result)
	}
}

func TestAdapter_Reload(t *testing.T) {
	service := configPkg.NewService(configPkg.DefaultConfig(), "/test/path", nil)
	adapter := config.NewAdapter(service)

	ctx := context.Background()
	err := adapter.Reload(ctx, "/nonexistent/path")
	// Reload now returns an error for explicitly specified missing config files
	if err == nil {
		t.Error("Expected error when reloading nonexistent config file, got nil")
	}
}

func TestAdapter_Validate(t *testing.T) {
	service := configPkg.NewService(configPkg.DefaultConfig(), "/test/path", nil)
	adapter := config.NewAdapter(service)

	validConfig := configPkg.DefaultConfig()

	err := adapter.Validate(validConfig)
	if err != nil {
		t.Errorf("Expected no error for valid config, got %v", err)
	}

	// Create invalid config: hints enabled but insufficient characters
	invalidConfig := configPkg.DefaultConfig()
	invalidConfig.Hints.Enabled = true
	invalidConfig.Hints.HintCharacters = "a" // Only 1 character, needs >=2

	err = adapter.Validate(invalidConfig)
	if err == nil {
		t.Error("Expected error for invalid config with insufficient hint characters")
	}

	// Create invalid config: hints enabled but empty clickable roles
	invalidConfig2 := configPkg.DefaultConfig()
	invalidConfig2.Hints.Enabled = true
	invalidConfig2.Hints.ClickableRoles = []string{} // Empty roles

	err = adapter.Validate(invalidConfig2)
	if err == nil {
		t.Error("Expected error for invalid config with empty clickable roles")
	}

	// Create invalid config: grid enabled but insufficient characters
	invalidConfig3 := configPkg.DefaultConfig()
	invalidConfig3.Grid.Enabled = true
	invalidConfig3.Grid.Characters = "a" // Only 1 character, needs >=2

	err = adapter.Validate(invalidConfig3)
	if err == nil {
		t.Error("Expected error for invalid config with insufficient grid characters")
	}
}

func TestAdapter_Watch(t *testing.T) {
	service := configPkg.NewService(configPkg.DefaultConfig(), "/test/path", nil)
	adapter := config.NewAdapter(service)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	watchCh := adapter.Watch(ctx)

	// Should receive initial config
	select {
	case cfg := <-watchCh:
		if cfg == nil {
			t.Error("Expected non-nil config from watch channel")
		}
	default:
		t.Error("Expected initial config from watch channel")
	}

	// Cancel context and check channel closes
	cancel()

	select {
	case _, ok := <-watchCh:
		if ok {
			t.Error("Expected watch channel to be closed")
		}
	default:
		// Channel already closed, which is fine
	}
}
