package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
}

// Response represents the API response structure
type Response struct {
	Message     string            `json:"message"`
	Timestamp   time.Time         `json:"timestamp"`
	Environment string            `json:"environment"`
	Version     string            `json:"version"`
	Config      AppConfig         `json:"config"`
	Headers     map[string]string `json:"headers"`
	Hostname    string            `json:"hostname"`
	PID         int               `json:"pid"`
	Uptime      time.Duration     `json:"uptime"`
}

var (
	config    AppConfig
	startTime time.Time
)

// loadConfig loads configuration from environment variables
func loadConfig() AppConfig {
	port, _ := strconv.Atoi(getEnv("PORT", "8080"))
	maxConn, _ := strconv.Atoi(getEnv("MAX_CONNECTIONS", "100"))
	timeout, _ := strconv.Atoi(getEnv("TIMEOUT_SECONDS", "30"))
	debugMode, _ := strconv.ParseBool(getEnv("DEBUG_MODE", "false"))

	return AppConfig{
		Port:           port,
		Host:           getEnv("HOST", "0.0.0.0"),
		Environment:    getEnv("ENVIRONMENT", "development"),
		Version:        getEnv("VERSION", "1.0.0"),
		LogLevel:       getEnv("LOG_LEVEL", "info"),
		DatabaseURL:    getEnv("DATABASE_URL", "postgresql://localhost:5432/hello"),
		APIKey:         getEnv("API_KEY", ""),
		DebugMode:      debugMode,
		MaxConnections: maxConn,
		TimeoutSeconds: timeout,
		FeatureFlags:   getEnv("FEATURE_FLAGS", "basic"),
		CustomMessage:  getEnv("CUSTOM_MESSAGE", "Hello from Rune!"),
	}
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// healthHandler handles health check requests
func healthHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":      "healthy",
		"timestamp":   time.Now(),
		"uptime":      time.Since(startTime),
		"version":     config.Version,
		"environment": config.Environment,
	}

	w.Header().Set("Content-Type", "application/json")
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
		Message:     config.CustomMessage,
		Timestamp:   time.Now(),
		Environment: config.Environment,
		Version:     config.Version,
		Config:      config,
		Headers:     headers,
		Hostname:    hostname,
		PID:         os.Getpid(),
		Uptime:      time.Since(startTime),
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
		"config":       config,
		"environment":  os.Environ(),
		"hostname":     hostname,
		"pid":          os.Getpid(),
		"uptime":       time.Since(startTime),
		"goroutines":   runtime.NumGoroutine(),
		"memory_stats": getMemoryStats(),
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
		"command":   request.Command,
		"timestamp": time.Now(),
		"result":    executeCommand(request.Command),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// executeCommand simulates command execution
func executeCommand(cmd string) string {
	switch cmd {
	case "ls":
		return "main.go\nDockerfile\nservice.yaml\nREADME.md"
	case "pwd":
		return "/app"
	case "env":
		return strings.Join(os.Environ(), "\n")
	case "ps":
		return fmt.Sprintf("PID: %d, Uptime: %v", os.Getpid(), time.Since(startTime))
	case "config":
		configJSON, _ := json.MarshalIndent(config, "", "  ")
		return string(configJSON)
	case "help":
		return "Available commands: ls, pwd, env, ps, config, help"
	default:
		return fmt.Sprintf("Unknown command: %s. Type 'help' for available commands.", cmd)
	}
}

// setupRoutes configures the HTTP routes
func setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/", infoHandler)
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/debug", debugHandler)
	mux.HandleFunc("/interactive", interactiveHandler)

	return mux
}

// logStartup logs startup information
func logStartup() {
	log.Printf("🚀 Starting Rune Hello World Application")
	log.Printf("📋 Configuration:")
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

	log.Println("🛑 Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("❌ Server forced to shutdown: %v", err)
	}

	log.Println("✅ Server exited")
}

func main() {
	startTime = time.Now()

	// Load configuration
	config = loadConfig()

	// Log startup information
	logStartup()

	// Setup routes
	mux := setupRoutes()

	// Create server
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", config.Host, config.Port),
		Handler:      mux,
		ReadTimeout:  time.Duration(config.TimeoutSeconds) * time.Second,
		WriteTimeout: time.Duration(config.TimeoutSeconds) * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("🌐 Server starting on %s:%d", config.Host, config.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Server failed to start: %v", err)
		}
	}()

	// Wait for shutdown signal
	gracefulShutdown(server)
}
