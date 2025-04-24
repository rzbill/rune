package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rzbill/rune/pkg/version"
)

func main() {
	// This is a placeholder for the actual server implementation
	// Will be expanded in future tickets

	// For now, just print the version information and wait for termination signal
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println(version.Info())
		return
	}

	fmt.Println("Rune Server - Lightweight Orchestration Platform")
	fmt.Printf("Version: %s\n", version.Version)
	fmt.Println("Use Ctrl+C to stop the server")

	// Wait for termination signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("Shutting down server...")
}
