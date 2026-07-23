package features

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type AppConfig struct {
	ApplicationName    string            `json:"applicationName"`
	Version           string            `json:"version"`
	Environment      string            `json:"environment"`
	Debug             bool              `json:"debug"`
	ServerPort        int               `json:"serverPort"`
	MaxConnections    int               `json:"maxConnections"`
	ConnectionTimeout int               `json:"connectionTimeout"`
	LogLevel          string            `json:"logLevel"`
	LogFile           string            `json:"logFile"`
	DataDirectory     string            `json:"dataDirectory"`
	TempDirectory     string            `json:"tempDirectory"`
	CustomSettings    map[string]string `json:"customSettings"`
}

func DefaultConfig() *AppConfig {
	return &AppConfig{
		ApplicationName:    "PROBE Client",
		Version:           "1.0.0",
		Environment:      "production",
		Debug:             false,
		ServerPort:        7700,
		MaxConnections:    50,
		ConnectionTimeout: 30,
		LogLevel:          "info",
		LogFile:           "logs/hermes.log",
		DataDirectory:     "data",
		TempDirectory:     os.TempDir(),
		CustomSettings:    make(map[string]string),
	}
}

func LoadConfig(path string) (*AppConfig, error) {
	cfg := DefaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) { return cfg, nil }
		return nil, fmt.Errorf("read config file: %w", err)
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}
	applyEnvOverrides(cfg)
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}
	return cfg, nil
}

func (c *AppConfig) Save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil { return fmt.Errorf("marshal config: %w", err) }
	return os.WriteFile(path, data, 0644)
}

func (c *AppConfig) Validate() error {
	if c.ServerPort < 1 || c.ServerPort > 65535 {
		return fmt.Errorf("invalid server port: %d", c.ServerPort)
	}
	if c.MaxConnections < 1 {
		return fmt.Errorf("max connections must be >= 1")
	}
	if c.ConnectionTimeout < 1 {
		return fmt.Errorf("connection timeout must be >= 1")
	}
	validLevels := []string{"debug", "info", "warn", "error"}
	if !contains(validLevels, strings.ToLower(c.LogLevel)) {
		return fmt.Errorf("invalid log level: %s", c.LogLevel)
	}
	return nil
}

func applyEnvOverrides(cfg *AppConfig) {
	if v := os.Getenv("HERMES_DEBUG"); v != "" {
		cfg.Debug = strings.ToLower(v) == "true" || v == "1"
	}
	if v := os.Getenv("HERMES_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil { cfg.ServerPort = port }
	}
	if v := os.Getenv("HERMES_LOG_LEVEL"); v != "" { cfg.LogLevel = v }
	if v := os.Getenv("HERMES_DATA_DIR"); v != "" { cfg.DataDirectory = v }
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item { return true }
	}
	return false
}
