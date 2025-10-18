package config

import (
	"os"
	"testing"

	"github.com/Project-Sylos/Spectra/internal/types"
)

// TestLoadConfig tests the LoadConfig function
func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name        string
		configPath  string
		expectError bool
		setup       func() string // Returns temp config path
		cleanup     func(string)
		validate    func(*testing.T, *types.Config)
	}{
		{
			name:        "valid default config",
			configPath:  "",
			expectError: false,
			setup: func() string {
				// Create a copy of the default config in a temp file
				tmpFile, err := os.CreateTemp("", "default-config-*.json")
				if err != nil {
					t.Fatal(err)
				}
				tmpFile.WriteString(`{
					"seed": {
						"max_depth": 4,
						"min_folders": 1,
						"max_folders": 3,
						"min_files": 2,
						"max_files": 5,
						"seed": 42,
						"db_path": "./spectra.db"
					},
					"api": {
						"host": "localhost",
						"port": 8086
					},
					"secondary_tables": {
						"s1": 0.7,
						"s2": 0.3
					}
				}`)
				tmpFile.Close()
				return tmpFile.Name()
			},
			cleanup: func(path string) {
				os.Remove(path)
			},
			validate: func(t *testing.T, cfg *types.Config) {
				if cfg == nil {
					t.Errorf("Expected config but got nil")
				}
				if cfg.Seed.MaxDepth < 1 {
					t.Errorf("Expected MaxDepth >= 1, got %d", cfg.Seed.MaxDepth)
				}
				if cfg.Seed.MinFolders < 0 {
					t.Errorf("Expected MinFolders >= 0, got %d", cfg.Seed.MinFolders)
				}
				if cfg.Seed.MaxFolders < cfg.Seed.MinFolders {
					t.Errorf("Expected MaxFolders >= MinFolders, got %d < %d", cfg.Seed.MaxFolders, cfg.Seed.MinFolders)
				}
			},
		},
		{
			name:        "nonexistent config file",
			configPath:  "nonexistent.json",
			expectError: true,
			setup:       nil,
			cleanup:     nil,
			validate:    nil,
		},
		{
			name:        "invalid JSON config",
			configPath:  "",
			expectError: true,
			setup: func() string {
				tmpFile, err := os.CreateTemp("", "invalid-config-*.json")
				if err != nil {
					t.Fatal(err)
				}
				tmpFile.WriteString(`{"invalid": json}`)
				tmpFile.Close()
				return tmpFile.Name()
			},
			cleanup: func(path string) {
				os.Remove(path)
			},
			validate: nil,
		},
		{
			name:        "minimal valid config",
			configPath:  "",
			expectError: false,
			setup: func() string {
				tmpFile, err := os.CreateTemp("", "minimal-config-*.json")
				if err != nil {
					t.Fatal(err)
				}
				tmpFile.WriteString(`{
					"seed": {
						"max_depth": 2,
						"min_folders": 1,
						"max_folders": 2,
						"min_files": 1,
						"max_files": 2,
						"seed": 123,
						"db_path": ":memory:"
					},
					"api": {
						"host": "localhost",
						"port": 8080
					},
					"secondary_tables": {
						"s1": 0.5
					}
				}`)
				tmpFile.Close()
				return tmpFile.Name()
			},
			cleanup: func(path string) {
				os.Remove(path)
			},
			validate: func(t *testing.T, cfg *types.Config) {
				if cfg.Seed.MaxDepth != 2 {
					t.Errorf("Expected MaxDepth 2, got %d", cfg.Seed.MaxDepth)
				}
				if cfg.Seed.MinFolders != 1 {
					t.Errorf("Expected MinFolders 1, got %d", cfg.Seed.MinFolders)
				}
				if cfg.Seed.MaxFolders != 2 {
					t.Errorf("Expected MaxFolders 2, got %d", cfg.Seed.MaxFolders)
				}
				if cfg.Seed.MinFiles != 1 {
					t.Errorf("Expected MinFiles 1, got %d", cfg.Seed.MinFiles)
				}
				if cfg.Seed.MaxFiles != 2 {
					t.Errorf("Expected MaxFiles 2, got %d", cfg.Seed.MaxFiles)
				}
				if cfg.Seed.Seed != 123 {
					t.Errorf("Expected Seed 123, got %d", cfg.Seed.Seed)
				}
				// DBPath gets resolved to absolute path, so just check it's not empty
				if cfg.Seed.DBPath == "" {
					t.Errorf("Expected non-empty DBPath, got empty string")
				}
				if cfg.API.Host != "localhost" {
					t.Errorf("Expected API Host 'localhost', got %s", cfg.API.Host)
				}
				if cfg.API.Port != 8080 {
					t.Errorf("Expected API Port 8080, got %d", cfg.API.Port)
				}
				if len(cfg.SecondaryTables) != 1 {
					t.Errorf("Expected 1 secondary table, got %d", len(cfg.SecondaryTables))
				}
				if cfg.SecondaryTables["s1"] != 0.5 {
					t.Errorf("Expected s1 probability 0.5, got %f", cfg.SecondaryTables["s1"])
				}
			},
		},
		{
			name:        "config with multiple secondary tables",
			configPath:  "",
			expectError: false,
			setup: func() string {
				tmpFile, err := os.CreateTemp("", "multi-secondary-config-*.json")
				if err != nil {
					t.Fatal(err)
				}
				tmpFile.WriteString(`{
					"seed": {
						"max_depth": 3,
						"min_folders": 1,
						"max_folders": 3,
						"min_files": 1,
						"max_files": 3,
						"seed": 456,
						"db_path": "./test.db"
					},
					"api": {
						"host": "0.0.0.0",
						"port": 8086
					},
					"secondary_tables": {
						"s1": 0.7,
						"s2": 0.3,
						"s3": 0.1
					}
				}`)
				tmpFile.Close()
				return tmpFile.Name()
			},
			cleanup: func(path string) {
				os.Remove(path)
			},
			validate: func(t *testing.T, cfg *types.Config) {
				if len(cfg.SecondaryTables) != 3 {
					t.Errorf("Expected 3 secondary tables, got %d", len(cfg.SecondaryTables))
				}
				if cfg.SecondaryTables["s1"] != 0.7 {
					t.Errorf("Expected s1 probability 0.7, got %f", cfg.SecondaryTables["s1"])
				}
				if cfg.SecondaryTables["s2"] != 0.3 {
					t.Errorf("Expected s2 probability 0.3, got %f", cfg.SecondaryTables["s2"])
				}
				if cfg.SecondaryTables["s3"] != 0.1 {
					t.Errorf("Expected s3 probability 0.1, got %f", cfg.SecondaryTables["s3"])
				}
				if cfg.API.Host != "0.0.0.0" {
					t.Errorf("Expected API Host '0.0.0.0', got %s", cfg.API.Host)
				}
				if cfg.API.Port != 8086 {
					t.Errorf("Expected API Port 8086, got %d", cfg.API.Port)
				}
			},
		},
		{
			name:        "config with empty secondary tables",
			configPath:  "",
			expectError: false,
			setup: func() string {
				tmpFile, err := os.CreateTemp("", "empty-secondary-config-*.json")
				if err != nil {
					t.Fatal(err)
				}
				tmpFile.WriteString(`{
					"seed": {
						"max_depth": 2,
						"min_folders": 1,
						"max_folders": 2,
						"min_files": 1,
						"max_files": 2,
						"seed": 789,
						"db_path": "./empty.db"
					},
					"api": {
						"host": "localhost",
						"port": 8080
					},
					"secondary_tables": {}
				}`)
				tmpFile.Close()
				return tmpFile.Name()
			},
			cleanup: func(path string) {
				os.Remove(path)
			},
			validate: func(t *testing.T, cfg *types.Config) {
				if len(cfg.SecondaryTables) != 0 {
					t.Errorf("Expected 0 secondary tables, got %d", len(cfg.SecondaryTables))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var configPath string
			if tt.setup != nil {
				configPath = tt.setup()
				defer tt.cleanup(configPath)
			} else {
				configPath = tt.configPath
			}

			cfg, err := LoadFromFile(configPath)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				if cfg != nil {
					t.Errorf("Expected nil config but got %v", cfg)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if cfg == nil {
					t.Errorf("Expected config but got nil")
				}
				if tt.validate != nil {
					tt.validate(t, cfg)
				}
			}
		})
	}
}

