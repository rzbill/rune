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

	"github.com/rzbill/rune/pkg/api/server"
	"github.com/rzbill/rune/pkg/log"
	"github.com/rzbill/rune/pkg/store"
	"github.com/rzbill/rune/pkg/types"
	"github.com/stretchr/testify/require"
)

// TestHelper provides utilities for integration testing
type TestHelper struct {
	t          *testing.T
	tempDir    string
	apiServer  *server.APIServer
	runeBinary string
	store      *store.MemoryStore
	fixtureDir string
}

// NewTestHelper creates a new test helper
func NewTestHelper(t *testing.T) *TestHelper {
	tempDir, err := os.MkdirTemp("", "rune-test-*")
	require.NoError(t, err)

	// Create fixture directory
	fixtureDir := filepath.Join(tempDir, "fixtures")
	err = os.MkdirAll(fixtureDir, 0755)
	require.NoError(t, err)

	// Find rune binary
	runeBinary := os.Getenv("RUNE_BINARY")
	if runeBinary == "" {
		// Look for test binary
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

	return &TestHelper{
		t:          t,
		tempDir:    tempDir,
		runeBinary: runeBinary,
		fixtureDir: fixtureDir,
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
	return apiServer
}

// RunCommand executes a rune CLI command
func (h *TestHelper) RunCommand(args ...string) (string, error) {
	// Set environment for the test binary
	env := append(os.Environ(),
		fmt.Sprintf("RUNE_FIXTURE_DIR=%s", h.fixtureDir),
	)

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

	// Make sure services resource type exists in the store
	h.store.EnsureResourceType("services")

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
		h.store.EnsureNamespace("services", svc.Namespace)

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
		err = h.store.Create(ctx, "services", svc.Namespace, svc.Name, svc)
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
	h.store.EnsureResourceType("services")
	h.store.EnsureNamespace("services", namespace)

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

	return h.store.Create(ctx, "services", svc.Namespace, svc.Name, svc)
}

// TempDir returns the temporary directory for this test
func (h *TestHelper) TempDir() string {
	return h.tempDir
}
