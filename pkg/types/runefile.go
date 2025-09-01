package types

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// RuneFile represents a Rune configuration file
type RuneFile struct {
	// Server configuration
	Server *ServerConfig `yaml:"server,omitempty"`

	// Data directory for persistent storage
	DataDir string `yaml:"data_dir,omitempty"`

	// Client configuration
	Client *ClientConfig `yaml:"client,omitempty"`

	// Docker runner configuration
	Docker *DockerConfig `yaml:"docker,omitempty"`

	// Default namespace
	Namespace string `yaml:"namespace,omitempty"`

	// Authentication configuration
	Auth *AuthConfig `yaml:"auth,omitempty"`

	// Resource limits and requests
	Resources *ResourceConfig `yaml:"resources,omitempty"`

	// Logging configuration
	Log *LogConfig `yaml:"log,omitempty"`

	// Secret encryption configuration
	Secret *SecretConfig `yaml:"secret,omitempty"`

	// Plugin configuration
	Plugins *PluginConfig `yaml:"plugins,omitempty"`

	// Internal tracking for line numbers (not serialized)
	lineInfo map[string]int `json:"-" yaml:"-"`
	rawNode  *yaml.Node     `json:"-" yaml:"-"`
}

func (rf *RuneFile) GetName() string {
	return "rune"
}

// ServerConfig represents server endpoint configuration
type ServerConfig struct {
	GRPCAddress string     `yaml:"grpc_address,omitempty"`
	HTTPAddress string     `yaml:"http_address,omitempty"`
	TLS         *TLSConfig `yaml:"tls,omitempty"`
}

// TLSConfig represents TLS configuration
type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file,omitempty"`
	KeyFile  string `yaml:"key_file,omitempty"`
}

// ClientConfig represents client configuration
type ClientConfig struct {
	Timeout time.Duration `yaml:"timeout,omitempty"`
	Retries int           `yaml:"retries,omitempty"`
}

// DockerConfig represents Docker runner configuration
type DockerConfig struct {
	APIVersion                string `yaml:"api_version,omitempty"`
	FallbackAPIVersion        string `yaml:"fallback_api_version,omitempty"`
	NegotiationTimeoutSeconds int    `yaml:"negotiation_timeout_seconds,omitempty"`
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	APIKeys          string `yaml:"api_keys,omitempty"`
	Provider         string `yaml:"provider,omitempty"`
	Token            string `yaml:"token,omitempty"`
	AllowRemoteAdmin bool   `yaml:"allow_remote_admin,omitempty"`
}

// ResourceConfig represents resource limits and requests
type ResourceConfig struct {
	CPU    *CPUConfig    `yaml:"cpu,omitempty"`
	Memory *MemoryConfig `yaml:"memory,omitempty"`
}

// CPUConfig represents CPU resource configuration
type CPUConfig struct {
	DefaultRequest string `yaml:"default_request,omitempty"`
	DefaultLimit   string `yaml:"default_limit,omitempty"`
}

// MemoryConfig represents memory resource configuration
type MemoryConfig struct {
	DefaultRequest string `yaml:"default_request,omitempty"`
	DefaultLimit   string `yaml:"default_limit,omitempty"`
}

// LogConfig represents logging configuration
type LogConfig struct {
	Level  string `yaml:"level,omitempty"`
	Format string `yaml:"format,omitempty"`
}

// SecretConfig represents secret encryption configuration
type SecretConfig struct {
	Encryption *EncryptionConfig `yaml:"encryption,omitempty"`
}

// EncryptionConfig represents encryption configuration
type EncryptionConfig struct {
	Enabled bool       `yaml:"enabled"`
	KEK     *KEKConfig `yaml:"kek,omitempty"`
}

// KEKConfig represents Key Encryption Key configuration
type KEKConfig struct {
	Source     string            `yaml:"source,omitempty"`
	File       string            `yaml:"file,omitempty"`
	Passphrase *PassphraseConfig `yaml:"passphrase,omitempty"`
}

// PassphraseConfig represents passphrase configuration
type PassphraseConfig struct {
	Enabled bool   `yaml:"enabled"`
	Env     string `yaml:"env,omitempty"`
}

// PluginConfig represents plugin configuration
type PluginConfig struct {
	Dir     string   `yaml:"dir,omitempty"`
	Enabled []string `yaml:"enabled,omitempty"`
}

// ParseRuneFile parses a Rune configuration file from the given file path
func ParseRuneFile(filePath string) (*RuneFile, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Validate top-level keys
	if err := validateRuneFileTopLevelKeys(&node); err != nil {
		return nil, fmt.Errorf("invalid top-level keys: %w", err)
	}

	var runeFile RuneFile
	if err := yaml.Unmarshal(data, &runeFile); err != nil {
		return nil, fmt.Errorf("failed to unmarshal RuneFile: %w", err)
	}

	// Store the raw node for validation
	runeFile.rawNode = &node

	// Initialize line info map
	runeFile.lineInfo = make(map[string]int)
	collectLineInfo(&node, runeFile.lineInfo)

	return &runeFile, nil
}