// TestLoadConfigWithDefaults tests the LoadConfigWithDefaults function
func TestLoadConfigWithDefaults(t *testing.T) {
	// Create a temporary config file with default values
	tmpFile, err := os.CreateTemp("", "default-config-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write default config
	tmpFile.WriteString(`{
		"seed": {
			"max_depth": 4,
			"min_folders": 1,
			"max_folders": 3,
			"min_files": 2,
			"max_files": 5,
			"seed": 42,
			"db_path": "./spectra.db"
		},
		"api": {
			"host": "localhost",
			"port": 8086
		},
		"secondary_tables": {
			"s1": 0.7,
			"s2": 0.3
		}
	}`)
	tmpFile.Close()

	cfg, err := LoadFromFile(tmpFile.Name())
	if err != nil {
		t.Errorf("Unexpected error loading config with defaults: %v", err)
	}
	if cfg == nil {
		t.Errorf("Expected config but got nil")
	}

	// Verify defaults are reasonable
	if cfg.Seed.MaxDepth < 1 {
		t.Errorf("Expected MaxDepth >= 1, got %d", cfg.Seed.MaxDepth)
	}
	if cfg.Seed.MinFolders < 0 {
		t.Errorf("Expected MinFolders >= 0, got %d", cfg.Seed.MinFolders)
	}
	if cfg.Seed.MaxFolders < cfg.Seed.MinFolders {
		t.Errorf("Expected MaxFolders >= MinFolders, got %d < %d", cfg.Seed.MaxFolders, cfg.Seed.MinFolders)
	}
	if cfg.Seed.MinFiles < 0 {
		t.Errorf("Expected MinFiles >= 0, got %d", cfg.Seed.MinFiles)
	}
	if cfg.Seed.MaxFiles < cfg.Seed.MinFiles {
		t.Errorf("Expected MaxFiles >= MinFiles, got %d < %d", cfg.Seed.MaxFiles, cfg.Seed.MinFiles)
	}
	if cfg.API.Host == "" {
		t.Errorf("Expected non-empty API Host")
	}
	if cfg.API.Port <= 0 {
		t.Errorf("Expected API Port > 0, got %d", cfg.API.Port)
	}
}

// TestValidateConfig tests the ValidateConfig function
func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *types.Config
		expectError bool
	}{
		{
			name: "valid config",
			config: &types.Config{
				Seed: types.SeedConfig{
					MaxDepth:   3,
					MinFolders: 1,
					MaxFolders: 3,
					MinFiles:   1,
					MaxFiles:   3,
					Seed:       42,
					DBPath:     "./test.db",
				},
				API: types.APIConfig{
					Host: "localhost",
					Port: 8080,
				},
				SecondaryTables: map[string]float64{
					"s1": 0.7,
					"s2": 0.3,
				},
			},
			expectError: false,
		},
		{
			name: "invalid max depth",
			config: &types.Config{
				Seed: types.SeedConfig{
					MaxDepth:   0, // Invalid
					MinFolders: 1,
					MaxFolders: 3,
					MinFiles:   1,
					MaxFiles:   3,
					Seed:       42,
					DBPath:     "./test.db",
				},
				API: types.APIConfig{
					Host: "localhost",
					Port: 8080,
				},
				SecondaryTables: map[string]float64{},
			},
			expectError: true,
		},
		{
			name: "invalid folder counts",
			config: &types.Config{
				Seed: types.SeedConfig{
					MaxDepth:   3,
					MinFolders: 5, // Invalid: > MaxFolders
					MaxFolders: 3,
					MinFiles:   1,
					MaxFiles:   3,
					Seed:       42,
					DBPath:     "./test.db",
				},
				API: types.APIConfig{
					Host: "localhost",
					Port: 8080,
				},
				SecondaryTables: map[string]float64{},
			},
			expectError: true,
		},
		{
			name: "invalid file counts",
			config: &types.Config{
				Seed: types.SeedConfig{
					MaxDepth:   3,
					MinFolders: 1,
					MaxFolders: 3,
					MinFiles:   5, // Invalid: > MaxFiles
					MaxFiles:   3,
					Seed:       42,
					DBPath:     "./test.db",
				},
				API: types.APIConfig{
					Host: "localhost",
					Port: 8080,
				},
				SecondaryTables: map[string]float64{},
			},
			expectError: true,
		},
		{
			name: "invalid API port",
			config: &types.Config{
				Seed: types.SeedConfig{
					MaxDepth:   3,
					MinFolders: 1,
					MaxFolders: 3,
					MinFiles:   1,
					MaxFiles:   3,
					Seed:       42,
					DBPath:     "./test.db",
				},
				API: types.APIConfig{
					Host: "localhost",
					Port: 0, // Invalid
				},
				SecondaryTables: map[string]float64{},
			},
			expectError: true,
		},
		{
			name: "invalid secondary table probabilities",
			config: &types.Config{
				Seed: types.SeedConfig{
					MaxDepth:   3,
					MinFolders: 1,
					MaxFolders: 3,
					MinFiles:   1,
					MaxFiles:   3,
					Seed:       42,
					DBPath:     "./test.db",
				},
				API: types.APIConfig{
					Host: "localhost",
					Port: 8080,
				},
				SecondaryTables: map[string]float64{
					"s1": -0.1, // Invalid: negative
					"s2": 1.5,  // Invalid: > 1.0
				},
			},
			expectError: true,
		},
		{
			name:        "nil config",
			config:      nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.config)
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// TestGetDefaultConfig tests the GetDefaultConfig function
func TestGetDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	// Verify default values are reasonable
	if config.Seed.MaxDepth < 1 {
		t.Errorf("Expected MaxDepth >= 1, got %d", config.Seed.MaxDepth)
	}
	if config.Seed.MinFolders < 0 {
		t.Errorf("Expected MinFolders >= 0, got %d", config.Seed.MinFolders)
	}
	if config.Seed.MaxFolders < config.Seed.MinFolders {
		t.Errorf("Expected MaxFolders >= MinFolders, got %d < %d", config.Seed.MaxFolders, config.Seed.MinFolders)
	}
	if config.Seed.MinFiles < 0 {
		t.Errorf("Expected MinFiles >= 0, got %d", config.Seed.MinFiles)
	}
	if config.Seed.MaxFiles < config.Seed.MinFiles {
		t.Errorf("Expected MaxFiles >= MinFiles, got %d < %d", config.Seed.MaxFiles, config.Seed.MinFiles)
	}
	if config.API.Host == "" {
		t.Errorf("Expected non-empty API Host")
	}
	if config.API.Port <= 0 {
		t.Errorf("Expected API Port > 0, got %d", config.API.Port)
	}

	// Verify validation passes
	err := Validate(&config)
	if err != nil {
		t.Errorf("Default config should be valid: %v", err)
	}
}

