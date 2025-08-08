package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// AppConfig holds the application configuration
type AppConfig struct {
	Port           int    `json:"port"`
	Host           string `json:"host"`
	Environment    string `json:"environment"`
	Version        string `json:"version"`
	LogLevel       string `json:"log_level"`
	DatabaseURL    string `json:"database_url"`
	APIKey         string `json:"api_key"`
	DebugMode      bool   `json:"debug_mode"`
	MaxConnections int    `json:"max_connections"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	FeatureFlags   string `json:"feature_flags"`
	CustomMessage  string `json:"custom_message"`
	ServiceName    string `json:"service_name"`
	InstanceID     string `json:"instance_id"`
	ReplicaIndex   int    `json:"replica_index"`
}

// Response represents the API response structure
type Response struct {
	Message      string            `json:"message"`
	Timestamp    time.Time         `json:"timestamp"`
	Environment  string            `json:"environment"`
	Version      string            `json:"version"`
	Config       AppConfig         `json:"config"`
	Headers      map[string]string `json:"headers"`
	Hostname     string            `json:"hostname"`
	PID          int               `json:"pid"`
	Uptime       time.Duration     `json:"uptime"`
	InstanceID   string            `json:"instance_id"`
	ReplicaIndex int               `json:"replica_index"`
}

// HealthStatus represents health check response
type HealthStatus struct {
	Status      string            `json:"status"`
	Timestamp   time.Time         `json:"timestamp"`
	Uptime      time.Duration     `json:"uptime"`
	Version     string            `json:"version"`
	Environment string            `json:"environment"`
	InstanceID  string            `json:"instance_id"`
	Checks      map[string]string `json:"checks"`
}

var (
	config       AppConfig
	startTime    time.Time
	instanceID   string
	requestCount int64
)

// loadConfig loads configuration from environment variables
func loadConfig() AppConfig {
	port, _ := strconv.Atoi(getEnv("PORT", "8080"))
	maxConn, _ := strconv.Atoi(getEnv("MAX_CONNECTIONS", "100"))
	timeout, _ := strconv.Atoi(getEnv("TIMEOUT_SECONDS", "30"))
	debugMode, _ := strconv.ParseBool(getEnv("DEBUG_MODE", "false"))
	replicaIndex, _ := strconv.Atoi(getEnv("REPLICA_INDEX", "0"))

	return AppConfig{
		Port:           port,
		Host:           getEnv("HOST", "0.0.0.0"),
		Environment:    getEnv("ENVIRONMENT", "development"),
		Version:        getEnv("VERSION", "1.0.0"),
		LogLevel:       getEnv("LOG_LEVEL", "info"),
		DatabaseURL:    getEnv("DATABASE_URL", "postgresql://localhost:5432/rune_demo"),
		APIKey:         getEnv("API_KEY", ""),
		DebugMode:      debugMode,
		MaxConnections: maxConn,
		TimeoutSeconds: timeout,
		FeatureFlags:   getEnv("FEATURE_FLAGS", "basic"),
		CustomMessage:  getEnv("CUSTOM_MESSAGE", "Hello from Rune Demo App!"),
		ServiceName:    getEnv("SERVICE_NAME", "rune-demo-app"),
		InstanceID:     getEnv("INSTANCE_ID", ""),
		ReplicaIndex:   replicaIndex,
	}
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// generateInstanceID generates a unique instance ID
func generateInstanceID() string {
	if config.InstanceID != "" {
		return config.InstanceID
	}
	hostname, _ := os.Hostname()
	return fmt.Sprintf("%s-%d-%d", hostname, os.Getpid(), time.Now().Unix())
}

// healthHandler handles health check requests
func healthHandler(w http.ResponseWriter, r *http.Request) {
	checks := map[string]string{
		"database": "healthy",
		"memory":   "healthy",
		"disk":     "healthy",
	}

	// Simulate occasional health check failures for testing
	if rand.Float64() < 0.05 { // 5% chance of failure
		checks["database"] = "unhealthy"
	}

	status := "healthy"
	for _, check := range checks {
		if check == "unhealthy" {
			status = "unhealthy"
			break
		}
	}

	response := HealthStatus{
		Status:      status,
		Timestamp:   time.Now(),
		Uptime:      time.Since(startTime),
		Version:     config.Version,
		Environment: config.Environment,
		InstanceID:  instanceID,
		Checks:      checks,
	}

	w.Header().Set("Content-Type", "application/json")
	if status == "unhealthy" {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	json.NewEncoder(w).Encode(response)
}

// infoHandler handles info requests
func infoHandler(w http.ResponseWriter, r *http.Request) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	// Collect headers
	headers := make(map[string]string)
	for name, values := range r.Header {
		headers[name] = strings.Join(values, ", ")
	}

	response := Response{
		Message:      config.CustomMessage,
		Timestamp:    time.Now(),
		Environment:  config.Environment,
		Version:      config.Version,
		Config:       config,
		Headers:      headers,
		Hostname:     hostname,
		PID:          os.Getpid(),
		Uptime:       time.Since(startTime),
		InstanceID:   instanceID,
		ReplicaIndex: config.ReplicaIndex,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// debugHandler provides debug information
func debugHandler(w http.ResponseWriter, r *http.Request) {
	if !config.DebugMode {
		http.Error(w, "Debug mode not enabled", http.StatusForbidden)
		return
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	debugInfo := map[string]interface{}{
		"config":        config,
		"environment":   os.Environ(),
		"hostname":      hostname,
		"pid":           os.Getpid(),
		"uptime":        time.Since(startTime),
		"goroutines":    runtime.NumGoroutine(),
		"memory_stats":  getMemoryStats(),
		"instance_id":   instanceID,
		"request_count": requestCount,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(debugInfo)
}

// getMemoryStats returns basic memory statistics
func getMemoryStats() map[string]interface{} {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return map[string]interface{}{
		"alloc":       m.Alloc,
		"total_alloc": m.TotalAlloc,
		"sys":         m.Sys,
		"num_gc":      m.NumGC,
		"heap_alloc":  m.HeapAlloc,
		"heap_sys":    m.HeapSys,
	}
}

// interactiveHandler provides an interactive shell-like interface
func interactiveHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		Command string `json:"command"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	response := map[string]interface{}{
		"command":     request.Command,
		"timestamp":   time.Now(),
		"instance_id": instanceID,
		"result":      executeCommand(request.Command),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// executeCommand simulates command execution
func executeCommand(cmd string) string {
	switch cmd {
	case "ls":
		return "main.go\nDockerfile\nservice.yaml\nREADME.md\nconfig.json"
	case "pwd":
		return "/app"
	case "env":
		return strings.Join(os.Environ(), "\n")
	case "ps":
		return fmt.Sprintf("PID: %d, Uptime: %v", os.Getpid(), time.Since(startTime))
	case "config":
		configJSON, _ := json.MarshalIndent(config, "", "  ")
		return string(configJSON)
	case "status":
		return fmt.Sprintf("Instance: %s, Replica: %d, Uptime: %v", instanceID, config.ReplicaIndex, time.Since(startTime))
	case "memory":
		stats := getMemoryStats()
		return fmt.Sprintf("Alloc: %d, Sys: %d, GC: %d", stats["alloc"], stats["sys"], stats["num_gc"])
	case "help":
		return "Available commands: ls, pwd, env, ps, config, status, memory, help"
	default:
		return fmt.Sprintf("Unknown command: %s. Type 'help' for available commands.", cmd)
	}
}

// metricsHandler provides Prometheus-style metrics
func metricsHandler(w http.ResponseWriter, r *http.Request) {
	hostname, _ := os.Hostname()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	metrics := fmt.Sprintf(`# HELP rune_demo_app_requests_total Total number of requests
# TYPE rune_demo_app_requests_total counter
rune_demo_app_requests_total{instance="%s",replica="%d"} %d

# HELP rune_demo_app_uptime_seconds Application uptime in seconds
# TYPE rune_demo_app_uptime_seconds gauge
rune_demo_app_uptime_seconds{instance="%s",replica="%d"} %.2f

# HELP rune_demo_app_memory_bytes Memory usage in bytes
# TYPE rune_demo_app_memory_bytes gauge
rune_demo_app_memory_bytes{instance="%s",replica="%d",type="alloc"} %d
rune_demo_app_memory_bytes{instance="%s",replica="%d",type="sys"} %d

# HELP rune_demo_app_goroutines Number of goroutines
# TYPE rune_demo_app_goroutines gauge
rune_demo_app_goroutines{instance="%s",replica="%d"} %d
`,
		hostname, config.ReplicaIndex, requestCount,
		hostname, config.ReplicaIndex, time.Since(startTime).Seconds(),
		hostname, config.ReplicaIndex, m.Alloc,
		hostname, config.ReplicaIndex, m.Sys,
		hostname, config.ReplicaIndex, runtime.NumGoroutine(),
	)

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(metrics))
}

// setupRoutes configures the HTTP routes
func setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/", infoHandler)
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/debug", debugHandler)
	mux.HandleFunc("/interactive", interactiveHandler)
	mux.HandleFunc("/metrics", metricsHandler)

	return mux
}

