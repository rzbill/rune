package types

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

// CastFile represents a YAML file that may contain multiple resource specifications.
type CastFile struct {
	Specs      []Spec          `yaml:"specs,omitempty"`
	Services   []ServiceSpec   `yaml:"services,omitempty"`
	Secrets    []SecretSpec    `yaml:"secrets,omitempty"`
	ConfigMaps []ConfigMapSpec `yaml:"configMaps,omitempty"`
	// Internal tracking for line numbers (not serialized)
	lineInfo map[string]int `json:"-" yaml:"-"`
	// Template references extracted during preprocessing
	templateMap map[string]string `json:"-" yaml:"-"`
}

// ParseCastFile reads and parses a cast file, handling template syntax
func ParseCastFile(filename string) (*CastFile, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Preprocess templates to handle {{...}} syntax
	processedData, templateMap, err := preprocessTemplates(data)
	if err != nil {
		return nil, fmt.Errorf("failed to preprocess templates: %w", err)
	}

	// Initialize empty cast file and line info
	var cf CastFile
	cf.lineInfo = make(map[string]int)
	cf.templateMap = templateMap // Store the template map

	// Parse the preprocessed YAML
	var node yaml.Node
	if err := yaml.Unmarshal(processedData, &node); err != nil {
		return nil, fmt.Errorf("failed to parse YAML structure: %w", err)
	}

	// Validate top-level keys are known nodes
	if err := validateTopLevelKeys(&node); err != nil {
		return nil, err
	}

	collectRepeatedSpecs(&node, &cf)

	return &cf, nil
}

// ParseCastFileFromBytes is a helper used in tests to parse cast YAML content from memory.
func ParseCastFileFromBytes(data []byte) (*CastFile, error) {
	// Preprocess templates to handle {{...}} syntax
	processedData, templateMap, err := preprocessTemplates(data)
	if err != nil {
		return nil, fmt.Errorf("failed to preprocess templates: %w", err)
	}

	// Initialize empty cast file and line info
	var cf CastFile
	cf.lineInfo = make(map[string]int)
	cf.templateMap = templateMap

	// Parse the preprocessed YAML
	var node yaml.Node
	if err := yaml.Unmarshal(processedData, &node); err != nil {
		return nil, fmt.Errorf("failed to parse YAML structure: %w", err)
	}

	// Validate top-level keys are known nodes
	if err := validateTopLevelKeys(&node); err != nil {
		return nil, err
	}

	collectRepeatedSpecs(&node, &cf)

	return &cf, nil
}

// IsCastFile performs a lightweight detection to determine if a YAML file
// appears to define Rune resource specs (services, secrets, configMaps).
// It does not validate structure; it only checks for presence of known keys.
func IsCastFile(filename string) (bool, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return false, err
	}
	var m map[string]interface{}
	if err := yaml.Unmarshal(data, &m); err != nil {
		return false, err
	}
	if m == nil {
		return false, nil
	}
	keys := []string{"service", "services", "secret", "secrets", "configMap", "configMaps"}
	for _, k := range keys {
		if _, ok := m[k]; ok {
			return true, nil
		}
	}
	return false, nil
}

