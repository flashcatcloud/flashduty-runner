// Package config handles configuration loading and validation.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/viper"
)

// Config represents the runner configuration.
type Config struct {
	// APIKey is the Flashduty API Key for authentication (required)
	APIKey string `mapstructure:"api_key"`

	// APIURL is the Flashduty WebSocket endpoint
	APIURL string `mapstructure:"API_url"`

	// WorknodeID is the unique worknode identifier (assigned by server)
	WorknodeID string `mapstructure:"worknode_id"`

	// Name is the runner display name
	Name string `mapstructure:"name"`

	// Labels are custom labels for task routing
	Labels []string `mapstructure:"labels"`

	// WorkspaceRoot is the root directory for workspace operations
	WorkspaceRoot string `mapstructure:"workspace_root"`

	// AutoUpdate enables automatic binary updates
	AutoUpdate bool `mapstructure:"auto_update"`

	// Permission defines command permission rules
	Permission PermissionConfig `mapstructure:"permission"`

	// Log defines logging configuration
	Log LogConfig `mapstructure:"log"`
}

// PermissionConfig defines permission rules.
type PermissionConfig struct {
	// Bash defines glob patterns for bash command permission
	// Key: glob pattern, Value: "allow" or "deny"
	Bash map[string]string `mapstructure:"bash"`
}

// LogConfig defines logging configuration.
type LogConfig struct {
	Level  string `mapstructure:"level"`  // debug, info, warn, error
	Format string `mapstructure:"format"` // json, text
}

// DefaultConfig returns a configuration with default values.
func DefaultConfig() *Config {
	hostname, _ := os.Hostname()
	homeDir, _ := os.UserHomeDir()

	return &Config{
		APIURL:        "wss://api.flashcat.cloud/runner/ws",
		Name:          hostname,
		Labels:        []string{},
		WorkspaceRoot: filepath.Join(homeDir, ".flashduty-runner", "workspace"),
		AutoUpdate:    true,
		Permission: PermissionConfig{
			Bash: map[string]string{
				"*": "deny", // Deny all by default
			},
		},
		Log: LogConfig{
			Level:  "info",
			Format: "text",
		},
	}
}

// Load loads configuration from file and environment variables.
func Load(configPath string) (*Config, error) {
	cfg := DefaultConfig()

	v := viper.New()
	v.SetConfigType("yaml")

	// Set default config path if not specified
	if configPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		configPath = filepath.Join(homeDir, ".flashduty-runner", "config.yaml")
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); err == nil {
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Environment variable overrides
	v.SetEnvPrefix("FLASHDUTY_RUNNER")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Unmarshal into config struct
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// Add built-in labels
	cfg.Labels = append(cfg.Labels, cfg.BuiltinLabels()...)

	return cfg, nil
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.APIKey == "" {
		return fmt.Errorf("api_key is required")
	}

	if !strings.HasPrefix(c.APIKey, "fk_") {
		return fmt.Errorf("api_key must start with 'fk_'")
	}

	if c.APIURL == "" {
		return fmt.Errorf("API_url is required")
	}

	if !isValidWebSocketURL(c.APIURL) {
		return fmt.Errorf("API_url must start with ws:// or wss://")
	}

	return nil
}

func isValidWebSocketURL(url string) bool {
	return strings.HasPrefix(url, "ws://") || strings.HasPrefix(url, "wss://")
}

// BuiltinLabels returns automatically detected labels.
func (c *Config) BuiltinLabels() []string {
	hostname, _ := os.Hostname()
	return []string{
		"os:" + runtime.GOOS,
		"arch:" + runtime.GOARCH,
		"hostname:" + hostname,
	}
}

// ConfigDir returns the configuration directory path.
func ConfigDir() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".flashduty-runner")
}

// EnsureConfigDir creates the configuration directory if it doesn't exist.
func EnsureConfigDir() error {
	dir := ConfigDir()
	return os.MkdirAll(dir, 0o700)
}
