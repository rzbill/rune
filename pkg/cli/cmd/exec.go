package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/rzbill/rune/pkg/api/client"
	"github.com/rzbill/rune/pkg/log"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	// Exec command flags
	execNamespace string
	execWorkdir   string
	execEnv       []string
	execTTY       bool
	execNoTTY     bool
	execTimeout   string
	execAPIServer string
	execAPIKey    string
)

// ExecOptions defines the configuration for the exec command
type ExecOptions struct {
	Namespace string
	Workdir   string
	Env       map[string]string
	TTY       bool
	Timeout   time.Duration
	APIServer string
	APIKey    string
}

// execCmd represents the exec command
var execCmd = &cobra.Command{
	Use:   "exec TARGET COMMAND [args...]",
	Short: "Execute a command in a running service or instance",
	Long: `Execute commands directly within running service instances.

This command provides interactive access to containers and processes,
allowing for real-time debugging, configuration verification, and
emergency maintenance tasks.

Examples:
  # Start an interactive bash session in a service
  rune exec api bash

  # Execute a one-off command
  rune exec api ls -la /app

  # Execute in a specific instance
  rune exec api-instance-123 ps aux

  # Set working directory and environment variables
  rune exec api --workdir=/app --env=DEBUG=true python debug.py

  # Disable TTY for non-interactive commands
  rune exec api --no-tty python script.py

  # Set a timeout for the exec session
  rune exec api --timeout=30s python long-running-script.py`,
	Aliases:       []string{"e"},
	Args:          cobra.MinimumNArgs(2),
	RunE:          runExec,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.AddCommand(execCmd)

	// Add flags
	execCmd.Flags().StringVarP(&execNamespace, "namespace", "n", "default", "namespace of the service/instance")
	execCmd.Flags().StringVar(&execWorkdir, "workdir", "", "working directory for the command")
	execCmd.Flags().StringArrayVar(&execEnv, "env", []string{}, "environment variables to set (key=value format)")
	execCmd.Flags().BoolVarP(&execTTY, "tty", "t", false, "allocate a pseudo-TTY (defaults to true for interactive commands)")
	execCmd.Flags().BoolVar(&execNoTTY, "no-tty", false, "disable TTY allocation for non-interactive commands")
	execCmd.Flags().StringVar(&execTimeout, "timeout", "5m", "timeout for the exec session")
	execCmd.Flags().StringVar(&execAPIServer, "api-server", "", "address of the API server")
	execCmd.Flags().StringVar(&execAPIKey, "api-key", "", "API key for authentication")

	// After the first positional arg (TARGET), stop parsing flags so command args like '-c' are passed through
	execCmd.Flags().SetInterspersed(false)
}

func runExec(cmd *cobra.Command, args []string) error {
	// Parse arguments
	target := args[0]
	command := args[1:]

	if len(command) == 0 {
		return fmt.Errorf("command cannot be empty")
	}

	// Parse options
	options, err := parseExecOptions()
	if err != nil {
		return fmt.Errorf("failed to parse options: %w", err)
	}

	// Create API client
	apiClient, err := createExecAPIClient(options)
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}
	defer apiClient.Close()

	// Create exec client
	execClient := client.NewExecClient(apiClient)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), options.Timeout)
	defer cancel()

	// Create exec session
	session, err := execClient.NewExecSession(ctx)
	if err != nil {
		return fmt.Errorf("failed to create exec session: %w", err)
	}
	defer session.Close()

	// If neither --tty nor --no-tty explicitly set, auto-detect based on the command
	if !execNoTTY && !execTTY {
		options.TTY = shouldAllocateTTY(command)
	}

	// Initialize the session
	execOptions := &client.ExecOptions{
		Command:    command,
		Env:        options.Env,
		WorkingDir: options.Workdir,
		TTY:        options.TTY,
		Timeout:    options.Timeout,
	}

	// Set terminal size if TTY is enabled
	if options.TTY {
		if width, height, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
			execOptions.TerminalWidth = uint32(width)
			execOptions.TerminalHeight = uint32(height)
		}
	}

	if err := session.Initialize(target, options.Namespace, execOptions); err != nil {
		return fmt.Errorf("failed to initialize exec session: %w", err)
	}

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	// Run the session
	var sessionErr error
	if options.TTY {
		sessionErr = session.RunInteractive()
	} else {
		sessionErr = session.RunNonInteractive()
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return fmt.Errorf("exec session timed out after %v", options.Timeout)
	default:
		return sessionErr
	}
}

func parseExecOptions() (*ExecOptions, error) {
	options := &ExecOptions{
		Namespace: execNamespace,
		Workdir:   execWorkdir,
		TTY:       execTTY,
		APIServer: execAPIServer,
		APIKey:    execAPIKey,
	}

	// Parse timeout
	timeout, err := time.ParseDuration(execTimeout)
	if err != nil {
		return nil, fmt.Errorf("invalid timeout format: %w", err)
	}
	options.Timeout = timeout

	// Parse environment variables
	options.Env = make(map[string]string)
	for _, env := range execEnv {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid environment variable format: %s (expected key=value)", env)
		}
		options.Env[parts[0]] = parts[1]
	}

	// Determine TTY allocation
	if execNoTTY {
		options.TTY = false
	} else if !execTTY {
		// Auto-detect TTY for interactive commands
		options.TTY = shouldAllocateTTY(os.Args[2:]) // Skip "rune" and "exec"
	}

	return options, nil
}

func createExecAPIClient(options *ExecOptions) (*client.Client, error) {
	// Get API server address
	apiServer := options.APIServer
	if apiServer == "" {
		apiServer = "localhost:8443"
	}

	// Create client options
	clientOptions := &client.ClientOptions{
		Address:     apiServer,
		APIKey:      options.APIKey,
		DialTimeout: 30 * time.Second,
		CallTimeout: options.Timeout,
		Logger:      log.GetDefaultLogger().WithComponent("exec-client"),
	}

	// Create client
	apiClient, err := client.NewClient(clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	return apiClient, nil
}

// shouldAllocateTTY determines if TTY allocation is appropriate for the command.
func shouldAllocateTTY(command []string) bool {
	if len(command) == 0 {
		return false
	}

	// Check if we're in a terminal
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return false
	}

	// Common interactive shells
	interactiveShells := []string{"bash", "sh", "zsh", "fish", "tcsh", "dash"}
	for _, shell := range interactiveShells {
		if command[0] == shell {
			return true
		}
	}

	// Commands that benefit from TTY
	ttyCommands := []string{"vim", "nano", "top", "htop", "less", "more", "vi"}
	for _, cmd := range ttyCommands {
		if command[0] == cmd {
			return true
		}
	}

	// Check if we're in a terminal
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return false
	}

	// For other commands, default to non-TTY for better script compatibility
	return false
}

// isInstanceID checks if the target string is an instance ID.
func isInstanceID(target string) bool {
	// Simple heuristic: instance IDs typically contain hyphens and follow a pattern
	// like "service-instance-123"
	return len(target) > 0 && containsSubstring(target, "-instance-")
}

// containsSubstring checks if a string contains a substring.
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			func() bool {
				for i := 0; i <= len(s)-len(substr); i++ {
					if s[i:i+len(substr)] == substr {
						return true
					}
				}
				return false
			}())))
}