// collectRepeatedSpecs traverses the YAML AST and appends all occurrences of
// top-level 'service', 'secret', and 'configMap' mapping nodes into cf.Specs (without deduping).
func collectRepeatedSpecs(node *yaml.Node, cf *CastFile) {
	if node == nil {
		return
	}
	// Descend to document root
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		collectRepeatedSpecs(node.Content[0], cf)
		return
	}
	if node.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i]
		val := node.Content[i+1]
		if key.Value == "service" && val.Kind == yaml.MappingNode {
			var spec ServiceSpec
			if b, err := yaml.Marshal(val); err == nil {
				if err := yaml.Unmarshal(b, &spec); err == nil {
					spec.rawNode = val
					// Restore template references in environment variables
					if cf.templateMap != nil {
						spec.RestoreTemplateReferences(cf.templateMap)
					}
					if !spec.Skip {
						cf.Services = append(cf.Services, spec)
						cf.Specs = append(cf.Specs, &spec)
						// record line info
						name, ns := extractNameNamespace(val)
						cf.lineInfo[makeLineKey("Service", ns, name)] = val.Line
					}
				}
			}
		}
		if key.Value == "secret" && val.Kind == yaml.MappingNode {
			var spec SecretSpec
			if b, err := yaml.Marshal(val); err == nil {
				if err := yaml.Unmarshal(b, &spec); err == nil {
					spec.rawNode = val
					if !spec.Skip {
						cf.Secrets = append(cf.Secrets, spec)
						cf.Specs = append(cf.Specs, &spec)
						// record line info
						name, ns := extractNameNamespace(val)
						cf.lineInfo[makeLineKey("Secret", ns, name)] = val.Line
					}
				}
			}
		}
		if key.Value == "configMap" && val.Kind == yaml.MappingNode {
			var spec ConfigMapSpec
			if b, err := yaml.Marshal(val); err == nil {
				if err := yaml.Unmarshal(b, &spec); err == nil {
					spec.rawNode = val
					if !spec.Skip {
						cf.ConfigMaps = append(cf.ConfigMaps, spec)
						cf.Specs = append(cf.Specs, &spec)
						// record line info
						name, ns := extractNameNamespace(val)
						cf.lineInfo[makeLineKey("Config", ns, name)] = val.Line
					}
				}
			}
		}

		// Also record line info for sequence forms
		if key.Value == "services" && val.Kind == yaml.SequenceNode {
			for _, item := range val.Content {
				if item.Kind == yaml.MappingNode {
					// Build typed spec from YAML node
					var spec ServiceSpec
					if b, err := yaml.Marshal(item); err == nil {
						if err := yaml.Unmarshal(b, &spec); err == nil {
							spec.rawNode = item
							// Restore template references in environment variables
							if cf.templateMap != nil {
								spec.RestoreTemplateReferences(cf.templateMap)
							}
							if !spec.Skip {
								cf.Services = append(cf.Services, spec)
								cf.Specs = append(cf.Specs, &spec)
							}
						}
					}
					name, ns := extractNameNamespace(item)
					cf.lineInfo[makeLineKey("Service", ns, name)] = item.Line
				}
			}
		}
		if key.Value == "secrets" && val.Kind == yaml.SequenceNode {
			for _, item := range val.Content {
				if item.Kind == yaml.MappingNode {
					var spec SecretSpec
					if b, err := yaml.Marshal(item); err == nil {
						if err := yaml.Unmarshal(b, &spec); err == nil {
							spec.rawNode = item
							if !spec.Skip {
								cf.Secrets = append(cf.Secrets, spec)
								cf.Specs = append(cf.Specs, &spec)
							}
						}
					}
					name, ns := extractNameNamespace(item)
					cf.lineInfo[makeLineKey("Secret", ns, name)] = item.Line
				}
			}
		}

		if key.Value == "configMaps" && val.Kind == yaml.SequenceNode {
			for _, item := range val.Content {
				if item.Kind == yaml.MappingNode {
					var spec ConfigMapSpec
					if b, err := yaml.Marshal(item); err == nil {
						if err := yaml.Unmarshal(b, &spec); err == nil {
							spec.rawNode = item
							if !spec.Skip {
								cf.ConfigMaps = append(cf.ConfigMaps, spec)
								cf.Specs = append(cf.Specs, &spec)
							}
						}
					}
					name, ns := extractNameNamespace(item)
					cf.lineInfo[makeLineKey("Config", ns, name)] = item.Line
				}
			}
		}
	}

}

// validateTopLevelKeys ensures only known top-level keys are present in the cast file
func validateTopLevelKeys(node *yaml.Node) error {
	if node == nil {
		return nil
	}
	// Descend to document root
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		return validateTopLevelKeys(node.Content[0])
	}
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("cast file must be a YAML mapping at the top level")
	}
	valid := map[string]bool{
		"service":    true,
		"services":   true,
		"secrets":    true,
		"secret":     true,
		"configMap":  true,
		"configMaps": true,
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i]
		if !valid[key.Value] {
			return fmt.Errorf("unknown top-level key '%s' at line %d", key.Value, key.Line)
		}
	}
	return nil
}

