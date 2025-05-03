package controllers

import (
	"testing"

	"github.com/rzbill/rune/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestPrepareEnvVars(t *testing.T) {
	// Create a test service with some env vars and ports
	service := &types.Service{
		ID:        "test-service",
		Name:      "test-service",
		Namespace: "default",
		Env: map[string]string{
			"SERVICE_VAR1": "value1",
			"SERVICE_VAR2": "value2",
		},
		Ports: []types.ServicePort{
			{
				Name: "http",
				Port: 8080,
			},
			{
				Name: "metrics",
				Port: 9090,
			},
		},
	}

	// Create a test instance
	instance := &types.Instance{
		ID:        "test-instance-1",
		Name:      "test-instance-1",
		Namespace: "default",
		ServiceID: "test-service",
	}

	// Prepare environment variables
	envVars := prepareEnvVars(service, instance)

	// Verify the results
	assert.NotNil(t, envVars, "Environment variables should not be nil")

	// Check service-defined vars
	assert.Equal(t, "value1", envVars["SERVICE_VAR1"])
	assert.Equal(t, "value2", envVars["SERVICE_VAR2"])

	// Check built-in vars
	assert.Equal(t, "test-service", envVars["RUNE_SERVICE_NAME"])
	assert.Equal(t, "default", envVars["RUNE_SERVICE_NAMESPACE"])
	assert.Equal(t, "test-instance-1", envVars["RUNE_INSTANCE_ID"])

	// Check normalized vars
	assert.Equal(t, "test-service.default.rune", envVars["TEST_SERVICE_SERVICE_HOST"])
	assert.Equal(t, "8080", envVars["TEST_SERVICE_SERVICE_PORT"])
	assert.Equal(t, "8080", envVars["TEST_SERVICE_SERVICE_PORT_HTTP"])
	assert.Equal(t, "9090", envVars["TEST_SERVICE_SERVICE_PORT_METRICS"])
}

func TestPrepareEnvVarsWithHyphenatedNames(t *testing.T) {
	// Create a test service with hyphenated names
	service := &types.Service{
		ID:        "test-hyphenated-service",
		Name:      "test-hyphenated-service",
		Namespace: "test-ns",
		Ports: []types.ServicePort{
			{
				Name: "api-port",
				Port: 8000,
			},
		},
	}

	// Create a test instance
	instance := &types.Instance{
		ID:        "test-instance-2",
		Name:      "test-instance-2",
		Namespace: "test-ns",
		ServiceID: "test-hyphenated-service",
	}

	// Prepare environment variables
	envVars := prepareEnvVars(service, instance)

	// Verify the results
	assert.NotNil(t, envVars, "Environment variables should not be nil")

	// Check normalization of hyphenated names
	assert.Equal(t, "test-hyphenated-service.test-ns.rune", envVars["TEST_HYPHENATED_SERVICE_SERVICE_HOST"])
	assert.Equal(t, "8000", envVars["TEST_HYPHENATED_SERVICE_SERVICE_PORT"])
	assert.Equal(t, "8000", envVars["TEST_HYPHENATED_SERVICE_SERVICE_PORT_API_PORT"])
}
