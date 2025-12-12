package main

import (
	"flag"
	"fmt"
	"light-llm-client/db"
	"light-llm-client/ui"
	"light-llm-client/utils"
	"os"
)

var (
	version = "0.1.0"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "", "Path to configuration file")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		fmt.Printf("Light LLM Client v%s\n", version)
		os.Exit(0)
	}

	// Initialize logger
	logger, err := utils.NewLogger(utils.GetLogPath())
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Close()

	logger.Info("Starting Light LLM Client v%s", version)

	// Load or create default configuration
	var config *utils.Config
	var actualConfigPath string
	if *configPath != "" {
		actualConfigPath = *configPath
		config, err = utils.LoadConfig(actualConfigPath)
		if err != nil {
			logger.Error("Failed to load config: %v", err)
			os.Exit(1)
		}
	} else {
		// Ensure default config exists
		actualConfigPath, err = utils.EnsureDefaultConfig()
		if err != nil {
			logger.Error("Failed to create default config: %v", err)
			os.Exit(1)
		}
		logger.Info("Using config file: %s", actualConfigPath)

		config, err = utils.LoadConfig(actualConfigPath)
		if err != nil {
			logger.Error("Failed to load config: %v", err)
			os.Exit(1)
		}
	}

	// Initialize database
	database, err := db.New(config.Data.DBPath)
	if err != nil {
		logger.Error("Failed to initialize database: %v", err)
		os.Exit(1)
	}
	defer database.Close()

	logger.Info("Database initialized: %s", config.Data.DBPath)

	// Create and run application
	app := ui.NewApp(config, actualConfigPath, database, logger)
	defer app.Cleanup()

	logger.Info("Application started")
	app.Run()
	logger.Info("Application stopped")
}
