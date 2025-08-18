package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rzbill/rune/pkg/log"
	"github.com/rzbill/rune/pkg/store"
	"github.com/rzbill/rune/pkg/store/repos"
	"github.com/rzbill/rune/pkg/types"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

// ensureBootstrapAdmin creates a default admin user and token on first run if none exist.
func ensureBootstrapAdmin(st store.Store, adminName, adminEmail, outPath string, logger log.Logger) error {
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

	// If an explicit output file was provided, honor it; otherwise add to CLI config context
	if outPath != "" {
		if err := os.MkdirAll(filepath.Dir(outPath), 0700); err != nil {
			return err
		}
		if err := os.WriteFile(outPath, []byte(secret), 0600); err != nil {
			return err
		}
		logger.Info("Bootstrap admin token written", log.Str("path", outPath))
		return nil
	}

	// No explicit output path: store credentials into the local Rune CLI config
	if err := upsertCLIContextWithToken(secret, *grpcAddr, logger); err != nil {
		return err
	}
	return nil
}

// isInteractive returns true if stdin is an interactive terminal (TTY)
func isInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// anyUserExists returns true if at least one user resource exists in the system namespace (MVP)
func anyUserExists(st store.Store) bool {
	var users []types.User
	if err := st.List(context.Background(), types.ResourceTypeUser, "system", &users); err != nil {
		return false
	}
	return len(users) > 0
}

// --- Local helpers to update Rune CLI config (mirrors pkg/cli/cmd/config.go minimal logic) ---

type cliContext struct {
	Server    string `yaml:"server"`
	Token     string `yaml:"token"`
	Namespace string `yaml:"namespace,omitempty"`
}

type cliContextConfig struct {
	CurrentContext string                `yaml:"current-context"`
	Contexts       map[string]cliContext `yaml:"contexts"`
}

func getCLIConfigPath() string {
	// Highest precedence: explicit --cli-config flag if provided
	if cliConfigPath != nil && *cliConfigPath != "" {
		return *cliConfigPath
	}
	// Next: environment variable
	if cfg := os.Getenv("RUNE_CLI_CONFIG"); cfg != "" {
		return cfg
	}
	// Fallback to default in home directory
	home, err := os.UserHomeDir()
	if err != nil {
		return "./.rune/config.yaml"
	}
	return filepath.Join(home, ".rune", "config.yaml")
}

func loadCLIContextConfig() (*cliContextConfig, error) {
	path := getCLIConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &cliContextConfig{CurrentContext: "default", Contexts: make(map[string]cliContext)}, nil
		}
		return nil, err
	}
	var cfg cliContextConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse CLI config: %w", err)
	}
	if cfg.Contexts == nil {
		cfg.Contexts = make(map[string]cliContext)
	}
	if cfg.CurrentContext == "" {
		cfg.CurrentContext = "default"
	}
	return &cfg, nil
}

func saveCLIContextConfig(cfg *cliContextConfig) (string, error) {
	path := getCLIConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return path, fmt.Errorf("failed to create config directory: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return path, fmt.Errorf("failed to marshal CLI config: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return path, fmt.Errorf("failed to write CLI config: %w", err)
	}
	return path, nil
}

func normalizeGRPCAddressForClient(addr string) string {
	// If addr is of form ":7863", convert to "localhost:7863"; otherwise return as-is
	if strings.HasPrefix(addr, ":") {
		return "localhost" + addr
	}
	// Strip any scheme if accidentally provided
	if i := strings.Index(addr, "://"); i >= 0 {
		addr = addr[i+3:]
	}
	return addr
}

func upsertCLIContextWithToken(token, grpcAddress string, logger log.Logger) error {
	cfg, err := loadCLIContextConfig()
	if err != nil {
		return err
	}

	server := normalizeGRPCAddressForClient(grpcAddress)
	ctxName := "server-admin"
	cfg.Contexts[ctxName] = cliContext{Server: server, Token: token, Namespace: "system"}
	cfg.CurrentContext = ctxName

	path, err := saveCLIContextConfig(cfg)
	if err != nil {
		return err
	}
	logger.Info("Bootstrap admin context configured", log.Str("config", path), log.Str("context", ctxName), log.Str("server", server))
	return nil
}
