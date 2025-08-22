package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// maxFileSize is the maximum size for any single file to prevent decompression bombs
	MAX_FILE_SIZE = 100 * 1024 * 1024 // 100MB
)

// IsDirectory checks if a path is a directory.
func IsDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// FileExists checks if a file exists.
func FileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// GetYAMLFilesInDirectory returns all YAML files (.yaml or .yml) in a directory.
func GetYAMLFilesInDirectory(dirPath string, recursive bool) ([]string, error) {
	var files []string

	if !IsDirectory(dirPath) {
		return nil, fmt.Errorf("not a directory: %s", dirPath)
	}

	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// If it's not a directory and has .yaml or .yml extension, add it to the list
		if !info.IsDir() {
			ext := strings.ToLower(filepath.Ext(path))
			if ext == ".yaml" || ext == ".yml" {
				files = append(files, path)
			}
		} else if path != dirPath && !recursive {
			// Skip subdirectories if not recursive
			return filepath.SkipDir
		}

		return nil
	}

	err := filepath.Walk(dirPath, walkFn)
	if err != nil {
		return nil, err
	}

	return files, nil
}

// ExpandFilePaths expands file paths to include files in directories and glob patterns.
func ExpandFilePaths(paths []string, recursive bool) ([]string, error) {
	var expandedPaths []string

	for _, path := range paths {
		// Check if the path is a directory
		if IsDirectory(path) {
			dirFiles, err := GetYAMLFilesInDirectory(path, recursive)
			if err != nil {
				return nil, fmt.Errorf("error getting YAML files from directory %s: %w", path, err)
			}
			expandedPaths = append(expandedPaths, dirFiles...)
			continue
		}

		// Check if the path contains glob patterns
		if strings.ContainsAny(path, "*?[") {
			matches, err := filepath.Glob(path)
			if err != nil {
				return nil, fmt.Errorf("error expanding glob pattern %s: %w", path, err)
			}

			// Process each match
			for _, match := range matches {
				if IsDirectory(match) {
					// If the match is a directory, get YAML files from it
					dirFiles, err := GetYAMLFilesInDirectory(match, recursive)
					if err != nil {
						return nil, fmt.Errorf("error getting YAML files from directory %s: %w", match, err)
					}
					expandedPaths = append(expandedPaths, dirFiles...)
				} else {
					// If the match is a file, add it directly
					expandedPaths = append(expandedPaths, match)
				}
			}
			continue
		}

		// Regular file path
		if FileExists(path) {
			expandedPaths = append(expandedPaths, path)
		} else {
			return nil, fmt.Errorf("file not found: %s", path)
		}
	}

	return expandedPaths, nil
}

// SafePath safely constructs a target path within a base directory, preventing directory traversal
func SafePath(baseDir, entryName string) (string, error) {
	// Clean the entry name to normalize path separators and remove . and ..
	cleanName := filepath.Clean(entryName)

	// Check for absolute paths or paths starting with ..
	if filepath.IsAbs(cleanName) || strings.HasPrefix(cleanName, "..") {
		return "", fmt.Errorf("unsafe path: %s", entryName)
	}

	// Join with base directory
	target := filepath.Join(baseDir, cleanName)

	// Ensure the resulting path is still within the base directory
	cleanBase := filepath.Clean(baseDir)
	if !strings.HasPrefix(target, cleanBase+string(os.PathSeparator)) && target != cleanBase {
		return "", fmt.Errorf("path escapes base directory: %s", entryName)
	}

	return target, nil
}