// TestConfigEdgeCases tests edge cases for config handling
func TestConfigEdgeCases(t *testing.T) {
	// Test with very large values
	config := &types.Config{
		Seed: types.SeedConfig{
			MaxDepth:   100,
			MinFolders: 0,
			MaxFolders: 100,
			MinFiles:   0,
			MaxFiles:   100,
			Seed:       999999,
			DBPath:     "/very/long/path/to/database.db",
		},
		API: types.APIConfig{
			Host: "very-long-hostname.example.com",
			Port: 65535,
		},
		SecondaryTables: map[string]float64{
			"s1": 0.0,
			"s2": 1.0,
		},
	}

	err := Validate(config)
	if err != nil {
		t.Errorf("Config with large values should be valid: %v", err)
	}

	// Test with minimal values
	config = &types.Config{
		Seed: types.SeedConfig{
			MaxDepth:   1,
			MinFolders: 0,
			MaxFolders: 0,
			MinFiles:   0,
			MaxFiles:   0,
			Seed:       0,
			DBPath:     ":memory:",
		},
		API: types.APIConfig{
			Host: "127.0.0.1",
			Port: 1,
		},
		SecondaryTables: map[string]float64{},
	}

	err = Validate(config)
	if err != nil {
		t.Errorf("Config with minimal values should be valid: %v", err)
	}
}

