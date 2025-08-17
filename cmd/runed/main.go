package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/rzbill/rune/internal/config"
	"github.com/rzbill/rune/pkg/api/server"
	"github.com/rzbill/rune/pkg/log"
	"github.com/rzbill/rune/pkg/store"
	"github.com/rzbill/rune/pkg/store/repos"
	"github.com/rzbill/rune/pkg/types"
	"github.com/rzbill/rune/pkg/version"
	"github.com/spf13/viper"
)

var (
	configFile          = flag.String("config", "", "Configuration file path")
	grpcAddr            = flag.String("grpc-addr", ":7863", "gRPC server address")
	httpAddr            = flag.String("http-addr", ":7861", "HTTP server address")
	dataDir             = flag.String("data-dir", "", "Data directory (if not specified, uses OS-specific application data directory)")
	logLevel            = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	debugLogLevel       = flag.Bool("debug", false, "Enable debug mode (shorthand for --log-level=debug)")
	logFormat           = flag.String("log-format", "text", "Log format (text, json)")
	prettyLogs          = flag.Bool("pretty", false, "Enable pretty text log format (shorthand for --log-format=text)")
	bootstrapAdmin      = flag.Bool("bootstrap-admin", true, "Bootstrap admin user and token on first run if none exist")
	bootstrapAdminName  = flag.String("bootstrap-admin-name", "admin", "Initial admin username")
	bootstrapAdminEmail = flag.String("bootstrap-admin-email", "", "Initial admin email (optional)")
	bootstrapTokenOut   = flag.String("bootstrap-token-file", "", "Write bootstrap token to this file (0600). Defaults to <data-dir>/bootstrap-admin-token")
	showHelp            = flag.Bool("help", false, "Show help")
	showVer             = flag.Bool("version", false, "Show version")
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

// initRuntimeConfig initializes runtime settings (viper defaults + config file + env + flags)
func initRuntimeConfig() {
	// Initialize viper
	v := viper.New()

	// 1. Set default values that will be used if nothing else is specified
	defaultDataDir := getDefaultDataDir()
	v.SetDefault("server.grpc_address", fmt.Sprintf(":%d", config.DefaultGRPCPort))
	v.SetDefault("server.http_address", fmt.Sprintf(":%d", config.DefaultHTTPPort))
	v.SetDefault("data_dir", defaultDataDir)
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "text")
	v.SetDefault("auth.api_keys", "")

	// Docker related defaults
	v.SetDefault("docker.fallback_api_version", "1.43")
	v.SetDefault("docker.negotiation_timeout_seconds", 3)

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

	// Explicitly bind key environment variables
	// This is the Kubernetes approach - explicitly declare the env vars we care about
	envVarMappings := map[string]string{
		// Server config
		"RUNE_SERVER_GRPC_ADDRESS": "server.grpc_address",
		"RUNE_SERVER_HTTP_ADDRESS": "server.http_address",
		"RUNE_DATA_DIR":            "data_dir",

		// Docker config - explicitly support the earlier direct env var
		"RUNE_DOCKER_API_VERSION":          "docker.api_version",
		"RUNE_DOCKER_FALLBACK_API_VERSION": "docker.fallback_api_version",
		"RUNE_DOCKER_NEGOTIATION_TIMEOUT":  "docker.negotiation_timeout_seconds",

		// Log config
		"RUNE_LOG_LEVEL":  "log.level",
		"RUNE_LOG_FORMAT": "log.format",

		// Auth config
		"RUNE_AUTH_API_KEYS": "auth.api_keys",
	}

	// Explicitly bind environment variables to configuration keys
	for env, configKey := range envVarMappings {
		_ = v.BindEnv(configKey, env)
	}

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

	if !cmdFlags["log-format"] {
		*logFormat = v.GetString("log.format")
	}

	if !cmdFlags["debug"] {
		*debugLogLevel = v.GetBool("debug")
	}

	if !cmdFlags["pretty"] {
		*prettyLogs = v.GetBool("pretty")
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

	// Initialize runtime configuration
	initRuntimeConfig()

	// If --pretty flag is set, override log format
	if *prettyLogs {
		*logFormat = "text"
	}

	if *debugLogLevel {
		*logLevel = "debug"
	}

	// Create logger with appropriate formatter
	var loggerOpts []log.LoggerOption

	// Convert string log level to log.Level type
	level, err := log.ParseLevel(*logLevel)
	if err != nil {
		fmt.Printf("Invalid log level: %s, defaulting to 'info'\n", *logLevel)
		level = log.InfoLevel
	}
	loggerOpts = append(loggerOpts, log.WithLevel(level))

	// Configure logger format
	switch strings.ToLower(*logFormat) {
	case "json":
		loggerOpts = append(loggerOpts, log.WithFormatter(&log.JSONFormatter{}))
	case "text", "pretty":
		loggerOpts = append(loggerOpts, log.WithFormatter(&log.TextFormatter{}))
	default:
		fmt.Printf("Invalid log format: %s, defaulting to 'text'\n", *logFormat)
		loggerOpts = append(loggerOpts, log.WithFormatter(&log.TextFormatter{}))
	}

	logger := log.NewLogger(loggerOpts...)

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

	// Load typed application config
	appCfg, _ := config.Load(*configFile)
	// If KEK file path not set, default it under the selected data dir so we
	// can auto-generate on first run without needing root permissions.
	if appCfg.Secret.Encryption.KEK.Source == "file" && appCfg.Secret.Encryption.KEK.File == "" {
		appCfg.Secret.Encryption.KEK.File = filepath.Join(*dataDir, "kek.b64")
	}

	// Initialize state store with options (KEK from config)
	logger.Info("Initializing state store", log.Str("path", storeDir))
	stateStore := store.NewBadgerStoreWithOptions(logger, store.StoreOptions{
		Path:                    storeDir,
		SecretEncryptionEnabled: appCfg.Secret.Encryption.Enabled,
		KEKOptions:              appCfg.KEKOptions(),
		SecretLimits:            appCfg.Secret.Limits,
		ConfigLimits:            appCfg.ConfigResource.Limits,
	})
	if err := stateStore.Open(storeDir); err != nil {
		logger.Error("Failed to open state store", log.Err(err))
		os.Exit(1)
	}
	defer stateStore.Close()

	// Bootstrap and resolve registry secrets into viper before runner init
	if err := bootstrapAndResolveRegistryAuth(appCfg, stateStore, logger); err != nil {
		logger.Error("Failed to bootstrap/resolve registry auth", log.Err(err))
		os.Exit(1)
	}

	// Bootstrap admin if none exists
	if *bootstrapAdmin {
		if err := ensureBootstrapAdmin(stateStore, *dataDir, *bootstrapAdminName, *bootstrapAdminEmail, *bootstrapTokenOut, logger); err != nil {
			logger.Error("Failed to bootstrap admin", log.Err(err))
			os.Exit(1)
		}
	} else {
		// Safety: if bootstrap disabled but no users exist, refuse to start to avoid lockout
		if !anyUserExists(stateStore) {
			logger.Error("No users found and --bootstrap-admin=false. Seed a user/token or start with --bootstrap-admin=true.")
			os.Exit(1)
		}
	}

	// Token-based auth is always enabled in MVP
	logger.Info("Authentication enabled (token-based)")

	// Create API server options
	serverOpts := []server.Option{
		server.WithGRPCAddr(*grpcAddr),
		server.WithHTTPAddr(*httpAddr),
		server.WithStore(stateStore),
		server.WithLogger(logger),
	}

	// Enable auth unconditionally
	serverOpts = append(serverOpts, server.WithAuth(nil))

	// Create and start API server
	apiServer, err := server.New(serverOpts...)
	if err != nil {
		logger.Error("Failed to create API server", log.Err(err))
		os.Exit(1)
	}

	if err := apiServer.Start(); err != nil {
		logger.Error("Failed to start API server", log.Err(err))
		os.Exit(1)
	}

	// Wait for cancellation
	<-ctx.Done()

	// Gracefully stop the API server
	if err := apiServer.Stop(); err != nil {
		logger.Error("Failed to stop API server", log.Err(err))
	}

	logger.Info("Rune server stopped")
}

// bootstrapAndResolveRegistryAuth creates/updates referenced Secrets based on runefile config
// and materializes credentials into viper's docker.registries for runner consumption.
func bootstrapAndResolveRegistryAuth(cfg *config.Config, st store.Store, logger log.Logger) error {
	if cfg == nil {
		return nil
	}
	regs := cfg.Docker.Registries
	if len(regs) == 0 {
		return nil
	}
	// For bootstrap we need encryption; if store opts don't include KEK, proceed but secrets won't be encrypted in memory store.
	secRepo := repos.NewSecretRepo(st)

	// Build a new slice of registry maps to set back into viper
	var outRegs []map[string]any
	for _, r := range regs {
		entry := map[string]any{
			"name":     r.Name,
			"registry": r.Registry,
		}
		auth := map[string]any{}
		// Copy simple fields
		if r.Auth.Type != "" {
			auth["type"] = r.Auth.Type
		}
		if r.Auth.Region != "" {
			auth["region"] = r.Auth.Region
		}

		// Handle fromSecret cases
		var fromSecretName string
		var fromSecretNS string
		switch v := r.Auth.FromSecret.(type) {
		case string:
			if v != "" {
				fromSecretName = v
			}
		case map[string]any:
			if n, ok := v["name"].(string); ok {
				fromSecretName = n
			}
			if ns, ok := v["namespace"].(string); ok {
				fromSecretNS = ns
			}
		}
		if fromSecretName != "" {
			if fromSecretNS == "" {
				fromSecretNS = "system"
			}
			// Bootstrap if requested
			if r.Auth.Bootstrap {
				// prepare data map from env-expanded values
				data := map[string]string{}
				for k, val := range r.Auth.Data {
					data[k] = os.ExpandEnv(val)
				}
				if len(data) > 0 {
					ref := types.FormatRef(types.ResourceTypeSecret, fromSecretNS, fromSecretName)
					existing, err := secRepo.Get(context.Background(), ref)
					if err != nil {
						// create
						s := &types.Secret{Name: fromSecretName, Namespace: fromSecretNS, Data: data, Type: "static"}
						if cerr := secRepo.Create(context.Background(), s); cerr != nil {
							logger.Error("Failed to create registry secret", log.Str("ref", ref), log.Err(cerr))
							return cerr
						}
						logger.Info("Created registry secret", log.Str("ref", ref))
					} else if !r.Auth.Immutable {
						// update if different and manage != ignore
						if r.Auth.Manage != "ignore" {
							// naive diff by JSON stringification via types; here just overwrite
							s := &types.Secret{Name: fromSecretName, Namespace: fromSecretNS, Data: data, Type: existing.Type}
							if uerr := secRepo.Update(context.Background(), ref, s); uerr != nil {
								logger.Error("Failed to update registry secret", log.Str("ref", ref), log.Err(uerr))
								return uerr
							}
							logger.Info("Updated registry secret", log.Str("ref", ref))
						}
					}
				}
			}
			// Resolve secret now
			ref := types.FormatRef(types.ResourceTypeSecret, fromSecretNS, fromSecretName)
			s, err := secRepo.Get(context.Background(), ref)
			if err != nil {
				logger.Error("Failed to fetch registry secret", log.Str("ref", ref), log.Err(err))
				return err
			}
			// infer keys: .dockerconfigjson > token > username/password > ecr keys
			if val, ok := s.Data[".dockerconfigjson"]; ok && val != "" {
				auth["type"] = "dockerconfigjson"
				auth["dockerconfigjson"] = val
			} else if tk, ok := s.Data["token"]; ok && tk != "" {
				auth["type"] = "token"
				auth["token"] = tk
			} else if u, uok := s.Data["username"]; uok {
				if p, pok := s.Data["password"]; pok {
					auth["type"] = "basic"
					auth["username"] = u
					auth["password"] = p
				}
			} else if ak, ok := s.Data["awsAccessKeyId"]; ok {
				// ECR explicit keys; region is in runefile
				auth["type"] = "ecr"
				auth["awsAccessKeyId"] = ak
				if sk, ok := s.Data["awsSecretAccessKey"]; ok {
					auth["awsSecretAccessKey"] = sk
				}
				if st, ok := s.Data["awsSessionToken"]; ok {
					auth["awsSessionToken"] = st
				}
			}
		} else {
			// If direct fields provided in runefile, expand env and set
			if r.Auth.Username != "" {
				auth["username"] = os.ExpandEnv(r.Auth.Username)
			}
			if r.Auth.Password != "" {
				auth["password"] = os.ExpandEnv(r.Auth.Password)
			}
			if r.Auth.Token != "" {
				auth["token"] = os.ExpandEnv(r.Auth.Token)
			}
		}

		entry["auth"] = auth
		outRegs = append(outRegs, entry)
	}

	// Write back into viper so runner manager can read
	viper.Set("docker.registries", outRegs)
	return nil
}

// ensureBootstrapAdmin creates a default admin user and token on first run if none exist.
func ensureBootstrapAdmin(st store.Store, dataDir, adminName, adminEmail, outPath string, logger log.Logger) error {
	userRepo := repos.NewUserRepo(st)
	tokenRepo := repos.NewTokenRepo(st)

	// Check if any user exists in system namespace by trying a get on the admin name
	// MVP: treat absence as needing bootstrap. If admin exists, skip.
	if _, err := userRepo.Get(context.Background(), "system", adminName); err == nil {
		return nil
	}

	// Interactive prompt for admin details if TTY
	if isInteractive() {
		reader := bufio.NewReader(os.Stdin)
		fmt.Printf("No users found. Create initial admin.\n")
		fmt.Printf("Admin username [%s]: ", adminName)
		if in, err := reader.ReadString('\n'); err == nil {
			in = strings.TrimSpace(in)
			if in != "" {
				adminName = in
			}
		}
		fmt.Printf("Admin email (optional)%s: ", func() string {
			if adminEmail != "" {
				return " [" + adminEmail + "]"
			}
			return ""
		}())
		if in, err := reader.ReadString('\n'); err == nil {
			in = strings.TrimSpace(in)
			if in != "" {
				adminEmail = in
			}
		}
	}

	// Create admin user
	u := &types.User{Namespace: "system", Name: adminName, ID: uuid.NewString(), Email: adminEmail, Roles: []types.Role{types.RoleAdmin}, CreatedAt: time.Now()}
	if err := userRepo.Create(context.Background(), u); err != nil {
		return err
	}

	// Issue admin token (no expiry by default for bootstrap)
	tok, secret, err := tokenRepo.Issue(context.Background(), "system", "bootstrap-admin", u.ID, "user", []types.Role{types.RoleAdmin}, "bootstrap-admin", 0)
	if err != nil {
		return err
	}
	_ = tok // stored in DB

	// Determine output path (default to data dir)
	if outPath == "" {
		outPath = filepath.Join(dataDir, "bootstrap-admin-token")
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(outPath), 0700); err != nil {
		return err
	}

	// Write token file with 0600
	if err := os.WriteFile(outPath, []byte(secret), 0600); err != nil {
		return err
	}
	logger.Info("Bootstrap admin token written", log.Str("path", outPath))
	return nil
}

// isInteractive returns true if stdin is a terminal
func isInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// anyUserExists returns true if at least one user resource exists in the system namespace (MVP)
func anyUserExists(st store.Store) bool {
	var users []types.User
	if err := st.List(context.Background(), types.ResourceTypeUser, "system", &users); err != nil {
		return false
	}
	return len(users) > 0
}
