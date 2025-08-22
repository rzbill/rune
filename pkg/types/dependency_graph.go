package types

// Generic helpers for building and validating dependency graphs across services

import (
	"fmt"
	"strings"
)

// MakeDependencyNodeKey constructs a stable key for a service node as "namespace/name".
func MakeDependencyNodeKey(namespace, name string) string {
	return namespace + "/" + name
}

// DetectDependencyCycles runs cycle detection on an adjacency list of service dependency edges.
// The adjacency list maps node keys ("ns/name") to a slice of neighbor node keys.
// It returns one error per cycle detected with a human-readable path; returns empty slice if no cycles.
func DetectDependencyCycles(adj map[string][]string) []error {
	const (
		colorWhite = 0 // unvisited
		colorGray  = 1 // visiting
		colorBlack = 2 // visited
	)
	color := make(map[string]int)
	stack := make([]string, 0, len(adj))
	var cycleErrs []error

	var dfs func(u string) bool
	dfs = func(u string) bool {
		color[u] = colorGray
		stack = append(stack, u)
		for _, v := range adj[u] {
			if color[v] == colorGray {
				// cycle found: extract stack from first occurrence of v
				start := 0
				for i := range stack {
					if stack[i] == v {
						start = i
						break
					}
				}
				path := append(stack[start:], v)
				cycleErrs = append(cycleErrs, fmt.Errorf("dependency cycle detected: %s", strings.Join(path, " -> ")))
				return true
			}
			if color[v] == colorWhite {
				if dfs(v) {
					return true
				}
			}
		}
		color[u] = colorBlack
		stack = stack[:len(stack)-1]
		return false
	}

	for u := range adj {
		if color[u] == colorWhite {
			dfs(u) // continue scanning to report additional cycles
		}
	}
	return cycleErrs
}
