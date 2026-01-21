package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, "wss://api.flashcat.cloud/runner/ws", cfg.APIURL)
	assert.True(t, cfg.AutoUpdate)
	assert.Equal(t, "deny", cfg.Permission.Bash["*"])
	assert.Equal(t, "info", cfg.Log.Level)
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr string
	}{
		{
			name: "valid config",
			config: &Config{
				APIKey: "test_key_12345",
				APIURL: "wss://api.flashcat.cloud/runner/ws",
			},
			wantErr: "",
		},
		{
			name: "missing api_key",
			config: &Config{
				APIURL: "wss://api.flashcat.cloud/runner/ws",
			},
			wantErr: "api_key is required",
		},
		{
			name: "missing API_url",
			config: &Config{
				APIKey: "test_key",
			},
			wantErr: "API_url is required",
		},
		{
			name: "invalid API_url scheme",
			config: &Config{
				APIKey: "test_key",
				APIURL: "http://invalid.url",
			},
			wantErr: "API_url must start with ws:// or wss://",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestBuiltinLabels(t *testing.T) {
	cfg := &Config{}
	labels := cfg.BuiltinLabels()

	assert.Len(t, labels, 3)

	// Check that labels contain expected prefixes
	var hasOS, hasArch, hasHostname bool
	for _, label := range labels {
		if len(label) > 3 && label[:3] == "os:" {
			hasOS = true
		}
		if len(label) > 5 && label[:5] == "arch:" {
			hasArch = true
		}
		if len(label) > 9 && label[:9] == "hostname:" {
			hasHostname = true
		}
	}

	assert.True(t, hasOS, "should have os label")
	assert.True(t, hasArch, "should have arch label")
	assert.True(t, hasHostname, "should have hostname label")
}

func TestLoadFromFile(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
api_key: "test_key_12345"
API_url: "wss://test.api.cloud/runner/ws"
name: "test-runner"
labels:
  - k8s
  - production
workspace_root: "/tmp/workspace"
auto_update: false
permission:
  bash:
    "*": "deny"
    "git *": "allow"
log:
  level: "debug"
  format: "json"
`
	err := os.WriteFile(configPath, []byte(configContent), 0o600)
	require.NoError(t, err)

	cfg, err := Load(configPath)
	require.NoError(t, err)

	assert.Equal(t, "test_key_12345", cfg.APIKey)
	assert.Equal(t, "wss://test.api.cloud/runner/ws", cfg.APIURL)
	assert.Equal(t, "test-runner", cfg.Name)
	assert.Contains(t, cfg.Labels, "k8s")
	assert.Contains(t, cfg.Labels, "production")
	assert.Equal(t, "/tmp/workspace", cfg.WorkspaceRoot)
	assert.False(t, cfg.AutoUpdate)
	assert.Equal(t, "deny", cfg.Permission.Bash["*"])
	assert.Equal(t, "allow", cfg.Permission.Bash["git *"])
	assert.Equal(t, "debug", cfg.Log.Level)
	assert.Equal(t, "json", cfg.Log.Format)
}

func TestLoadWithEnvOverride(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
api_key: "file_key_123"
API_url: "wss://api.flashcat.cloud/runner/ws"
`
	err := os.WriteFile(configPath, []byte(configContent), 0o600)
	require.NoError(t, err)

	// Set environment variable
	t.Setenv("FLASHDUTY_RUNNER_API_KEY", "env_key_456")

	cfg, err := Load(configPath)
	require.NoError(t, err)

	// Environment variable should override file value
	assert.Equal(t, "env_key_456", cfg.APIKey)
}
