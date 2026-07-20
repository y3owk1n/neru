package config_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

const (
	testHintChars = "qwerty"
	testColorHex  = "#FF0000AA"
)

func TestSetField_String(t *testing.T) {
	cfg := config.DefaultConfig()

	err := config.SetField(cfg, "hints.hint_characters", testHintChars)
	if err != nil {
		t.Fatalf("SetField() unexpected error: %v", err)
	}

	if cfg.Hints.HintCharacters != testHintChars {
		t.Fatalf("Expected hint_characters=%q, got %q", testHintChars, cfg.Hints.HintCharacters)
	}
}

func TestSetField_Integer(t *testing.T) {
	cfg := config.DefaultConfig()

	err := config.SetField(cfg, "hints.ui.font_size", "14")
	if err != nil {
		t.Fatalf("SetField() unexpected error: %v", err)
	}

	if cfg.Hints.UI.FontSize != 14 {
		t.Fatalf("Expected font_size=14, got %d", cfg.Hints.UI.FontSize)
	}
}

func TestSetField_Bool(t *testing.T) {
	cfg := config.DefaultConfig()

	err := config.SetField(cfg, "general.passthrough_unbounded_keys", "true")
	if err != nil {
		t.Fatalf("SetField() unexpected error: %v", err)
	}

	if !cfg.General.PassthroughUnboundedKeys {
		t.Fatal("Expected passthrough_unbounded_keys=true")
	}
}

func TestSetField_Float(t *testing.T) {
	cfg := config.DefaultConfig()

	err := config.SetField(cfg, "hints.vision.minimum_confidence", "0.5")
	if err != nil {
		t.Fatalf("SetField() unexpected error: %v", err)
	}

	if cfg.Hints.Vision.MinimumConfidence != 0.5 {
		t.Fatalf("Expected minimum_confidence=0.5, got %f", cfg.Hints.Vision.MinimumConfidence)
	}
}

func TestSetField_StringSlice(t *testing.T) {
	cfg := config.DefaultConfig()

	err := config.SetField(cfg, "hints.clickable_roles", "AXButton,AXLink")
	if err != nil {
		t.Fatalf("SetField() unexpected error: %v", err)
	}

	expected := []string{"AXButton", "AXLink"}
	if len(cfg.Hints.ClickableRoles) != len(expected) {
		t.Fatalf("Expected %d roles, got %d", len(expected), len(cfg.Hints.ClickableRoles))
	}

	for i, role := range expected {
		if cfg.Hints.ClickableRoles[i] != role {
			t.Fatalf("Expected role[%d]=%q, got %q", i, role, cfg.Hints.ClickableRoles[i])
		}
	}
}

func TestSetField_Color(t *testing.T) {
	cfg := config.DefaultConfig()

	err := config.SetField(cfg, "hints.ui.background_color", testColorHex)
	if err != nil {
		t.Fatalf("SetField() unexpected error: %v", err)
	}

	if cfg.Hints.UI.BackgroundColor.Light != testColorHex {
		t.Fatalf(
			"Expected background_color light=%q, got %q",
			testColorHex,
			cfg.Hints.UI.BackgroundColor.Light,
		)
	}

	if cfg.Hints.UI.BackgroundColor.Dark != testColorHex {
		t.Fatalf(
			"Expected background_color dark=%q, got %q",
			testColorHex,
			cfg.Hints.UI.BackgroundColor.Dark,
		)
	}
}

func TestSetField_ColorJSON(t *testing.T) {
	cfg := config.DefaultConfig()

	err := config.SetField(
		cfg,
		"hints.ui.background_color",
		`{"light":"#000","dark":"#FFF"}`,
	)
	if err != nil {
		t.Fatalf("SetField() unexpected error: %v", err)
	}

	if cfg.Hints.UI.BackgroundColor.Light != "#000" {
		t.Fatalf(
			"Expected background_color light=%q, got %q",
			"#000",
			cfg.Hints.UI.BackgroundColor.Light,
		)
	}

	if cfg.Hints.UI.BackgroundColor.Dark != "#FFF" {
		t.Fatalf(
			"Expected background_color dark=%q, got %q",
			"#FFF",
			cfg.Hints.UI.BackgroundColor.Dark,
		)
	}
}

