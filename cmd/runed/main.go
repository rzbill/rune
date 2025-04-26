package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/rzbill/rune/pkg/api/server"
	"github.com/rzbill/rune/pkg/log"
	"github.com/rzbill/rune/pkg/store"
	"github.com/rzbill/rune/pkg/version"
	"github.com/spf13/viper"
)

var (
	configFile = flag.String("config", "", "Configuration file path")
	grpcAddr   = flag.String("grpc-addr", ":8080", "gRPC server address")
	httpAddr   = flag.String("http-addr", ":8081", "HTTP server address")
	dataDir    = flag.String("data-dir", "", "Data directory (if not specified, uses OS-specific application data directory)")
	logLevel   = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	apiKeys    = flag.String("api-keys", "", "Comma-separated list of API keys (empty to disable auth)")
	showHelp   = flag.Bool("help", false, "Show help")
	showVer    = flag.Bool("version", false, "Show version")
)

// getDefaultDataDir returns the default data directory based on the OS
func getDefaultDataDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "./data"
	}

	// OS-specific paths
	switch {
	case os.Getenv("XDG_DATA_HOME") != "":
		// Linux with XDG
		return filepath.Join(os.Getenv("XDG_DATA_HOME"), "rune")
	case isDir("/var/lib"):
		// Linux/Unix system dir
		return "/var/lib/rune"
	case isDir(filepath.Join(homeDir, "Library")):
		// macOS
		return filepath.Join(homeDir, "Library", "Application Support", "Rune")
	case isDir(filepath.Join(homeDir, "AppData")):
		// Windows
		return filepath.Join(homeDir, "AppData", "Local", "Rune")
	default:
		// Fallback
		return filepath.Join(homeDir, ".rune")
	}
}

// isDir checks if a path exists and is a directory
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// loadConfig loads configuration from file and environment variables
func loadConfig() {
	// Initialize viper
	v := viper.New()

	// 1. Set default values that will be used if nothing else is specified
	defaultDataDir := getDefaultDataDir()
	v.SetDefault("server.grpc_address", ":8080")
	v.SetDefault("server.http_address", ":8081")
	v.SetDefault("data_dir", defaultDataDir)
	v.SetDefault("log.level", "info")
	v.SetDefault("auth.api_keys", "")

	// 2. Try to load config file if specified or look in standard locations
	configFileSpecified := *configFile != ""
	if configFileSpecified {
		v.SetConfigFile(*configFile)
	} else {
		v.SetConfigName("rune")
		v.SetConfigType("yaml")
		v.AddConfigPath("/etc/rune/")
		v.AddConfigPath("$HOME/.rune")
		v.AddConfigPath(".")
	}

	// Read config file if available
	if err := v.ReadInConfig(); err != nil {
		if configFileSpecified {
			// Only show an error if user explicitly specified a config file
			fmt.Printf("Error reading config file %s: %s\n", *configFile, err)
		} else if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Show non-"not found" errors even for auto-discovered config
			fmt.Printf("Error reading config file: %s\n", err)
		}
	} else {
		fmt.Printf("Using config file: %s\n", v.ConfigFileUsed())
	}

	// 3. Override with environment variables
	v.SetEnvPrefix("RUNE")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// 4. Track which parameters were explicitly set via command-line flags
	// These will override everything else
	cmdFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		cmdFlags[f.Name] = true
	})

	// 5. Apply values in order of precedence:
	// Command-line flags (already set) > env vars > config file > defaults (already set)

	// Only apply values from config/env if not explicitly set by command-line flags
	if !cmdFlags["grpc-addr"] {
		*grpcAddr = v.GetString("server.grpc_address")
	}

	if !cmdFlags["http-addr"] {
		*httpAddr = v.GetString("server.http_address")
	}

	if !cmdFlags["data-dir"] {
		dataDirFromConfig := v.GetString("data_dir")
		if dataDirFromConfig != "" {
			*dataDir = dataDirFromConfig
		} else {
			*dataDir = defaultDataDir
		}
	}

	if !cmdFlags["log-level"] {
		*logLevel = v.GetString("log.level")
	}

	if !cmdFlags["api-keys"] {
		*apiKeys = v.GetString("auth.api_keys")
	}

	// Final validation and defaults for required parameters
	if *dataDir == "" {
		*dataDir = defaultDataDir
	}
}

