package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextConfigManagement(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "rune-config-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Set the global config file to our temp directory
	originalCfgFile := cfgFile
	cfgFile = filepath.Join(tempDir, "config.yaml")
	defer func() { cfgFile = originalCfgFile }()

	t.Run("Create new config", func(t *testing.T) {
		config, err := loadContextConfig()
		require.NoError(t, err)
		assert.Equal(t, "default", config.CurrentContext)
		assert.Empty(t, config.Contexts)
	})

	t.Run("Add context", func(t *testing.T) {
		config, err := loadContextConfig()
		require.NoError(t, err)

		config.Contexts["production"] = Context{
			Server:           "https://prod-server.com:7863",
			Token:            "prod-token-123",
			DefaultNamespace: "prod",
		}

		err = saveContextConfig(config)
		require.NoError(t, err)

		// Verify it was saved
		loaded, err := loadContextConfig()
		require.NoError(t, err)
		assert.Equal(t, "https://prod-server.com:7863", loaded.Contexts["production"].Server)
		assert.Equal(t, "prod-token-123", loaded.Contexts["production"].Token)
		assert.Equal(t, "prod", loaded.Contexts["production"].DefaultNamespace)
	})

	t.Run("Update existing context", func(t *testing.T) {
		config, err := loadContextConfig()
		require.NoError(t, err)

		// Update the production context
		config.Contexts["production"] = Context{
			Server:           "https://new-prod-server.com:7863",
			Token:            "new-prod-token-456",
			DefaultNamespace: "new-prod",
		}

		err = saveContextConfig(config)
		require.NoError(t, err)

		// Verify it was updated
		loaded, err := loadContextConfig()
		require.NoError(t, err)
		assert.Equal(t, "https://new-prod-server.com:7863", loaded.Contexts["production"].Server)
		assert.Equal(t, "new-prod-token-456", loaded.Contexts["production"].Token)
		assert.Equal(t, "new-prod", loaded.Contexts["production"].DefaultNamespace)
	})

	t.Run("Switch current context", func(t *testing.T) {
		config, err := loadContextConfig()
		require.NoError(t, err)

		config.CurrentContext = "production"
		err = saveContextConfig(config)
		require.NoError(t, err)

		// Verify current context was switched
		loaded, err := loadContextConfig()
		require.NoError(t, err)
		assert.Equal(t, "production", loaded.CurrentContext)
	})

	t.Run("Delete context", func(t *testing.T) {
		config, err := loadContextConfig()
		require.NoError(t, err)

		// Switch to default first so we can delete production
		config.CurrentContext = "default"
		err = saveContextConfig(config)
		require.NoError(t, err)

		// Delete production context
		delete(config.Contexts, "production")
		err = saveContextConfig(config)
		require.NoError(t, err)

		// Verify it was deleted
		loaded, err := loadContextConfig()
		require.NoError(t, err)
		_, exists := loaded.Contexts["production"]
		assert.False(t, exists)
	})
}

func TestMaskToken(t *testing.T) {
	// Test that maskToken function exists and works
	// This function is defined in whoami.go, so we just verify it's accessible
	token := "very-long-token-string-12345"
	masked := maskToken(token)
	assert.NotEqual(t, token, masked)
	assert.Contains(t, masked, "******")
}

func TestGetConfigPath(t *testing.T) {
	// Test default config path
	originalCfgFile := cfgFile
	cfgFile = ""
	defer func() { cfgFile = originalCfgFile }()

	path := getConfigPath()
	assert.Contains(t, path, ".rune")
	assert.Contains(t, path, "config.yaml")

	// Test custom config path
	cfgFile = "/custom/path/config.yaml"
	path = getConfigPath()
	assert.Equal(t, "/custom/path/config.yaml", path)
}