// validateRuneFileTopLevelKeys validates that all top-level keys are known
func validateRuneFileTopLevelKeys(node *yaml.Node) error {
	if node.Kind != yaml.DocumentNode || len(node.Content) == 0 {
		return fmt.Errorf("invalid YAML document structure")
	}

	root := node.Content[0]
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("root must be a mapping")
	}

	knownKeys := map[string]bool{
		"server":    true,
		"data_dir":  true,
		"client":    true,
		"docker":    true,
		"namespace": true,
		"auth":      true,
		"resources": true,
		"log":       true,
		"secret":    true,
		"plugins":   true,
	}

	for i := 0; i < len(root.Content); i += 2 {
		if i+1 >= len(root.Content) {
			break
		}
		key := root.Content[i]
		if key.Kind != yaml.ScalarNode {
			continue
		}
		keyName := key.Value
		if !knownKeys[keyName] {
			return fmt.Errorf("unknown top-level key '%s' at line %d", keyName, key.Line)
		}
	}

	return nil
}

// collectLineInfo collects line numbers for all keys in the YAML document
func collectLineInfo(node *yaml.Node, lineInfo map[string]int) {
	if node == nil {
		return
	}

	if node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content); i += 2 {
			if i+1 >= len(node.Content) {
				break
			}
			key := node.Content[i]
			value := node.Content[i+1]
			if key.Kind == yaml.ScalarNode {
				lineInfo[key.Value] = key.Line
			}
			collectLineInfo(value, lineInfo)
		}
	} else if node.Kind == yaml.SequenceNode {
		for _, item := range node.Content {
			collectLineInfo(item, lineInfo)
		}
	}
}

// Validate validates the RuneFile configuration
func (rf *RuneFile) Validate() error {
	var errors []string

	// Validate server configuration
	if rf.Server != nil {
		if err := rf.validateServer(); err != nil {
			errors = append(errors, fmt.Sprintf("server: %v", err))
		}
	}

	// Validate client configuration
	if rf.Client != nil {
		if err := rf.validateClient(); err != nil {
			errors = append(errors, fmt.Sprintf("client: %v", err))
		}
	}

	// Validate docker configuration
	if rf.Docker != nil {
		if err := rf.validateDocker(); err != nil {
			errors = append(errors, fmt.Sprintf("docker: %v", err))
		}
	}

	// Validate auth configuration
	if rf.Auth != nil {
		if err := rf.validateAuth(); err != nil {
			errors = append(errors, fmt.Sprintf("auth: %v", err))
		}
	}

	// Validate resources configuration
	if rf.Resources != nil {
		if err := rf.validateResources(); err != nil {
			errors = append(errors, fmt.Sprintf("resources: %v", err))
		}
	}

	// Validate log configuration
	if rf.Log != nil {
		if err := rf.validateLog(); err != nil {
			errors = append(errors, fmt.Sprintf("log: %v", err))
		}
	}

	// Validate secret configuration
	if rf.Secret != nil {
		if err := rf.validateSecret(); err != nil {
			errors = append(errors, fmt.Sprintf("secret: %v", err))
		}
	}

	// Validate plugins configuration
	if rf.Plugins != nil {
		if err := rf.validatePlugins(); err != nil {
			errors = append(errors, fmt.Sprintf("plugins: %v", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation failed:\n%s", strings.Join(errors, "\n"))
	}

	return nil
}

// Lint performs validation and returns a list of errors with line numbers
func (rf *RuneFile) Lint() []error {
	var errors []error

	// Validate the configuration
	if err := rf.Validate(); err != nil {
		// Split the error message and create individual errors with line numbers
		lines := strings.Split(err.Error(), "\n")
		for _, line := range lines {
			if strings.Contains(line, ":") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					message := strings.TrimSpace(parts[1])

					// Get line number for this key
					lineNum := rf.lineInfo[key]
					if lineNum > 0 {
						errors = append(errors, fmt.Errorf("line %d: %s: %s", lineNum, key, message))
					} else {
						errors = append(errors, fmt.Errorf("%s: %s", key, message))
					}
				}
			}
		}
	}

	// Validate structure (unknown fields)
	if rf.rawNode != nil {
		if err := rf.validateStructureFromNode(); err != nil {
			errors = append(errors, err)
		}
	}

	return errors
}

// validateStructureFromNode validates that no unknown fields are present
func (rf *RuneFile) validateStructureFromNode() error {
	if rf.rawNode == nil {
		return nil
	}

	var errors []string
	validateNodeStructure(rf.rawNode, &errors)

	if len(errors) > 0 {
		return fmt.Errorf("structure validation failed:\n%s", strings.Join(errors, "\n"))
	}

	return nil
}

// validateNodeStructure recursively validates YAML node structure
func validateNodeStructure(node *yaml.Node, errors *[]string) {
	if node == nil {
		return
	}

	if node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content); i += 2 {
			if i+1 >= len(node.Content) {
				break
			}
			key := node.Content[i]
			value := node.Content[i+1]

			if key.Kind == yaml.ScalarNode {
				// Check if this is a known field at this level
				if !isKnownField(key.Value, node) {
					*errors = append(*errors, fmt.Sprintf("unknown field '%s' in rune configuration at line %d", key.Value, key.Line))
				}
			}
			validateNodeStructure(value, errors)
		}
	} else if node.Kind == yaml.SequenceNode {
		for _, item := range node.Content {
			validateNodeStructure(item, errors)
		}
	}
}