// logStartup logs startup information
func logStartup() {
	log.Printf("🚀 Starting Rune Demo Application")
	log.Printf("📋 Configuration:")
	log.Printf("   Service Name: %s", config.ServiceName)
	log.Printf("   Instance ID: %s", instanceID)
	log.Printf("   Replica Index: %d", config.ReplicaIndex)
	log.Printf("   Environment: %s", config.Environment)
	log.Printf("   Version: %s", config.Version)
	log.Printf("   Port: %d", config.Port)
	log.Printf("   Host: %s", config.Host)
	log.Printf("   Debug Mode: %t", config.DebugMode)
	log.Printf("   Log Level: %s", config.LogLevel)
	log.Printf("   Database URL: %s", config.DatabaseURL)
	log.Printf("   Max Connections: %d", config.MaxConnections)
	log.Printf("   Timeout: %d seconds", config.TimeoutSeconds)
	log.Printf("   Feature Flags: %s", config.FeatureFlags)
	log.Printf("   Custom Message: %s", config.CustomMessage)

	if config.APIKey != "" {
		log.Printf("   API Key: [REDACTED]")
	} else {
		log.Printf("   API Key: [NOT SET]")
	}
}

// gracefulShutdown handles graceful shutdown
func gracefulShutdown(server *http.Server) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Printf("🛑 Shutting down server (Instance: %s)...", instanceID)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("❌ Server forced to shutdown: %v", err)
	}

	log.Printf("✅ Server exited (Instance: %s)", instanceID)
}

// middleware for request counting
func requestCounter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		next.ServeHTTP(w, r)
	})
}

func main() {
	startTime = time.Now()

	// Load configuration
	config = loadConfig()

	// Generate instance ID
	instanceID = generateInstanceID()

	// Log startup information
	logStartup()

	// Setup routes
	mux := setupRoutes()

	// Create server with middleware
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", config.Host, config.Port),
		Handler:      requestCounter(mux),
		ReadTimeout:  time.Duration(config.TimeoutSeconds) * time.Second,
		WriteTimeout: time.Duration(config.TimeoutSeconds) * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("🌐 Server starting on %s:%d (Instance: %s)", config.Host, config.Port, instanceID)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Server failed to start: %v", err)
		}
	}()

	// Wait for shutdown signal
	gracefulShutdown(server)
}