// GetLineInfo returns the approximate line number for a spec by kind/namespace/name
func (cf *CastFile) GetLineInfo(kind, namespace, name string) (int, bool) {
	if cf == nil || cf.lineInfo == nil {
		return 0, false
	}
	line, ok := cf.lineInfo[makeLineKey(kind, namespace, name)]
	return line, ok
}

func makeLineKey(kind, namespace, name string) string {
	return kind + "/" + namespace + "/" + name
}

// extractNameNamespace scans a YAML mapping node for "name" and "namespace"
func extractNameNamespace(m *yaml.Node) (name, namespace string) {
	if m == nil || m.Kind != yaml.MappingNode {
		return "", ""
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		k := m.Content[i]
		v := m.Content[i+1]
		if k.Value == "name" && v.Kind == yaml.ScalarNode {
			name = v.Value
		}
		if k.Value == "namespace" && v.Kind == yaml.ScalarNode {
			namespace = v.Value
		}
	}
	return name, namespace
}

// GetSpecs returns all specs defined in the cast file.
func (cf *CastFile) GetSpecs() []Spec {
	return cf.Specs
}

// Lint validates all specs in the cast file and returns a list of errors.
// It does not stop on first error; all validation errors are collected.
func (cf *CastFile) Lint() []error {
	var errs []error

	// Services
	for i := range cf.Services {
		spec := &cf.Services[i]
		if err := spec.Validate(); err != nil {
			if line, ok := cf.GetLineInfo("Service", spec.GetNamespace(), spec.GetName()); ok {
				errs = append(errs, fmt.Errorf("Service %q (ns=%q) at line %d: %w", spec.GetName(), EnsureNamespace(spec.GetNamespace()), line, err))
			} else {
				errs = append(errs, fmt.Errorf("Service %q (ns=%q): %w", spec.GetName(), EnsureNamespace(spec.GetNamespace()), err))
			}
		}
	}

	// Secrets
	for i := range cf.Secrets {
		spec := &cf.Secrets[i]
		if err := spec.Validate(); err != nil {
			if line, ok := cf.GetLineInfo("Secret", spec.GetNamespace(), spec.GetName()); ok {
				errs = append(errs, fmt.Errorf("Secret %q (ns=%q) at line %d: %w", spec.GetName(), EnsureNamespace(spec.GetNamespace()), line, err))
			} else {
				errs = append(errs, fmt.Errorf("Secret %q (ns=%q): %w", spec.GetName(), EnsureNamespace(spec.GetNamespace()), err))
			}
		}
	}

	// ConfigMaps
	for i := range cf.ConfigMaps {
		spec := &cf.ConfigMaps[i]
		if err := spec.Validate(); err != nil {
			if line, ok := cf.GetLineInfo("Config", spec.GetNamespace(), spec.GetName()); ok {
				errs = append(errs, fmt.Errorf("Config %q (ns=%q) at line %d: %w", spec.GetName(), EnsureNamespace(spec.GetNamespace()), line, err))
			} else {
				errs = append(errs, fmt.Errorf("Config %q (ns=%q): %w", spec.GetName(), EnsureNamespace(spec.GetNamespace()), err))
			}
		}
	}

	return errs
}

// Validate runs Lint and returns a single error if any issues are found.
func (cf *CastFile) Validate() error {
	errs := cf.Lint()
	if len(errs) == 0 {
		return nil
	}
	parts := make([]string, 0, len(errs))
	for _, e := range errs {
		parts = append(parts, e.Error())
	}
	return fmt.Errorf("cast file validation failed:\n%s", strings.Join(parts, "\n"))
}