// TestConfigFileOperations tests file operations for config
func TestConfigFileOperations(t *testing.T) {
	// Test creating and reading a config file
	originalConfig := &types.Config{
		Seed: types.SeedConfig{
			MaxDepth:   3,
			MinFolders: 1,
			MaxFolders: 3,
			MinFiles:   1,
			MaxFiles:   3,
			Seed:       42,
			DBPath:     "./test.db",
		},
		API: types.APIConfig{
			Host: "localhost",
			Port: 8080,
		},
		SecondaryTables: map[string]float64{
			"s1": 0.7,
			"s2": 0.3,
		},
	}

	// Create temporary config file
	tmpFile, err := os.CreateTemp("", "test-config-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write config to file
	err = SaveToFile(originalConfig, tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Read config back
	loadedConfig, err := LoadFromFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify loaded config matches original
	if loadedConfig.Seed.MaxDepth != originalConfig.Seed.MaxDepth {
		t.Errorf("MaxDepth mismatch: expected %d, got %d", originalConfig.Seed.MaxDepth, loadedConfig.Seed.MaxDepth)
	}
	if loadedConfig.Seed.MinFolders != originalConfig.Seed.MinFolders {
		t.Errorf("MinFolders mismatch: expected %d, got %d", originalConfig.Seed.MinFolders, loadedConfig.Seed.MinFolders)
	}
	if loadedConfig.Seed.MaxFolders != originalConfig.Seed.MaxFolders {
		t.Errorf("MaxFolders mismatch: expected %d, got %d", originalConfig.Seed.MaxFolders, loadedConfig.Seed.MaxFolders)
	}
	if loadedConfig.Seed.MinFiles != originalConfig.Seed.MinFiles {
		t.Errorf("MinFiles mismatch: expected %d, got %d", originalConfig.Seed.MinFiles, loadedConfig.Seed.MinFiles)
	}
	if loadedConfig.Seed.MaxFiles != originalConfig.Seed.MaxFiles {
		t.Errorf("MaxFiles mismatch: expected %d, got %d", originalConfig.Seed.MaxFiles, loadedConfig.Seed.MaxFiles)
	}
	if loadedConfig.Seed.Seed != originalConfig.Seed.Seed {
		t.Errorf("Seed mismatch: expected %d, got %d", originalConfig.Seed.Seed, loadedConfig.Seed.Seed)
	}
	// DBPath gets resolved to absolute path, so just check it's not empty
	if loadedConfig.Seed.DBPath == "" {
		t.Errorf("Expected non-empty DBPath after loading, got empty string")
	}
	if loadedConfig.API.Host != originalConfig.API.Host {
		t.Errorf("API Host mismatch: expected %s, got %s", originalConfig.API.Host, loadedConfig.API.Host)
	}
	if loadedConfig.API.Port != originalConfig.API.Port {
		t.Errorf("API Port mismatch: expected %d, got %d", originalConfig.API.Port, loadedConfig.API.Port)
	}
	if len(loadedConfig.SecondaryTables) != len(originalConfig.SecondaryTables) {
		t.Errorf("SecondaryTables length mismatch: expected %d, got %d", len(originalConfig.SecondaryTables), len(loadedConfig.SecondaryTables))
	}
	for key, value := range originalConfig.SecondaryTables {
		if loadedConfig.SecondaryTables[key] != value {
			t.Errorf("SecondaryTables[%s] mismatch: expected %f, got %f", key, value, loadedConfig.SecondaryTables[key])
		}
	}
}

// TestConfigConcurrency tests thread safety of config functions
func TestConfigConcurrency(t *testing.T) {
	done := make(chan bool, 10)

	// Test multiple goroutines calling config functions
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()

			// Test various config functions
			config := DefaultConfig()
			Validate(&config)
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

// Benchmark tests for config functions
func BenchmarkLoadConfig(b *testing.B) {
	// Create a temporary config file for benchmarking
	tmpFile, err := os.CreateTemp("", "bench-config-*.json")
	if err != nil {
		b.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString(`{
		"seed": {
			"max_depth": 3,
			"min_folders": 1,
			"max_folders": 3,
			"min_files": 1,
			"max_files": 3,
			"seed": 42,
			"db_path": ":memory:"
		},
		"api": {
			"host": "localhost",
			"port": 8080
		},
		"secondary_tables": {
			"s1": 0.7,
			"s2": 0.3
		}
	}`)
	tmpFile.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := LoadFromFile(tmpFile.Name())
		if err != nil {
			b.Fatalf("Failed to load config: %v", err)
		}
	}
}

func BenchmarkValidateConfig(b *testing.B) {
	config := DefaultConfig()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Validate(&config)
	}
}
