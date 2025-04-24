// Package version provides version information for the Rune platform.
package version

import (
	"fmt"
	"runtime"
)

var (
	// Version is the current version of Rune.
	// This is set during build time via ldflags.
	Version = "dev"

	// BuildTime is the time when the binary was built.
	// This is set during build time via ldflags.
	BuildTime = "unknown"

	// Commit is the git commit SHA that the binary was built from.
	// This is set during build time via ldflags.
	Commit = "unknown"
)

// Info returns version information as a formatted string.
func Info() string {
	commitID := Commit
	if len(commitID) > 8 {
		commitID = commitID[:8]
	}

	return fmt.Sprintf("Rune %s (%s) - %s %s/%s",
		Version,
		commitID,
		BuildTime,
		runtime.GOOS,
		runtime.GOARCH,
	)
}

// Map returns version information as a map.
func Map() map[string]string {
	return map[string]string{
		"version":   Version,
		"commit":    Commit,
		"buildTime": BuildTime,
		"goVersion": runtime.Version(),
		"os":        runtime.GOOS,
		"arch":      runtime.GOARCH,
	}
}