// isKnownField checks if a field name is known at the current YAML level
func isKnownField(fieldName string, node *yaml.Node) bool {
	// This is a simplified check - in a real implementation, you'd want to track
	// the current path and check against the appropriate struct definition
	knownKeys := map[string]bool{
		"server":                      true,
		"data_dir":                    true,
		"client":                      true,
		"docker":                      true,
		"namespace":                   true,
		"auth":                        true,
		"resources":                   true,
		"log":                         true,
		"secret":                      true,
		"plugins":                     true,
		"grpc_address":                true,
		"http_address":                true,
		"tls":                         true,
		"enabled":                     true,
		"cert_file":                   true,
		"key_file":                    true,
		"timeout":                     true,
		"retries":                     true,
		"api_version":                 true,
		"fallback_api_version":        true,
		"negotiation_timeout_seconds": true,
		"api_keys":                    true,
		"provider":                    true,
		"token":                       true,
		"cpu":                         true,
		"memory":                      true,
		"default_request":             true,
		"default_limit":               true,
		"level":                       true,
		"format":                      true,
		"encryption":                  true,
		"kek":                         true,
		"source":                      true,
		"file":                        true,
		"passphrase":                  true,
		"env":                         true,
		"dir":                         true,
	}

	return knownKeys[fieldName]
}

// Individual validation methods
func (rf *RuneFile) validateServer() error {
	if rf.Server.GRPCAddress == "" && rf.Server.HTTPAddress == "" {
		return fmt.Errorf("at least one of grpc_address or http_address must be specified")
	}
	return nil
}

func (rf *RuneFile) validateClient() error {
	if rf.Client.Timeout < 0 {
		return fmt.Errorf("timeout cannot be negative")
	}
	if rf.Client.Retries < 0 {
		return fmt.Errorf("retries cannot be negative")
	}
	return nil
}

func (rf *RuneFile) validateDocker() error {
	if rf.Docker.NegotiationTimeoutSeconds < 0 {
		return fmt.Errorf("negotiation_timeout_seconds cannot be negative")
	}
	return nil
}

func (rf *RuneFile) validateAuth() error {
	if rf.Auth.Provider != "" && rf.Auth.Provider != "token" && rf.Auth.Provider != "oidc" && rf.Auth.Provider != "none" {
		return fmt.Errorf("provider must be one of: token, oidc, none")
	}
	return nil
}

func (rf *RuneFile) validateResources() error {
	// Add resource validation logic here
	return nil
}

func (rf *RuneFile) validateLog() error {
	if rf.Log.Level != "" {
		validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
		if !validLevels[rf.Log.Level] {
			return fmt.Errorf("log level must be one of: debug, info, warn, error")
		}
	}
	return nil
}

func (rf *RuneFile) validateSecret() error {
	if rf.Secret.Encryption != nil && rf.Secret.Encryption.Enabled {
		if rf.Secret.Encryption.KEK == nil {
			return fmt.Errorf("kek configuration is required when encryption is enabled")
		}
		if rf.Secret.Encryption.KEK.Source == "" {
			return fmt.Errorf("kek source is required when encryption is enabled")
		}
		if rf.Secret.Encryption.KEK.Source != "file" && rf.Secret.Encryption.KEK.Source != "env" && rf.Secret.Encryption.KEK.Source != "generated" {
			return fmt.Errorf("kek source must be one of: file, env, generated")
		}
		if rf.Secret.Encryption.KEK.Source == "file" && rf.Secret.Encryption.KEK.File == "" {
			return fmt.Errorf("kek file path is required when source is 'file'")
		}
	}
	return nil
}

func (rf *RuneFile) validatePlugins() error {
	// Add plugin validation logic here
	return nil
}

// GetLineInfo returns the line number for a given key
func (rf *RuneFile) GetLineInfo(key string) (int, bool) {
	line, exists := rf.lineInfo[key]
	return line, exists
}

// IsRuneConfigFile checks if a file appears to be a Rune configuration file
func IsRuneConfigFile(filePath string) (bool, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false, err
	}

	// Use YAML AST to avoid duplicate-key unmarshal errors
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return false, err
	}
	// Navigate to document root mapping
	if node.Kind != yaml.DocumentNode || len(node.Content) == 0 {
		return false, nil
	}
	root := node.Content[0]
	if root.Kind != yaml.MappingNode {
		return false, nil
	}

	runeKeys := map[string]bool{
		"server":    true,
		"client":    true,
		"auth":      true,
		"secret":    true,
		"plugins":   true,
		"docker":    true,
		"log":       true,
		"namespace": true,
		"resources": true,
	}

	count := 0
	for i := 0; i+1 < len(root.Content); i += 2 {
		k := root.Content[i]
		if k.Kind == yaml.ScalarNode {
			if runeKeys[k.Value] {
				count++
			}
		}
	}
	return count >= 2, nil
}
