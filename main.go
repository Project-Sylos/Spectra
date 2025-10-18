package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/Project-Sylos/Spectra/sdk"
)

func main() {
	var (
		config = flag.String("config", "configs/default.json", "Configuration file path")
		help   = flag.Bool("help", false, "Show help")
	)
	flag.Parse()

	if *help {
		showHelp()
		return
	}

	fmt.Println("Spectra - SDK Demo")
	fmt.Println("==================")
	fmt.Println("This is a demonstration of the Spectra SDK functionality.")
	fmt.Println("For the API server, run: go run cmd/api/main.go")
	fmt.Println()

	runDemo(*config)
}

func showHelp() {
	fmt.Println("Spectra - Synthetic Filesystem Simulator")
	fmt.Println("========================================")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  go run main.go [options]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -config string")
	fmt.Println("        Configuration file path (default: configs/default.json)")
	fmt.Println("  -help")
	fmt.Println("        Show this help message")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  go run main.go")
	fmt.Println("  go run main.go -config configs/custom.json")
	fmt.Println()
	fmt.Println("API Server:")
	fmt.Println("  go run cmd/api/main.go [config-file]")
	fmt.Println("  go run cmd/api/main.go configs/custom.json")
}

func runDemo(configPath string) {
	fmt.Printf("Loading configuration from: %s\n", configPath)

	// Initialize SpectraFS with configuration
	fs, err := sdk.New(configPath)
	if err != nil {
		log.Fatalf("Failed to initialize SpectraFS: %v", err)
	}

	// Get configuration
	cfg := fs.GetConfig()
	fmt.Printf("Configuration loaded: MaxDepth=%d, Seed=%d\n", cfg.Seed.MaxDepth, cfg.Seed.Seed)

	// Get table information
	fmt.Println("\nTable Information:")
	tableInfo, err := fs.GetTableInfo()
	if err != nil {
		log.Printf("Failed to get table info: %v", err)
	} else {
		for _, table := range tableInfo {
			fmt.Printf("  %s: %d rows\n", table.Name, table.RowCount)
		}
	}

	// List root children (this will trigger generation)
	fmt.Println("\nListing root children (triggering generation)...")
	result, err := fs.ListChildren("p-root")
	if err != nil {
		log.Printf("Failed to list root children: %v", err)
	} else {
		fmt.Printf("Success: %t, Message: %s\n", result.Success, result.Message)
		fmt.Printf("Generated %d folders and %d files\n", len(result.Folders), len(result.Files))

		// Show some details about generated nodes
		if len(result.Folders) > 0 {
			fmt.Printf("First folder: %s (ID: %s)\n", result.Folders[0].Name, result.Folders[0].ID)
		}
		if len(result.Files) > 0 {
			fmt.Printf("First file: %s (ID: %s, Size: %d bytes)\n", result.Files[0].Name, result.Files[0].ID, result.Files[0].Size)
		}
	}

	// Get updated table information
	fmt.Println("\nUpdated Table Information:")
	tableInfo, err = fs.GetTableInfo()
	if err != nil {
		log.Printf("Failed to get updated table info: %v", err)
	} else {
		for _, table := range tableInfo {
			fmt.Printf("  %s: %d rows\n", table.Name, table.RowCount)
		}
	}

	// Show secondary tables
	fmt.Println("\nSecondary Tables:")
	secondaryTables := fs.GetSecondaryTables()
	for _, table := range secondaryTables {
		count, err := fs.GetNodeCount(table)
		if err != nil {
			log.Printf("Failed to get count for %s: %v", table, err)
		} else {
			fmt.Printf("  %s: %d nodes\n", table, count)
		}
	}

	// Reset filesystem
	fmt.Println("\nResetting filesystem...")
	if err := fs.Reset(); err != nil {
		log.Printf("Failed to reset filesystem: %v", err)
	} else {
		fmt.Println("Filesystem reset completed!")
	}

	fmt.Println("\nSpectra SDK demo completed successfully!")
	fmt.Println("\nTo start the API server, run:")
	fmt.Println("  go run cmd/api/main.go")
}
