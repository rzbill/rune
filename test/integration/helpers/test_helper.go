package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rzbill/rune/pkg/api/server"
	"github.com/rzbill/rune/pkg/log"
	"github.com/rzbill/rune/pkg/store"
	"github.com/rzbill/rune/pkg/types"
	"github.com/stretchr/testify/require"
)

// StoreType defines the type of store to use in tests
type StoreType string

const (
	StoreTypeMemory StoreType = "memory"
	StoreTypeBadger StoreType = "badger"
)

// TestHelper provides utilities for integration testing
type TestHelper struct {
	t          *testing.T
	tempDir    string
	apiServer  *server.APIServer
	runeBinary string
	store      store.Store
	fixtureDir string
	storeType  StoreType
	memStore   *store.MemoryStore // Keep reference for helper methods
}

// NewTestHelper creates a new test helper with configurable store type
func NewTestHelper(t *testing.T) *TestHelper {
	// Check environment variable for store type
	storeType := StoreTypeBadger // Default to BadgerDB store (production-like)
	if envStoreType := os.Getenv("RUNE_TEST_STORE_TYPE"); envStoreType != "" {
		switch envStoreType {
		case "memory", "Memory", "MEMORY":
			storeType = StoreTypeMemory
		case "badger", "Badger", "BADGER":
			storeType = StoreTypeBadger
		default:
			t.Logf("Unknown store type '%s', defaulting to BadgerDB", envStoreType)
			storeType = StoreTypeBadger
		}
	}

	return NewTestHelperWithStore(t, storeType)
}

// NewTestHelperWithStore creates a new test helper with specified store type
func NewTestHelperWithStore(t *testing.T, storeType StoreType) *TestHelper {
	tempDir, err := os.MkdirTemp("", "rune-test-*")
	require.NoError(t, err)

	// Create fixture directory
	fixtureDir := filepath.Join(tempDir, "fixtures")
	err = os.MkdirAll(fixtureDir, 0755)
	require.NoError(t, err)

	// Always use rune-test binary for integration tests
	// This gives us full control over the test environment
	runeBinary := os.Getenv("RUNE_BINARY")
	if runeBinary == "" {
		// Look for rune-test binary in the project's bin directory first
		// Get the current working directory to build absolute path
		cwd, err := os.Getwd()
		if err == nil {
			projectBin := filepath.Join(cwd, "bin", "rune-test")
			if _, err := os.Stat(projectBin); err == nil {
				runeBinary = projectBin
			}
		}

		// If not found in project bin, fall back to PATH lookup
		if runeBinary == "" {
			runeBinary, err = exec.LookPath("rune-test")
			if err != nil {
				// Build the test binary
				cmd := exec.Command("go", "build", "-o", filepath.Join(tempDir, "rune-test"), "github.com/rzbill/rune/cmd/rune-test")
				output, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("Failed to build test binary: %s\nOutput: %s", err, output)
				}
				runeBinary = filepath.Join(tempDir, "rune-test")
			}
		}
	}

	// Initialize store based on type
	var storeInstance store.Store
	var memStore *store.MemoryStore

	switch storeType {
	case StoreTypeMemory:
		memStore = store.NewMemoryStore()
		storeInstance = memStore
	case StoreTypeBadger:
		logger := log.GetDefaultLogger().WithComponent("test-badger-store")
		badgerStore := store.NewBadgerStore(logger)
		// Open store with temporary directory
		err = badgerStore.Open(filepath.Join(tempDir, "badger"))
		require.NoError(t, err)
		storeInstance = badgerStore
	default:
		t.Fatalf("Unknown store type: %s", storeType)
	}

	return &TestHelper{
		t:          t,
		tempDir:    tempDir,
		runeBinary: runeBinary,
		fixtureDir: fixtureDir,
		store:      storeInstance,
		storeType:  storeType,
		memStore:   memStore,
	}
}

// Cleanup removes temporary test resources
func (h *TestHelper) Cleanup() {
	if h.apiServer != nil {
		// Protect against double-closing the server
		server := h.apiServer
		h.apiServer = nil
		server.Stop()
	}
	os.RemoveAll(h.tempDir)
}

// StartAPIServer starts a test API server
func (h *TestHelper) StartAPIServer() *server.APIServer {
	// Set log level to error for tests to suppress repetitive background controller logs
	log.SetDefaultLogger(log.NewLogger(log.WithLevel(log.ErrorLevel)))
	logger := log.GetDefaultLogger().WithComponent("test-api-server")

	// Create a memory store for testing
	memStore := store.NewMemoryStore()
	h.store = memStore

	// Create API server options
	opts := []server.Option{
		server.WithStore(memStore),
		server.WithLogger(logger),
		server.WithGRPCAddr(":0"), // Random port
		server.WithHTTPAddr(":0"), // Random port
	}

	apiServer, err := server.New(opts...)
	require.NoError(h.t, err)

	err = apiServer.Start()
	require.NoError(h.t, err)

	h.apiServer = apiServer

	// Note: Since we're using rune-test binary, we don't need to set up
	// environment variables for API server connection

	return apiServer
}

