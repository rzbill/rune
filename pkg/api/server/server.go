package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	grpc_validator "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/validator"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/rzbill/rune/pkg/api/generated"
	"github.com/rzbill/rune/pkg/api/service"
	"github.com/rzbill/rune/pkg/log"
	"github.com/rzbill/rune/pkg/orchestrator"
	"github.com/rzbill/rune/pkg/runner/manager"
	"github.com/rzbill/rune/pkg/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

// APIServer represents the gRPC API server for Rune.
type APIServer struct {
	options *Options
	logger  log.Logger

	// Core services
	serviceService  *service.ServiceService
	instanceService *service.InstanceService
	logService      *service.LogService
	execService     *service.ExecService
	healthService   *service.HealthService
	secretService   *service.SecretService
	configService   *service.ConfigMapService

	// gRPC server
	grpcServer *grpc.Server

	// HTTP server for REST gateway
	httpServer *http.Server

	// State store
	store store.Store

	// Orchestrator
	orchestrator orchestrator.Orchestrator

	// Shutdown channel
	shutdownCh chan struct{}

	// Wait group for server goroutines
	wg sync.WaitGroup

	// Runner manager
	runnerManager *manager.RunnerManager
}

// New creates a new API server with the given options.
func New(opts ...Option) (*APIServer, error) {
	options := DefaultOptions()
	for _, opt := range opts {
		opt(options)
	}

	logger := options.Logger
	if logger == nil {
		logger = log.GetDefaultLogger().WithComponent("api-server")
	}

	runnerManager := options.RunnerManager
	if runnerManager == nil {
		runnerManager = manager.NewRunnerManager(logger)
	}

	// Initialize the basic server with options
	server := &APIServer{
		options:       options,
		logger:        logger,
		store:         options.Store,
		orchestrator:  options.Orchestrator,
		shutdownCh:    make(chan struct{}),
		runnerManager: runnerManager,
	}

	return server, nil
}

// Start starts the API server.
func (s *APIServer) Start() error {
	// Ensure we have required dependencies
	if s.store == nil {
		return fmt.Errorf("state store is required")
	}

	s.logger.Info("Starting Rune Server")

	// Initialize the runner manager
	if err := s.runnerManager.Initialize(); err != nil {
		s.logger.Warn("Error initializing runners", log.Err(err))
	}

	// Initialize orchestrator if not provided
	if s.orchestrator == nil {
		var err error

		// Use the default orchestrator creation which handles all component setup internally
		s.orchestrator, err = orchestrator.NewDefaultOrchestrator(s.store, s.logger, s.runnerManager)
		if err != nil {
			return fmt.Errorf("failed to create default orchestrator: %w", err)
		}
	}

	// Start the orchestrator
	if err := s.orchestrator.Start(context.Background()); err != nil {
		return fmt.Errorf("failed to start orchestrator: %w", err)
	}

	// Create service implementations
	s.serviceService = service.NewServiceService(s.orchestrator, s.logger)
	s.instanceService = service.NewInstanceService(s.store, s.runnerManager, s.logger)
	s.logService = service.NewLogService(s.store, s.logger, s.orchestrator)
	s.execService = service.NewExecService(s.logger, s.orchestrator)
	s.healthService = service.NewHealthService(s.store, s.logger)
	s.secretService = service.NewSecretService(s.store, s.logger)
	s.configService = service.NewConfigMapService(s.store, s.logger)

	// Start gRPC server
	if err := s.startGRPCServer(); err != nil {
		return fmt.Errorf("failed to start gRPC server: %w", err)
	}

	// Start REST gateway
	if err := s.startRESTGateway(); err != nil {
		return fmt.Errorf("failed to start REST gateway: %w", err)
	}

	// Handle signals for graceful shutdown
	go s.handleSignals()

	return nil
}

// startGRPCServer starts the gRPC server.
func (s *APIServer) startGRPCServer() error {
	// Create listener
	lis, err := net.Listen("tcp", s.options.GRPCAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.options.GRPCAddr, err)
	}

	// Set up server options
	var opts []grpc.ServerOption

	// Add TLS if enabled
	if s.options.EnableTLS {
		creds, err := credentials.NewServerTLSFromFile(s.options.TLSCertFile, s.options.TLSKeyFile)
		if err != nil {
			return fmt.Errorf("failed to load TLS credentials: %w", err)
		}
		opts = append(opts, grpc.Creds(creds))
	}

	// Add middleware
	opts = append(opts, grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
		s.logUnaryInterceptor(),
		s.authInterceptor(),
		grpc_recovery.UnaryServerInterceptor(),
		grpc_validator.UnaryServerInterceptor(),
	)))

	opts = append(opts, grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(
		s.logStreamInterceptor(),
		s.authStreamInterceptor(),
		grpc_recovery.StreamServerInterceptor(),
		grpc_validator.StreamServerInterceptor(),
	)))

	// Create gRPC server
	s.grpcServer = grpc.NewServer(opts...)

	// Register services
	generated.RegisterServiceServiceServer(s.grpcServer, s.serviceService)
	generated.RegisterInstanceServiceServer(s.grpcServer, s.instanceService)
	generated.RegisterLogServiceServer(s.grpcServer, s.logService)
	generated.RegisterExecServiceServer(s.grpcServer, s.execService)
	generated.RegisterHealthServiceServer(s.grpcServer, s.healthService)
	generated.RegisterSecretServiceServer(s.grpcServer, s.secretService)
	generated.RegisterConfigMapServiceServer(s.grpcServer, s.configService)

	// Register reflection service for grpcurl/development
	reflection.Register(s.grpcServer)

	// Start server in a goroutine
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.logger.Info("Starting gRPC server", log.Str("address", s.options.GRPCAddr))
		if err := s.grpcServer.Serve(lis); err != nil {
			s.logger.Error("gRPC server error", log.Err(err))
		}
	}()

	return nil
}

