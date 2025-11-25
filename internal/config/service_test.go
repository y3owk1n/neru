package config_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/config"
)

func TestNewService(t *testing.T) {
	cfg := config.DefaultConfig()
	path := "/tmp/config.toml"
	service := config.NewService(cfg, path)

	if service == nil {
		t.Fatal("NewService returned nil")
	}

	if service.Get() != cfg {
		t.Error("Get() returned incorrect config")
	}

	if service.Path() != path {
		t.Errorf("Path() = %v, want %v", service.Path(), path)
	}
}

func TestService_Validate(t *testing.T) {
	service := config.NewService(config.DefaultConfig(), "")

	tests := []struct {
		name    string
		cfg     *config.Config
		wantErr bool
	}{
		{
			name:    "nil config",
			cfg:     nil,
			wantErr: true,
		},
		{
			name:    "valid default config",
			cfg:     config.DefaultConfig(),
			wantErr: false,
		},
		{
			name: "invalid hints characters",
			cfg: func() *config.Config {
				c := config.DefaultConfig()
				c.Hints.Enabled = true
				c.Hints.HintCharacters = "a" // Too short

				return c
			}(),
			wantErr: true,
		},
		{
			name: "empty clickable roles",
			cfg: func() *config.Config {
				c := config.DefaultConfig()
				c.Hints.Enabled = true
				c.Hints.ClickableRoles = []string{}

				return c
			}(),
			wantErr: true,
		},
		{
			name: "invalid grid characters",
			cfg: func() *config.Config {
				c := config.DefaultConfig()
				c.Grid.Enabled = true
				c.Grid.Characters = "a" // Too short

				return c
			}(),
			wantErr: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			validateErr := service.Validate(testCase.cfg)
			if (validateErr != nil) != testCase.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", validateErr, testCase.wantErr)
			}
		})
	}
}

func TestService_Update(t *testing.T) {
	service := config.NewService(config.DefaultConfig(), "")
	newConfig := config.DefaultConfig()
	newConfig.Hints.HintCharacters = "xyz"

	updateErr := service.Update(newConfig)
	if updateErr != nil {
		t.Fatalf("Update() failed: %v", updateErr)
	}

	if service.Get().Hints.HintCharacters != "xyz" {
		t.Error("Update() did not update config")
	}

	// Test invalid update
	invalidConfig := config.DefaultConfig()
	invalidConfig.Hints.Enabled = true
	invalidConfig.Hints.HintCharacters = "a" // Invalid

	updateErr = service.Update(invalidConfig)
	if updateErr == nil {
		t.Error("Update() should fail with invalid config")
	}

	// Ensure config wasn't updated on error
	if service.Get().Hints.HintCharacters == "a" {
		t.Error("Update() updated config despite validation error")
	}
}

func TestService_Watch(t *testing.T) {
	service := config.NewService(config.DefaultConfig(), "")
	context := t.Context()

	channel := service.Watch(context)

	// Should receive initial config
	select {
	case config := <-channel:
		if config == nil {
			t.Error("Watch channel received nil config")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Watch channel did not receive initial config")
	}

	// Update config
	newConfig := config.DefaultConfig()
	newConfig.Hints.HintCharacters = "abc"
	_ = service.Update(newConfig)

	// Should receive update
	select {
	case config := <-channel:
		if config.Hints.HintCharacters != "abc" {
			t.Error("Watch channel received incorrect update")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Watch channel did not receive update")
	}
}

func TestService_Reload(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	configContent := `
[hints]
enabled = true
hint_characters = "asdf"
clickable_roles = ["AXButton"]
`

	writeFileErr := os.WriteFile(configPath, []byte(configContent), 0o644)
	if writeFileErr != nil {
		t.Fatalf("Failed to write temp config: %v", writeFileErr)
	}

	service := config.NewService(config.DefaultConfig(), configPath)

	// Test Reload
	context := context.Background()

	reloadErr := service.Reload(context, configPath)
	if reloadErr != nil {
		t.Fatalf("Reload() failed: %v", reloadErr)
	}

	if service.Get().Hints.HintCharacters != "asdf" {
		t.Errorf("Reload() did not load correct config, got %v", service.Get().Hints.HintCharacters)
	}

	// Test Reload with invalid file
	anotherWriteFileErr := os.WriteFile(configPath, []byte("invalid toml content"), 0o644)
	if anotherWriteFileErr != nil {
		t.Fatalf("Failed to update temp config: %v", anotherWriteFileErr)
	}

	anotherReloadErr := service.Reload(context, configPath)
	if anotherReloadErr == nil {
		t.Error("Reload() should fail with invalid config file")
	}
}

func TestService_Concurrency(_ *testing.T) {
	service := config.NewService(config.DefaultConfig(), "")

	var waitGroup sync.WaitGroup

	// Concurrent reads
	for range 100 {
		waitGroup.Go(func() {
			_ = service.Get()
		})
	}

	// Concurrent updates
	for index := range 100 {
		waitGroup.Add(1)

		go func(id int) {
			defer waitGroup.Done()

			cfg := config.DefaultConfig()
			if id%2 == 0 {
				cfg.Hints.HintCharacters = "even"
			} else {
				cfg.Hints.HintCharacters = "odd"
			}

			_ = service.Update(cfg)
		}(index)
	}

	waitGroup.Wait()
}