// RunCommand executes a rune CLI command
func (h *TestHelper) RunCommand(args ...string) (string, error) {
	// Set environment for the test binary
	env := append(os.Environ(),
		fmt.Sprintf("RUNE_FIXTURE_DIR=%s", h.fixtureDir),
		"RUNE_LOG_LEVEL=error", // Set log level to error to suppress repetitive logs
	)

	// Check if this is a watch command
	for _, arg := range args {
		if arg == "--watch" {
			env = append(env, "RUNE_TEST_WATCH=true")
			break
		}
	}

	// Run the command
	cmd := exec.Command(h.runeBinary, args...)
	cmd.Env = env

	// Get output
	out, err := cmd.CombinedOutput()

	// After running a cast command, load any fixtures that were created
	if len(args) > 0 && args[0] == "cast" && err == nil {
		h.loadFixtures(context.Background())
	}

	return string(out), err
}

// RunCommandWithTimeout executes a rune CLI command with a timeout
func (h *TestHelper) RunCommandWithTimeout(timeout time.Duration, args ...string) (string, error) {

	// Set environment for the test binary
	env := append(os.Environ(),
		fmt.Sprintf("RUNE_FIXTURE_DIR=%s", h.fixtureDir),
		"RUNE_LOG_LEVEL=error", // Set log level to error to suppress repetitive logs
	)

	// Check if this is a watch command
	for _, arg := range args {
		if arg == "--watch" {
			env = append(env, "RUNE_TEST_WATCH=true")
		}
	}

	// Create context with timeout
	fmt.Printf("DEBUG: Creating context with timeout\n")
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Run the command with context
	fmt.Printf("DEBUG: Creating exec.CommandContext\n")
	cmd := exec.CommandContext(ctx, h.runeBinary, args...)
	fmt.Printf("DEBUG: Setting environment variables\n")
	cmd.Env = env

	// Get output
	fmt.Printf("DEBUG: Calling cmd.CombinedOutput()\n")
	out, err := cmd.CombinedOutput()
	fmt.Printf("DEBUG: cmd.CombinedOutput() completed\n")

	// After running a cast command, load any fixtures that were created
	if len(args) > 0 && args[0] == "cast" && err == nil {
		h.loadFixtures(context.Background())
	}

	return string(out), err
}

// loadFixtures loads any fixture files in the fixture directory into the memory store
func (h *TestHelper) loadFixtures(ctx context.Context) error {
	if h.store == nil {
		return fmt.Errorf("store not initialized")
	}

	// Read fixture directory
	files, err := ioutil.ReadDir(h.fixtureDir)
	if err != nil {
		return err
	}

	// Make sure service resource type exists in the store
	if h.memStore != nil {
		h.memStore.EnsureResourceType(types.ResourceTypeService)
	}

	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			continue
		}

		// Read fixture file
		data, err := ioutil.ReadFile(filepath.Join(h.fixtureDir, file.Name()))
		if err != nil {
			return err
		}

		// Parse service
		var serviceMap map[string]interface{}
		if err := json.Unmarshal(data, &serviceMap); err != nil {
			return err
		}

		// Convert to service type
		svc := &types.Service{}

		// Set basic fields
		if name, ok := serviceMap["name"].(string); ok {
			svc.Name = name
		}

		if namespace, ok := serviceMap["namespace"].(string); ok {
			svc.Namespace = namespace
		} else {
			svc.Namespace = "default"
		}

		// Make sure the namespace exists in the store
		if h.memStore != nil {
			h.memStore.EnsureNamespace(types.ResourceTypeService, svc.Namespace)
		}

		if runtime, ok := serviceMap["runtime"].(string); ok {
			svc.Runtime = types.RuntimeType(runtime)
		}

		if image, ok := serviceMap["image"].(string); ok {
			svc.Image = image
		}

		if command, ok := serviceMap["command"].([]interface{}); ok {
			svc.Command = command[0].(string)
			if len(command) > 1 {
				args := make([]string, len(command)-1)
				for i, arg := range command[1:] {
					args[i] = arg.(string)
				}
				svc.Args = args
			}
		}

		if scale, ok := serviceMap["scale"].(float64); ok {
			svc.Scale = int(scale)
		} else if scale, ok := serviceMap["scale"].(string); ok {
			// Try to parse the string
			var s int
			fmt.Sscanf(scale, "%d", &s)
			svc.Scale = s
		}

		// Set status to running by default
		svc.Status = types.ServiceStatusRunning

		// Store the service
		err = h.store.Create(ctx, types.ResourceTypeService, svc.Namespace, svc.Name, svc)
		if err != nil {
			// If it already exists, that's not a fatal error
			if !strings.Contains(err.Error(), "already exists") {
				return err
			}
		}
	}

	return nil
}

// GetService directly accesses the memory store to verify a service was created
func (h *TestHelper) GetService(ctx context.Context, namespace, name string) (*types.Service, error) {
	if h.store == nil {
		return nil, fmt.Errorf("store not initialized")
	}

	// First ensure the namespace exists
	if h.memStore != nil {
		h.memStore.EnsureResourceType(types.ResourceTypeService)
	}
	if h.memStore != nil {
		h.memStore.EnsureNamespace(types.ResourceTypeService, namespace)
	}

	var svc types.Service
	err := h.store.Get(ctx, types.ResourceTypeService, namespace, name, &svc)
	if err != nil {
		return nil, err
	}

	return &svc, nil
}

// CreateTestService creates a service directly in the store for testing
func (h *TestHelper) CreateTestService(ctx context.Context, svc *types.Service) error {
	if h.store == nil {
		return fmt.Errorf("store not initialized")
	}

	if svc.Namespace == "" {
		svc.Namespace = "default"
	}

	return h.store.Create(ctx, types.ResourceTypeService, svc.Namespace, svc.Name, svc)
}

// TempDir returns the temporary directory for this test
func (h *TestHelper) TempDir() string {
	return h.tempDir
}
