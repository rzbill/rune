package registryauth

import (
	"context"
	"strings"
)

// Provider supplies Docker RegistryAuth for a given image host
type Provider interface {
	Match(host string) bool
	Resolve(ctx context.Context, host string, imageRef string) (string, error)
}

// simple wildcard matcher: supports leading '*.' or '*'
func hostMatches(pattern, host string) bool {
	if pattern == "" {
		return false
	}
	if !strings.Contains(pattern, "*") {
		return strings.EqualFold(pattern, host)
	}
	idx := strings.Index(pattern, "*")
	suffix := pattern[idx+1:]
	return strings.HasSuffix(host, suffix)
}