// GetServices returns all service specs defined in the cast file.
func (cf *CastFile) GetServiceSpecs() []*ServiceSpec {
	var result []*ServiceSpec
	if len(cf.Services) > 0 {
		for i := range cf.Services {
			result = append(result, &cf.Services[i])
		}
	}
	return result
}

// GetServices converts service specs to concrete Service objects.
func (cf *CastFile) GetServices() ([]*Service, error) {
	specs := cf.GetServiceSpecs()
	if len(specs) == 0 {
		return nil, nil
	}
	var result []*Service
	for _, spec := range specs {
		service, err := spec.ToService()
		if err != nil {
			return nil, err
		}
		result = append(result, service)
	}
	return result, nil
}

// GetSecretSpecs returns all secret specs defined in the cast file.
func (cf *CastFile) GetSecretSpecs() []*SecretSpec {
	var result []*SecretSpec
	if len(cf.Secrets) > 0 {
		for i := range cf.Secrets {
			result = append(result, &cf.Secrets[i])
		}
	}
	return result
}

// GetConfigMapSpecs returns all configmap specs defined in the cast file.
func (cf *CastFile) GetConfigMapSpecs() []*ConfigMapSpec {
	var result []*ConfigMapSpec
	if len(cf.ConfigMaps) > 0 {
		for i := range cf.ConfigMaps {
			result = append(result, &cf.ConfigMaps[i])
		}
	}
	return result
}

// GetSecrets converts inline secret entries to concrete Secret objects.
func (cf *CastFile) GetSecrets() ([]*Secret, error) {
	if len(cf.Secrets) == 0 {
		return nil, nil
	}
	var result []*Secret
	for _, spec := range cf.Secrets {
		s := spec
		if s.Namespace == "" {
			s.Namespace = "default"
		}
		if s.Type == "" {
			s.Type = "static"
		}
		if err := s.Validate(); err != nil {
			return nil, err
		}
		sec, err := s.ToSecret()
		if err != nil {
			return nil, err
		}
		result = append(result, sec)
	}
	return result, nil
}

// GetConfigMaps converts inline configmap entries to concrete ConfigMap objects.
func (cf *CastFile) GetConfigMaps() ([]*ConfigMap, error) {
	if len(cf.ConfigMaps) == 0 {
		return nil, nil
	}
	var result []*ConfigMap
	for _, spec := range cf.ConfigMaps {
		c := spec
		if c.Namespace == "" {
			c.Namespace = "default"
		}
		if err := c.Validate(); err != nil {
			return nil, err
		}
		cfg, err := c.ToConfigMap()
		if err != nil {
			return nil, err
		}
		result = append(result, cfg)
	}
	return result, nil
}

// preprocessTemplates handles template syntax in YAML content before parsing
// It replaces template references with placeholders and stores the original references
func preprocessTemplates(data []byte) ([]byte, map[string]string, error) {
	content := string(data)
	templateMap := make(map[string]string)

	// Regex to find template references: {{type:reference}}
	// This handles: {{configmap:name/key}}, {{secret:name/key}}, etc.
	templateRegex := regexp.MustCompile(`\{\{([^}]+)\}\}`)

	// Replace each template reference with a unique placeholder
	placeholderCounter := 0
	processedContent := templateRegex.ReplaceAllStringFunc(content, func(match string) string {
		placeholderCounter++
		placeholder := fmt.Sprintf("__TEMPLATE_PLACEHOLDER_%d__", placeholderCounter)

		// Extract the template content (remove {{ and }})
		templateContent := match[2 : len(match)-2]
		templateMap[placeholder] = templateContent

		return placeholder
	})

	return []byte(processedContent), templateMap, nil
}

// GetTemplateMap returns the map of template placeholders to their original template references
func (cf *CastFile) GetTemplateMap() map[string]string {
	return cf.templateMap
}

// RestoreTemplateReferences replaces template placeholders with their original template syntax
func (cf *CastFile) RestoreTemplateReferences(content string) string {
	result := content
	for placeholder, templateRef := range cf.templateMap {
		result = strings.ReplaceAll(result, placeholder, "{{"+templateRef+"}}")
	}
	return result
}
