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
	svc := config.NewService(cfg, path)

	if svc == nil {
		t.Fatal("NewService returned nil")
	}

	if svc.Get() != cfg {
		t.Error("Get() returned incorrect config")
	}

	if svc.Path() != path {
		t.Errorf("Path() = %v, want %v", svc.Path(), path)
	}
}

func TestService_Validate(t *testing.T) {
	svc := config.NewService(config.DefaultConfig(), "")

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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.Validate(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestService_Update(t *testing.T) {
	svc := config.NewService(config.DefaultConfig(), "")
	newCfg := config.DefaultConfig()
	newCfg.Hints.HintCharacters = "xyz"

	err := svc.Update(newCfg)
	if err != nil {
		t.Fatalf("Update() failed: %v", err)
	}

	if svc.Get().Hints.HintCharacters != "xyz" {
		t.Error("Update() did not update config")
	}

	// Test invalid update
	invalidCfg := config.DefaultConfig()
	invalidCfg.Hints.Enabled = true
	invalidCfg.Hints.HintCharacters = "a" // Invalid

	err = svc.Update(invalidCfg)
	if err == nil {
		t.Error("Update() should fail with invalid config")
	}

	// Ensure config wasn't updated on error
	if svc.Get().Hints.HintCharacters == "a" {
		t.Error("Update() updated config despite validation error")
	}
}

func TestService_Watch(t *testing.T) {
	svc := config.NewService(config.DefaultConfig(), "")
	ctx := t.Context()

	ch := svc.Watch(ctx)

	// Should receive initial config
	select {
	case cfg := <-ch:
		if cfg == nil {
			t.Error("Watch channel received nil config")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Watch channel did not receive initial config")
	}

	// Update config
	newCfg := config.DefaultConfig()
	newCfg.Hints.HintCharacters = "abc"
	_ = svc.Update(newCfg)

	// Should receive update
	select {
	case cfg := <-ch:
		if cfg.Hints.HintCharacters != "abc" {
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
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write temp config: %v", err)
	}

	svc := config.NewService(config.DefaultConfig(), configPath)

	// Test Reload
	ctx := context.Background()
	err = svc.Reload(ctx, configPath)
	if err != nil {
		t.Fatalf("Reload() failed: %v", err)
	}

	if svc.Get().Hints.HintCharacters != "asdf" {
		t.Errorf("Reload() did not load correct config, got %v", svc.Get().Hints.HintCharacters)
	}

	// Test Reload with invalid file
	err = os.WriteFile(configPath, []byte("invalid toml content"), 0o644)
	if err != nil {
		t.Fatalf("Failed to update temp config: %v", err)
	}

	err = svc.Reload(ctx, configPath)
	if err == nil {
		t.Error("Reload() should fail with invalid config file")
	}
}

func TestService_Concurrency(_ *testing.T) {
	svc := config.NewService(config.DefaultConfig(), "")
	var wg sync.WaitGroup

	// Concurrent reads
	for range 100 {
		//nolint:modernize // WaitGroup.Go is not available in standard library
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = svc.Get()
		}()
	}

	// Concurrent updates
	for i := range 100 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			cfg := config.DefaultConfig()
			if id%2 == 0 {
				cfg.Hints.HintCharacters = "even"
			} else {
				cfg.Hints.HintCharacters = "odd"
			}
			_ = svc.Update(cfg)
		}(i)
	}

	wg.Wait()
}
