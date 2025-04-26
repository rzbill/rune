package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/rzbill/rune/pkg/log"
	"github.com/rzbill/rune/pkg/runner"
	"github.com/rzbill/rune/pkg/runner/process"
	"github.com/rzbill/rune/pkg/types"
)

func main() {
	// Create a context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create a logger
	logger := log.NewLogger()

	// Create a process runner
	processRunner, err := process.NewProcessRunner(
		process.WithLogger(logger),
		process.WithNamespace("example"),
	)
	if err != nil {
		logger.Error("Failed to create process runner", log.Err(err))
		os.Exit(1)
	}

	logger.Info("Created process runner")

	// Create an instance with resource limits
	resources := &types.Resources{
		CPU: types.ResourceLimit{
			Request: "100m", // 0.1 CPU cores
			Limit:   "200m", // 0.2 CPU cores
		},
		Memory: types.ResourceLimit{
			Request: "64Mi",  // 64 MiB
			Limit:   "128Mi", // 128 MiB
		},
	}

	instance := &types.Instance{
		ID:        "example-echo",
		Name:      "example-echo",
		NodeID:    "local",
		ServiceID: "example-service",
		Resources: resources,
		Process: &types.ProcessSpec{
			Command: "echo",
			Args:    []string{"Hello from process runner!"},
		},
	}

	// Create the instance
	logger.Info("Creating process instance")
	if err := processRunner.Create(ctx, instance); err != nil {
		logger.Error("Failed to create instance", log.Err(err))
		os.Exit(1)
	}

	// Start the instance
	logger.Info("Starting process instance")
	if err := processRunner.Start(ctx, instance.ID); err != nil {
		logger.Error("Failed to start instance", log.Err(err))
		os.Exit(1)
	}

	// Wait for the process to complete
	time.Sleep(1 * time.Second)

	// Get the status of the process
	status, err := processRunner.Status(ctx, instance.ID)
	if err != nil {
		logger.Error("Failed to get instance status", log.Err(err))
		os.Exit(1)
	}
	logger.Info("Process status", log.Str("status", string(status)))

	// Get the logs from the process
	logger.Info("Getting process logs")
	logs, err := processRunner.GetLogs(ctx, instance.ID, runner.LogOptions{
		Tail:       10,
		Timestamps: true,
	})
	if err != nil {
		logger.Error("Failed to get logs", log.Err(err))
		os.Exit(1)
	}
	defer logs.Close()

	// Print the logs
	logContent, err := io.ReadAll(logs)
	if err != nil {
		logger.Error("Failed to read logs", log.Err(err))
		os.Exit(1)
	}
	fmt.Printf("Log output:\n%s\n", string(logContent))

	// List all instances
	instances, err := processRunner.List(ctx)
	if err != nil {
		logger.Error("Failed to list instances", log.Err(err))
		os.Exit(1)
	}
	logger.Info(fmt.Sprintf("Found %d instances", len(instances)))

	// Remove the instance
	logger.Info("Removing process instance")
	if err := processRunner.Remove(ctx, instance.ID, true); err != nil {
		logger.Error("Failed to remove instance", log.Err(err))
		os.Exit(1)
	}

	logger.Info("Process example completed successfully")
}
