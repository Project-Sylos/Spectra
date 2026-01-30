package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"codeberg.org/Sylos/Spectra/internal/types"
)

// DefaultConfig returns a default configuration with new structure
func DefaultConfig() types.Config {
	return types.Config{
		Mode: "persistent",
		Seed: types.SeedConfig{
			MaxDepth:               4,
			MaxFolders:             100,
			FolderBackoffFactor:    0.5,
			FolderDepthDecayFactor: 0.8,
			MaxFiles:               100,
			FileBackoffFactor:      0.5,
			FileDepthDecayFactor:   0.85,
			Seed:                   42,
			DBPath:                 "./spectra.db",
			FileBinarySeed:         0,
			EnableCache:            false,
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

	// Set default mode if not specified
	if cfg.Mode == "" {
		cfg.Mode = "persistent"
	}

	// Set default DB path if not specified (only for persistent mode)
	if cfg.Mode == "persistent" {
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

	// Validate mode
	if cfg.Mode == "" {
		cfg.Mode = "persistent" // Default to persistent
	}
	if cfg.Mode != "persistent" && cfg.Mode != "ephemeral" {
		return fmt.Errorf("mode must be either 'persistent' or 'ephemeral', got %s", cfg.Mode)
	}

	// Validate seed config
	if cfg.Seed.MaxDepth < 1 {
		return fmt.Errorf("max_depth must be at least 1, got %d", cfg.Seed.MaxDepth)
	}

	// Validate folder fanout config
	if cfg.Seed.MaxFolders < 0 {
		return fmt.Errorf("max_folders must be non-negative, got %d", cfg.Seed.MaxFolders)
	}

	if cfg.Seed.FolderBackoffFactor <= 0.0 || cfg.Seed.FolderBackoffFactor > 1.0 {
		return fmt.Errorf("folder_backoff_factor must be in (0.0, 1.0], got %f", cfg.Seed.FolderBackoffFactor)
	}

	if cfg.Seed.FolderDepthDecayFactor <= 0.0 || cfg.Seed.FolderDepthDecayFactor > 1.0 {
		return fmt.Errorf("folder_depth_decay_factor must be in (0.0, 1.0], got %f", cfg.Seed.FolderDepthDecayFactor)
	}

	// Validate file fanout config
	if cfg.Seed.MaxFiles < 0 {
		return fmt.Errorf("max_files must be non-negative, got %d", cfg.Seed.MaxFiles)
	}

	if cfg.Seed.FileBackoffFactor <= 0.0 || cfg.Seed.FileBackoffFactor > 1.0 {
		return fmt.Errorf("file_backoff_factor must be in (0.0, 1.0], got %f", cfg.Seed.FileBackoffFactor)
	}

	if cfg.Seed.FileDepthDecayFactor <= 0.0 || cfg.Seed.FileDepthDecayFactor > 1.0 {
		return fmt.Errorf("file_depth_decay_factor must be in (0.0, 1.0], got %f", cfg.Seed.FileDepthDecayFactor)
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
