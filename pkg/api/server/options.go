package server

import (
	"github.com/rzbill/rune/pkg/log"
	"github.com/rzbill/rune/pkg/runner"
	"github.com/rzbill/rune/pkg/store"
)

// Options defines configuration options for the API server.
type Options struct {
	// GRPCAddr is the address to listen on for gRPC connections.
	GRPCAddr string

	// HTTPAddr is the address to listen on for HTTP connections.
	HTTPAddr string

	// TLSCertFile is the path to the TLS certificate file.
	TLSCertFile string

	// TLSKeyFile is the path to the TLS key file.
	TLSKeyFile string

	// EnableTLS indicates whether to enable TLS.
	EnableTLS bool

	// EnableAuth indicates whether to enable authentication.
	EnableAuth bool

	// APIKeys is a list of valid API keys for authentication.
	APIKeys []string

	// Store is the state store to use.
	Store store.Store

	// DockerRunner is the Docker runner to use.
	DockerRunner runner.Runner

	// ProcessRunner is the process runner to use.
	ProcessRunner runner.Runner

	// Logger is the logger to use.
	Logger log.Logger
}

// DefaultOptions returns the default options for the API server.
func DefaultOptions() *Options {
	return &Options{
		GRPCAddr:   ":8080",
		HTTPAddr:   ":8081",
		EnableTLS:  false,
		EnableAuth: false,
		Logger:     log.GetDefaultLogger().WithComponent("api-server"),
	}
}

// Option is a function that configures the API server options.
type Option func(*Options)

// WithGRPCAddr sets the gRPC address.
func WithGRPCAddr(addr string) Option {
	return func(o *Options) {
		o.GRPCAddr = addr
	}
}

// WithHTTPAddr sets the HTTP address.
func WithHTTPAddr(addr string) Option {
	return func(o *Options) {
		o.HTTPAddr = addr
	}
}

// WithTLS enables TLS with the given certificate and key files.
func WithTLS(certFile, keyFile string) Option {
	return func(o *Options) {
		o.TLSCertFile = certFile
		o.TLSKeyFile = keyFile
		o.EnableTLS = true
	}
}

// WithAuth enables authentication with the given API keys.
func WithAuth(apiKeys []string) Option {
	return func(o *Options) {
		o.APIKeys = apiKeys
		o.EnableAuth = true
	}
}

// WithStore sets the state store.
func WithStore(store store.Store) Option {
	return func(o *Options) {
		o.Store = store
	}
}

// WithDockerRunner sets the Docker runner.
func WithDockerRunner(runner runner.Runner) Option {
	return func(o *Options) {
		o.DockerRunner = runner
	}
}

// WithProcessRunner sets the process runner.
func WithProcessRunner(runner runner.Runner) Option {
	return func(o *Options) {
		o.ProcessRunner = runner
	}
}

// WithLogger sets the logger.
func WithLogger(logger log.Logger) Option {
	return func(o *Options) {
		o.Logger = logger
	}
}
