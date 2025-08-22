package types

import (
	"fmt"
	"os"
	"regexp"

	"github.com/rzbill/rune/pkg/utils"
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

// NewRunesetRenderer creates a renderer with sane defaults (maxDepth=32).
func NewRunesetRenderer(root string, values map[string]interface{}, ctx map[string]interface{}) *utils.Renderer {
	return &utils.Renderer{
		Root:          root,
		Values:        values,
		Ctx:           ctx,
		FileCache:     make(map[string]string),
		TemplateCache: make(map[string]string),
		Stack:         nil,
		MaxDepth:      32,
		PhRegex:       regexp.MustCompile(`\{\{(.*?)\}\}`),
	}
}

// RenderRunesetYAML performs a minimal render pass supporting values:/context:/file:/template:
// and pass-through for secret:/configmap: placeholders.
func RenderRunesetYAML(input, root string, values map[string]interface{}, ctx map[string]interface{}) (string, error) {
	r := NewRunesetRenderer(root, values, ctx)
	return r.RenderString(input, 0)
}
