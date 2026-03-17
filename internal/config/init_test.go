package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/y3owk1n/neru/internal/config"
)

func TestWriteDefaultConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")

	err := config.WriteDefaultConfig(cfgPath, false)
	require.NoError(t, err)

	info, statErr := os.Stat(cfgPath)
	require.NoError(t, statErr)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())

	content, readErr := os.ReadFile(cfgPath)
	require.NoError(t, readErr)
	assert.NotEmpty(t, content)
}

func TestWriteDefaultConfig_ExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")

	err := os.WriteFile(cfgPath, []byte("existing"), 0o644)
	require.NoError(t, err)

	err = config.WriteDefaultConfig(cfgPath, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestWriteDefaultConfig_ForceOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")

	err := os.WriteFile(cfgPath, []byte("existing"), 0o644)
	require.NoError(t, err)

	err = config.WriteDefaultConfig(cfgPath, true)
	require.NoError(t, err)

	content, readErr := os.ReadFile(cfgPath)
	require.NoError(t, readErr)
	assert.NotEqual(t, "existing", string(content))
}

func TestWriteDefaultConfig_CreatesParentDirs(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "subdir", "config.toml")

	err := config.WriteDefaultConfig(cfgPath, false)
	require.NoError(t, err)

	info, statErr := os.Stat(filepath.Dir(cfgPath))
	require.NoError(t, statErr)
	assert.True(t, info.IsDir())
}

func TestDefaultConfigPath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/custom/config")

	path, err := config.DefaultConfigPath()
	require.NoError(t, err)
	assert.Equal(t, "/custom/config/neru/config.toml", path)
}

func TestDefaultConfigPath_NoXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/home/user")

	path, err := config.DefaultConfigPath()
	require.NoError(t, err)
	assert.Equal(t, "/home/user/.config/neru/config.toml", path)
}
