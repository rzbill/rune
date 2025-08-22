package types

import (
	"fmt"
	"os"

	yaml "gopkg.in/yaml.v3"
)

type RunesetSourceType string

const (
	RunesetSourceTypeDirectory      RunesetSourceType = "directory"
	RunesetSourceTypeRemoteArchive  RunesetSourceType = "remote-archive"
	RunesetSourceTypePackageArchive RunesetSourceType = "package-archive"
	RunesetSourceTypeGitHub         RunesetSourceType = "github"
	RunesetSourceTypeUnknown        RunesetSourceType = "unknown"
)

type RunesetManifest struct {
	Name        string                 `yaml:"name"`
	Version     string                 `yaml:"version"`
	Description string                 `yaml:"description"`
	Namespace   string                 `yaml:"namespace"`
	Defaults    map[string]interface{} `yaml:"defaults"`
}

// ParseRunesetManifest reads and parses a runeset manifest YAML file.
func ParseRunesetManifest(manifestPath string) (RunesetManifest, error) {
	var mf RunesetManifest
	b, err := os.ReadFile(manifestPath)
	if err != nil {
		return mf, fmt.Errorf("failed to read runeset.yaml: %w", err)
	}
	if err := yaml.Unmarshal(b, &mf); err != nil {
		return mf, fmt.Errorf("failed to parse runeset.yaml: %w", err)
	}
	return mf, nil
}
