package types

// Runtime types
type RuntimeType string

const (
	RuntimeTypeContainer RuntimeType = "container"
	RuntimeTypeProcess   RuntimeType = "process"

	// ResourceTypeService is the resource type for services.
	ResourceTypeService = "services"

	// ResourceTypeInstance is the resource type for instances.
	ResourceTypeInstance = "instances"
)

// RunnerType is the type of runner for an instance.
type RunnerType string

const (
	RunnerTypeTest    RunnerType = "test"
	RunnerTypeDocker  RunnerType = "docker"
	RunnerTypeProcess RunnerType = "process"
)
