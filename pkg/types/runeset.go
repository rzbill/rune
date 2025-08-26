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

type RunesetContext struct {
	Namespace   string
	Mode        string
	ReleaseName string
	Runeset     RunesetManifest
}

func (ctx RunesetContext) GetValues() map[string]interface{} {
	return map[string]interface{}{
		"namespace":   ctx.Namespace,
		"mode":        ctx.Mode,
		"releaseName": ctx.ReleaseName,
		"runeset": map[string]interface{}{
			"name":        ctx.Runeset.Name,
			"version":     ctx.Runeset.Version,
			"description": ctx.Runeset.Description,
			"namespace":   ctx.Runeset.Namespace,
			"defaults":    ctx.Runeset.Defaults,
		},
	}
}

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
