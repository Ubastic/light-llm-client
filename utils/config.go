package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config represents the application configuration
type Config struct {
	LLMProviders map[string]ProviderConfig `json:"llm_providers"`
	UI           UIConfig                  `json:"ui"`
	Data         DataConfig                `json:"data"`
	Proxy        ProxyConfig               `json:"proxy"`
}

// ProviderConfig represents LLM provider configuration
type ProviderConfig struct {
	DisplayName  string   `json:"display_name,omitempty"` // Display name for UI
	APIKey       string   `json:"api_key"`
	BaseURL      string   `json:"base_url"`
	DefaultModel string   `json:"default_model"`
	Models       []string `json:"models,omitempty"`       // Available models list
	Enabled      bool     `json:"enabled"`
	MaxTokens    int      `json:"max_tokens,omitempty"`
	Temperature  float64  `json:"temperature,omitempty"`
}

// UIConfig represents UI configuration
type UIConfig struct {
	Theme          string `json:"theme"`
	FontSize       int    `json:"font_size"`
	WindowWidth    int    `json:"window_width"`
	WindowHeight   int    `json:"window_height"`
	MinimizeToTray bool   `json:"minimize_to_tray"`
}

// DataConfig represents data storage configuration
type DataConfig struct {
	DBPath     string `json:"db_path"`
	MaxHistory int    `json:"max_history"`
}

// ProxyConfig represents proxy configuration
type ProxyConfig struct {
	Enabled bool   `json:"enabled"`
	URL     string `json:"url"`
}

// LoadConfig loads configuration from file
func LoadConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Expand paths
	if config.Data.DBPath != "" {
		config.Data.DBPath = expandPath(config.Data.DBPath)
	}

	return &config, nil
}

// SaveConfig saves configuration to file
func SaveConfig(configPath string, config *Config) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// expandPath expands ~ and relative paths
func expandPath(path string) string {
	if len(path) == 0 {
		return path
	}

	// Expand ~
	if path[0] == '~' {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[1:])
		}
	}

	// Make absolute
	absPath, err := filepath.Abs(path)
	if err == nil {
		return absPath
	}

	return path
}

// GetConfigPath returns the default config path
func GetConfigPath() string {
	// Try to get user config directory
	configDir, err := os.UserConfigDir()
	if err != nil {
		// Fallback to current directory
		return "./config/default.json"
	}

	return filepath.Join(configDir, "light-llm-client", "config.json")
}

// EnsureDefaultConfig creates a default config file if it doesn't exist
func EnsureDefaultConfig() (string, error) {
	configPath := GetConfigPath()

	// Check if config exists
	if _, err := os.Stat(configPath); err == nil {
		return configPath, nil
	}

	// Create default config
	defaultConfig := &Config{
		LLMProviders: map[string]ProviderConfig{
			"openai": {
				DisplayName:  "OpenAI",
				APIKey:       "",
				BaseURL:      "https://api.openai.com/v1",
				DefaultModel: "gpt-4-turbo-preview",
				Models: []string{
					"gpt-4-turbo-preview",
					"gpt-4",
					"gpt-3.5-turbo",
				},
				Enabled: true,
			},
			"ollama": {
				DisplayName:  "Ollama",
				BaseURL:      "http://localhost:11434",
				DefaultModel: "llama2",
				Models: []string{
					"llama2",
					"mistral",
					"codellama",
				},
				Enabled: false,
			},
			"claude": {
				DisplayName:  "Claude",
				APIKey:       "",
				BaseURL:      "https://api.anthropic.com/v1",
				DefaultModel: "claude-3-5-sonnet-20241022",
				Models: []string{
					"claude-3-5-sonnet-20241022",
					"claude-3-5-haiku-20241022",
					"claude-3-opus-20240229",
				},
				MaxTokens:   4096,
				Temperature: 0.7,
				Enabled:     false,
			},
			"gemini": {
				DisplayName:  "Gemini",
				APIKey:       "",
				BaseURL:      "https://generativelanguage.googleapis.com/v1beta",
				DefaultModel: "gemini-1.5-flash",
				Models: []string{
					"gemini-1.5-flash",
					"gemini-1.5-pro",
					"gemini-1.0-pro",
					"gemini-2.0-flash-exp",
				},
				MaxTokens:   8192,
				Temperature: 0.7,
				Enabled:     false,
			},
		},
		UI: UIConfig{
			Theme:          "light",
			FontSize:       14,
			WindowWidth:    1200,
			WindowHeight:   800,
			MinimizeToTray: true,
		},
		Data: DataConfig{
			DBPath:     "./data/chat.db",
			MaxHistory: 1000,
		},
		Proxy: ProxyConfig{
			Enabled: false,
			URL:     "",
		},
	}

	if err := SaveConfig(configPath, defaultConfig); err != nil {
		return "", err
	}

	return configPath, nil
}
