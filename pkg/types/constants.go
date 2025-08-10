package types

// Runtime types
type RuntimeType string

const (
	RuntimeTypeContainer RuntimeType = "container"
	RuntimeTypeProcess   RuntimeType = "process"
)

// ResourceType is the type of resource.
type ResourceType string

const (
	// ResourceTypeService is the resource type for services.
	ResourceTypeService ResourceType = "service"

	// ResourceTypeInstance is the resource type for instances.
	ResourceTypeInstance ResourceType = "instance"

	// ResourceTypeNamespace is the resource type for namespaces.
	ResourceTypeNamespace ResourceType = "namespace"

	// ResourceTypeScalingOperation is the resource type for scaling operations.
	ResourceTypeScalingOperation ResourceType = "scaling_operation"

	// ResourceTypeDeletionOperation is the resource type for deletion operations.
	ResourceTypeDeletionOperation ResourceType = "deletion_operation"

	// ResourceTypeSecret represents secrets (encrypted at rest)
	ResourceTypeSecret ResourceType = "secret"

	// ResourceTypeConfigMap represents non-sensitive configs
	ResourceTypeConfigMap ResourceType = "configmap"
)

// RunnerType is the type of runner for an instance.
type RunnerType string

const (
	RunnerTypeTest    RunnerType = "test"
	RunnerTypeDocker  RunnerType = "docker"
	RunnerTypeProcess RunnerType = "process"
)
