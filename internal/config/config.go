package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Project-Sylos/Spectra/internal/types"
)

// DefaultConfig returns a default configuration with new structure
func DefaultConfig() types.Config {
	return types.Config{
		Seed: types.SeedConfig{
			MaxDepth:   4,
			MinFolders: 1,
			MaxFolders: 3,
			MinFiles:   2,
			MaxFiles:   5,
			Seed:       42,
			DBPath:     "./spectra.db",
		},
		API: types.APIConfig{
			Host: "localhost",
			Port: 8086,
		},
		SecondaryTables: map[string]float64{
			"s1": 0.7,
		},
	}
}

// LoadFromFile loads configuration from a JSON file
func LoadFromFile(configPath string) (*types.Config, error) {
	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", configPath)
	}

	// Read file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse JSON
	var cfg types.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	// Validate configuration
	if err := Validate(&cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	// Set default DB path if not specified
	if cfg.Seed.DBPath == "" {
		cfg.Seed.DBPath = "./spectra.db"
	}

	// Ensure DB path is absolute
	if !filepath.IsAbs(cfg.Seed.DBPath) {
		absPath, err := filepath.Abs(cfg.Seed.DBPath)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve DB path: %w", err)
		}
		cfg.Seed.DBPath = absPath
	}

	// Set default API config if not specified
	if cfg.API.Host == "" {
		cfg.API.Host = "localhost"
	}
	if cfg.API.Port == 0 {
		cfg.API.Port = 8086
	}

	return &cfg, nil
}

// Validate checks that the configuration parameters are valid
func Validate(cfg *types.Config) error {
	if cfg == nil {
		return fmt.Errorf("config cannot be nil")
	}
	
	// Validate seed config
	if cfg.Seed.MaxDepth < 1 {
		return fmt.Errorf("max_depth must be at least 1, got %d", cfg.Seed.MaxDepth)
	}

	if cfg.Seed.MinFolders < 0 {
		return fmt.Errorf("min_folders must be non-negative, got %d", cfg.Seed.MinFolders)
	}

	if cfg.Seed.MaxFolders < cfg.Seed.MinFolders {
		return fmt.Errorf("max_folders (%d) must be >= min_folders (%d)", cfg.Seed.MaxFolders, cfg.Seed.MinFolders)
	}

	if cfg.Seed.MinFiles < 0 {
		return fmt.Errorf("min_files must be non-negative, got %d", cfg.Seed.MinFiles)
	}

	if cfg.Seed.MaxFiles < cfg.Seed.MinFiles {
		return fmt.Errorf("max_files (%d) must be >= min_files (%d)", cfg.Seed.MaxFiles, cfg.Seed.MinFiles)
	}

	// Validate API config
	if cfg.API.Port < 1 || cfg.API.Port > 65535 {
		return fmt.Errorf("API port must be between 1 and 65535, got %d", cfg.API.Port)
	}

	// Validate secondary tables
	for tableName, probability := range cfg.SecondaryTables {
		if probability < 0.0 || probability > 1.0 {
			return fmt.Errorf("secondary table %s probability must be between 0.0 and 1.0, got %f", tableName, probability)
		}
	}

	return nil
}

// SaveToFile saves configuration to a JSON file
func SaveToFile(cfg *types.Config, configPath string) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config to JSON: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