// startRESTGateway starts the REST gateway.
func (s *APIServer) startRESTGateway() error {
	// Create HTTP mux
	mux := runtime.NewServeMux()

	// Set up dial options
	var dialOpts []grpc.DialOption
	if s.options.EnableTLS {
		creds, err := credentials.NewClientTLSFromFile(s.options.TLSCertFile, "")
		if err != nil {
			return fmt.Errorf("failed to load TLS credentials: %w", err)
		}
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
	} else {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Register handlers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Get the gRPC server endpoint
	endpoint := s.options.GRPCAddr

	// Register service handlers
	if err := generated.RegisterServiceServiceHandlerFromEndpoint(ctx, mux, endpoint, dialOpts); err != nil {
		return fmt.Errorf("failed to register service handler: %w", err)
	}

	if err := generated.RegisterInstanceServiceHandlerFromEndpoint(ctx, mux, endpoint, dialOpts); err != nil {
		return fmt.Errorf("failed to register instance handler: %w", err)
	}

	if err := generated.RegisterHealthServiceHandlerFromEndpoint(ctx, mux, endpoint, dialOpts); err != nil {
		return fmt.Errorf("failed to register health handler: %w", err)
	}

	if err := generated.RegisterSecretServiceHandlerFromEndpoint(ctx, mux, endpoint, dialOpts); err != nil {
		return fmt.Errorf("failed to register secret handler: %w", err)
	}

	if err := generated.RegisterConfigMapServiceHandlerFromEndpoint(ctx, mux, endpoint, dialOpts); err != nil {
		return fmt.Errorf("failed to register config map handler: %w", err)
	}

	// Create HTTP server
	s.httpServer = &http.Server{
		Addr:    s.options.HTTPAddr,
		Handler: mux,
	}

	// Start HTTP server in a goroutine
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.logger.Info("Starting REST gateway", log.Str("address", s.options.HTTPAddr))
		var err error
		if s.options.EnableTLS {
			err = s.httpServer.ListenAndServeTLS(s.options.TLSCertFile, s.options.TLSKeyFile)
		} else {
			err = s.httpServer.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			s.logger.Error("REST gateway error", log.Err(err))
		}
	}()

	return nil
}

// Stop stops the API server gracefully.
func (s *APIServer) Stop() error {
	s.logger.Info("Stopping Rune Server")

	// Stop the orchestrator first
	if s.orchestrator != nil {
		s.logger.Info("Stopping orchestrator")
		if err := s.orchestrator.Stop(); err != nil {
			s.logger.Error("Error stopping orchestrator", log.Err(err))
		}
	}

	// Ensure we only close the channel once
	select {
	case <-s.shutdownCh:
		// Channel is already closed, nothing to do
	default:
		close(s.shutdownCh)
	}

	// Stop gRPC server
	if s.grpcServer != nil {
		s.logger.Info("Stopping gRPC server")
		s.grpcServer.GracefulStop()
	}

	// Stop HTTP server
	if s.httpServer != nil {
		s.logger.Info("Stopping REST gateway")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.httpServer.Shutdown(ctx); err != nil {
			s.logger.Error("Error shutting down REST gateway", log.Err(err))
		}
	}

	// Wait for all goroutines to finish
	s.wg.Wait()
	s.logger.Info("Rune Server stopped")

	return nil
}

// handleSignals handles OS signals for graceful shutdown.
func (s *APIServer) handleSignals() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		s.logger.Info("Received signal", log.Str("signal", sig.String()))
		_ = s.Stop()
	case <-s.shutdownCh:
		return
	}
}

// logUnaryInterceptor returns a unary interceptor for logging.
func (s *APIServer) logUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		s.logger.Debug("gRPC request", log.Str("method", info.FullMethod))

		resp, err := handler(ctx, req)

		duration := time.Since(start)
		if err != nil {
			s.logger.Error("gRPC error",
				log.Str("method", info.FullMethod),
				log.Err(err),
				log.Duration("duration", duration))
		} else {
			s.logger.Debug("gRPC response",
				log.Str("method", info.FullMethod),
				log.Duration("duration", duration))
		}

		return resp, err
	}
}

// logStreamInterceptor returns a stream interceptor for logging.
func (s *APIServer) logStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		s.logger.Debug("gRPC stream request", log.Str("method", info.FullMethod))

		err := handler(srv, ss)

		duration := time.Since(start)
		if err != nil {
			s.logger.Error("gRPC stream error",
				log.Str("method", info.FullMethod),
				log.Err(err),
				log.Duration("duration", duration))
		} else {
			s.logger.Debug("gRPC stream complete",
				log.Str("method", info.FullMethod),
				log.Duration("duration", duration))
		}

		return err
	}
}