func main() {
	// Parse flags
	flag.Parse()

	// Show help if requested
	if *showHelp {
		flag.Usage()
		return
	}

	// Show version if requested
	if *showVer {
		fmt.Println(version.Info())
		return
	}

	// Load configuration
	loadConfig()

	// Create logger
	logger := log.NewLogger()

	// Convert string log level to log.Level type
	level, err := log.ParseLevel(*logLevel)
	if err != nil {
		fmt.Printf("Invalid log level: %s, defaulting to 'info'\n", *logLevel)
		level = log.InfoLevel
	}
	logger.SetLevel(level)

	logger.Info("Starting Rune Server", log.Str("version", version.Version))

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handler for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		logger.Info("Received signal", log.Str("signal", sig.String()))
		cancel()
	}()

	// Ensure data directory exists
	storeDir := filepath.Join(*dataDir, "store")
	if err := os.MkdirAll(storeDir, 0755); err != nil {
		logger.Error("Failed to create data directory", log.Str("path", storeDir), log.Err(err))
		os.Exit(1)
	}

	// Initialize state store
	logger.Info("Initializing state store", log.Str("path", storeDir))
	stateStore := store.NewBadgerStore(logger)
	if err := stateStore.Open(storeDir); err != nil {
		logger.Error("Failed to open state store", log.Err(err))
		os.Exit(1)
	}
	defer stateStore.Close()

	// Parse API keys
	var apiKeysList []string
	if *apiKeys != "" {
		apiKeysList = parseAPIKeys(*apiKeys)
		logger.Info("Authentication enabled", log.Int("numKeys", len(apiKeysList)))
	} else {
		logger.Warn("Authentication disabled")
	}

	// Create API server options
	serverOpts := []server.Option{
		server.WithGRPCAddr(*grpcAddr),
		server.WithHTTPAddr(*httpAddr),
		server.WithStore(stateStore),
		server.WithLogger(logger),
	}

	// Add auth if API keys are provided
	if len(apiKeysList) > 0 {
		serverOpts = append(serverOpts, server.WithAuth(apiKeysList))
	}

	// Create and start API server
	logger.Info("Creating API server")
	apiServer, err := server.New(serverOpts...)
	if err != nil {
		logger.Error("Failed to create API server", log.Err(err))
		os.Exit(1)
	}

	logger.Info("Starting API server")
	if err := apiServer.Start(); err != nil {
		logger.Error("Failed to start API server", log.Err(err))
		os.Exit(1)
	}

	// Wait for cancellation
	<-ctx.Done()

	// Gracefully stop the API server
	logger.Info("Stopping API server")
	if err := apiServer.Stop(); err != nil {
		logger.Error("Failed to stop API server", log.Err(err))
	}

	logger.Info("Rune server stopped")
}

// parseAPIKeys parses a comma-separated list of API keys.
func parseAPIKeys(keys string) []string {
	if keys == "" {
		return nil
	}

	return splitCSV(keys)
}

// splitCSV splits a comma-separated string into a slice of strings.
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}

	var result []string
	for _, part := range splitAndTrim(s, ',') {
		if part != "" {
			result = append(result, part)
		}
	}

	return result
}

// splitAndTrim splits a string by a separator and trims each part.
func splitAndTrim(s string, sep rune) []string {
	var result []string
	var part string
	for _, c := range s {
		if c == sep {
			result = append(result, part)
			part = ""
		} else {
			part += string(c)
		}
	}
	if part != "" {
		result = append(result, part)
	}
	return result
}