func TestSetField_InvalidPath(t *testing.T) {
	cfg := config.DefaultConfig()

	err := config.SetField(cfg, "hints.nonexistent_field", "value")
	if err == nil {
		t.Fatal("Expected error for invalid path, got nil")
	}
}

func TestSetField_EmptyPath(t *testing.T) {
	cfg := config.DefaultConfig()

	err := config.SetField(cfg, "", "value")
	if err == nil {
		t.Fatal("Expected error for empty path, got nil")
	}
}

func TestSetField_InvalidIntegerValue(t *testing.T) {
	cfg := config.DefaultConfig()

	err := config.SetField(cfg, "hints.ui.font_size", "not-a-number")
	if err == nil {
		t.Fatal("Expected error for invalid integer value, got nil")
	}
}

func TestSetField_InvalidBoolValue(t *testing.T) {
	cfg := config.DefaultConfig()

	err := config.SetField(cfg, "general.passthrough_unbounded_keys", "not-a-bool")
	if err == nil {
		t.Fatal("Expected error for invalid boolean value, got nil")
	}
}

func TestDeepCopyConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Hints.HintCharacters = testHintChars

	deepCopy, err := config.DeepCopyConfig(cfg)
	if err != nil {
		t.Fatalf("DeepCopyConfig() unexpected error: %v", err)
	}

	if deepCopy.Hints.HintCharacters != testHintChars {
		t.Fatalf(
			"Expected copy hint_characters=%q, got %q",
			testHintChars,
			deepCopy.Hints.HintCharacters,
		)
	}

	// Ensure it's a deep copy — modifying the copy should not affect the original
	deepCopy.Hints.HintCharacters = "asdf"
	if cfg.Hints.HintCharacters != testHintChars {
		t.Fatal("DeepCopyConfig() did not produce an independent copy")
	}
}

func TestValidateConfigSetField_Valid(t *testing.T) {
	err := config.ValidateConfigSetField("hints.hint_characters", testHintChars)
	if err != nil {
		t.Fatalf("ValidateConfigSetField() unexpected error: %v", err)
	}
}

func TestValidateConfigSetField_InvalidPath(t *testing.T) {
	err := config.ValidateConfigSetField("hints.nonexistent", "value")
	if err == nil {
		t.Fatal("Expected error for invalid path")
	}
}

func TestConfigFieldType_String(t *testing.T) {
	typeHint := config.ConfigFieldType("hints.hint_characters")
	if typeHint != "string" {
		t.Fatalf("Expected type 'string', got %q", typeHint)
	}
}

func TestConfigFieldType_Integer(t *testing.T) {
	typeHint := config.ConfigFieldType("hints.ui.font_size")
	if typeHint != "integer" {
		t.Fatalf("Expected type 'integer', got %q", typeHint)
	}
}

func TestConfigFieldType_Bool(t *testing.T) {
	typeHint := config.ConfigFieldType("general.passthrough_unbounded_keys")
	if typeHint != "boolean" {
		t.Fatalf("Expected type 'boolean', got %q", typeHint)
	}
}

func TestConfigFieldType_Float(t *testing.T) {
	typeHint := config.ConfigFieldType("hints.vision.minimum_confidence")
	if typeHint != "float" {
		t.Fatalf("Expected type 'float', got %q", typeHint)
	}
}

func TestConfigFieldType_Array(t *testing.T) {
	typeHint := config.ConfigFieldType("hints.clickable_roles")
	if typeHint != "array" {
		t.Fatalf("Expected type 'array', got %q", typeHint)
	}
}

func TestConfigFieldType_Color(t *testing.T) {
	const want = "color (#RGB/#RRGGBB/#AARRGGBB or JSON object)"

	typeHint := config.ConfigFieldType("hints.ui.background_color")
	if typeHint != want {
		t.Fatalf("Expected type %q, got %q", want, typeHint)
	}
}

func TestConfigFieldType_Unknown(t *testing.T) {
	typeHint := config.ConfigFieldType("nonexistent.path")
	if typeHint != "unknown" {
		t.Fatalf("Expected 'unknown', got %q", typeHint)
	}
}
