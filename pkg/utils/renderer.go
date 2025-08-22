package utils

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

// Renderer provides cached, guarded rendering of runeset placeholders.
type Renderer struct {
	Root          string
	Values        map[string]interface{}
	Ctx           map[string]interface{}
	FileCache     map[string]string
	TemplateCache map[string]string
	Stack         []string // include stack for cycle detection (absolute paths)
	MaxDepth      int
	PhRegex       *regexp.Regexp
}

// RenderString renders a YAML text with recursion/depth guards and caching.
func (r *Renderer) RenderString(input string, depth int) (string, error) {
	if depth > r.MaxDepth {
		return "", fmt.Errorf("template recursion exceeded max depth %d", r.MaxDepth)
	}
	lines := strings.Split(input, "\n")
	var out []string
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		// Standalone inclusion for template: (full line)
		if strings.HasPrefix(trim, "{{") && strings.HasSuffix(trim, "}}") {
			inner := strings.TrimSpace(trim[2 : len(trim)-2])
			if strings.HasPrefix(inner, "template:") {
				path := strings.TrimSpace(strings.TrimPrefix(inner, "template:"))
				rendered, err := r.renderIncludeTemplate(path, indent, depth)
				if err != nil {
					return "", err
				}
				out = append(out, rendered...)
				continue
			}
		}
		// Process inline placeholders (values:/context:) and whole-node insertions including file:
		processed, err := r.processLine(line, indent, depth)
		if err != nil {
			return "", err
		}
		if processed == nil {
			// line consumed (e.g., file: whole-node)
			continue
		}
		out = append(out, *processed)
	}
	return strings.Join(out, "\n"), nil
}

func (r *Renderer) renderIncludeTemplate(path, indent string, depth int) ([]string, error) {
	incPath := filepath.Join(r.Root, path)
	baseRoot, _ := filepath.Abs(r.Root)
	incAbs, _ := filepath.Abs(incPath)
	if !strings.HasPrefix(incAbs, baseRoot+string(os.PathSeparator)) {
		return nil, fmt.Errorf("template include escapes runeset root: %s", path)
	}
	if r.inStack(incAbs) {
		return nil, fmt.Errorf("template include cycle detected: %s", incAbs)
	}
	content, err := r.readCached(incAbs)
	if err != nil {
		return nil, fmt.Errorf("include template %s: %w", path, err)
	}
	r.push(incAbs)
	rendered, err := r.RenderString(content, depth+1)
	r.pop()
	if err != nil {
		return nil, err
	}
	var out []string
	for _, l := range strings.Split(rendered, "\n") {
		if l == "" {
			out = append(out, "")
			continue
		}
		out = append(out, indent+l)
	}
	return out, nil
}

// placeholder AST (Design C)
type refType int

const (
	refValues refType = iota
	refContext
)

type filterNode struct {
	name string
	args []interface{}
}

type placeholderExpr struct {
	rt      refType
	path    string
	filters []filterNode
}

func (r *Renderer) parsePlaceholder(expr string) (*placeholderExpr, error) {
	// Split pipeline by unquoted |
	parts := splitPipeline(expr)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty placeholder")
	}
	head := strings.TrimSpace(parts[0])
	pe := &placeholderExpr{}
	switch {
	case strings.HasPrefix(head, "values:"):
		pe.rt = refValues
		pe.path = strings.TrimSpace(strings.TrimPrefix(head, "values:"))
	case strings.HasPrefix(head, "context:"):
		pe.rt = refContext
		pe.path = strings.TrimSpace(strings.TrimPrefix(head, "context:"))
	default:
		return nil, fmt.Errorf("unknown placeholder head: %s", head)
	}
	// Filters
	for i := 1; i < len(parts); i++ {
		seg := strings.TrimSpace(parts[i])
		if seg == "" {
			continue
		}
		name, argStr := splitFilter(seg)
		fn := filterNode{name: name}
		if argStr != "" {
			var arg interface{}
			if err := yaml.Unmarshal([]byte(argStr), &arg); err != nil {
				return nil, fmt.Errorf("invalid filter arg for %s: %w", name, err)
			}
			fn.args = []interface{}{arg}
		}
		pe.filters = append(pe.filters, fn)
	}
	return pe, nil
}

func splitPipeline(s string) []string {
	var parts []string
	cur := &strings.Builder{}
	inSingle := false
	inDouble := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			cur.WriteByte(ch)
			continue
		}
		if ch == '"' && !inSingle {
			inDouble = !inDouble
			cur.WriteByte(ch)
			continue
		}
		if ch == '|' && !inSingle && !inDouble {
			parts = append(parts, strings.TrimSpace(cur.String()))
			cur.Reset()
			continue
		}
		cur.WriteByte(ch)
	}
	if cur.Len() > 0 {
		parts = append(parts, strings.TrimSpace(cur.String()))
	}
	return parts
}

func splitFilter(seg string) (string, string) {
	// name[: arg]
	idx := strings.Index(seg, ":")
	if idx < 0 {
		return strings.TrimSpace(seg), ""
	}
	name := strings.TrimSpace(seg[:idx])
	arg := strings.TrimSpace(seg[idx+1:])
	return name, arg
}

func (r *Renderer) evalPlaceholder(pe *placeholderExpr) (interface{}, bool, error) {
	var src map[string]interface{}
	if pe.rt == refValues {
		src = r.Values
	} else {
		src = r.Ctx
	}
	val, ok := lookupRunesetValue(src, pe.path)
	// Apply filters left-to-right
	for _, f := range pe.filters {
		switch f.name {
		case "default":
			if len(f.args) != 1 {
				return nil, false, fmt.Errorf("default requires exactly one argument")
			}
			if !ok || val == nil {
				val = f.args[0]
				ok = true
			}
		default:
			return nil, false, fmt.Errorf("unknown filter: %s", f.name)
		}
	}
	return val, ok, nil
}

func (r *Renderer) processLine(line, indent string, depth int) (*string, error) {
	builder := &bytes.Buffer{}
	restOfLine := line
	for {
		loc := r.PhRegex.FindStringIndex(restOfLine)
		if loc == nil {
			builder.WriteString(restOfLine)
			break
		}
		before := restOfLine[:loc[0]]
		match := restOfLine[loc[0]:loc[1]]
		inner := strings.TrimSpace(match[2 : len(match)-2])
		rest := restOfLine[loc[1]:]
		// Whole-node insertions if key: {{...}} at end
		if strings.HasSuffix(before, ": ") && strings.TrimSpace(rest) == "" {
			// file:
			if strings.HasPrefix(inner, "file:") {
				path := strings.TrimSpace(strings.TrimPrefix(inner, "file:"))
				incPath := filepath.Join(r.Root, path)
				baseRoot, _ := filepath.Abs(r.Root)
				incAbs, _ := filepath.Abs(incPath)
				if !strings.HasPrefix(incAbs, baseRoot+string(os.PathSeparator)) {
					return nil, fmt.Errorf("file include escapes runeset root: %s", path)
				}
				if r.inStack(incAbs) {
					return nil, fmt.Errorf("file include cycle detected: %s", incAbs)
				}
				content, err := r.readCached(incAbs)
				if err != nil {
					return nil, fmt.Errorf("include file %s: %w", path, err)
				}
				r.push(incAbs)
				renderedInc, err := r.RenderString(content, depth+1)
				r.pop()
				if err != nil {
					return nil, err
				}
				var outLines []string
				outLines = append(outLines, before+"|-")
				for _, l := range strings.Split(renderedInc, "\n") {
					outLines = append(outLines, indent+"  "+l)
				}
				joined := strings.Join(outLines, "\n")
				return &joined, nil
			}
			// values:/context: whole-node with optional filters
			if strings.HasPrefix(inner, "values:") || strings.HasPrefix(inner, "context:") {
				exp, err := r.parsePlaceholder(inner)
				if err != nil {
					return nil, err
				}
				val, ok, err := r.evalPlaceholder(exp)
				if err != nil {
					return nil, err
				}
				if !ok {
					return nil, fmt.Errorf("missing %s:%s", map[refType]string{refValues: "values", refContext: "context"}[exp.rt], exp.path)
				}
				switch v := val.(type) {
				case string:
					joined := before + v
					return &joined, nil
				case int, int32, int64, float32, float64, bool:
					joined := before + fmt.Sprintf("%v", v)
					return &joined, nil
				default:
					frag, err := marshalRunesetYAMLFragment(val, indent+"  ")
					if err != nil {
						return nil, err
					}
					lines := []string{strings.TrimRight(before, " ")}
					lines = append(lines, frag...)
					joined := strings.Join(lines, "\n")
					return &joined, nil
				}
			}
		}
		// Inline replacements (values:/context: with optional filters)
		if strings.HasPrefix(inner, "values:") || strings.HasPrefix(inner, "context:") {
			exp, err := r.parsePlaceholder(inner)
			if err != nil {
				return nil, err
			}
			val, ok, err := r.evalPlaceholder(exp)
			if err != nil {
				return nil, err
			}
			if !ok {
				return nil, fmt.Errorf("missing %s:%s", map[refType]string{refValues: "values", refContext: "context"}[exp.rt], exp.path)
			}
			builder.WriteString(before)
			builder.WriteString(fmt.Sprintf("%v", val))
			restOfLine = rest
			continue
		}
		// Unsupported placeholder in this loop; just emit literal and continue scanning after it
		builder.WriteString(before)
		builder.WriteString(match)
		restOfLine = rest
	}
	result := builder.String()
	if result == "" {
		return nil, nil
	}
	return &result, nil
}

func (r *Renderer) readCached(absPath string) (string, error) {
	if v, ok := r.FileCache[absPath]; ok {
		return v, nil
	}
	b, err := os.ReadFile(absPath)
	if err != nil {
		return "", err
	}
	s := string(b)
	r.FileCache[absPath] = s
	return s, nil
}

func (r *Renderer) push(p string) { r.Stack = append(r.Stack, p) }
func (r *Renderer) pop() {
	if len(r.Stack) > 0 {
		r.Stack = r.Stack[:len(r.Stack)-1]
	}
}
func (r *Renderer) inStack(p string) bool {
	for _, s := range r.Stack {
		if s == p {
			return true
		}
	}
	return false
}

func lookupRunesetValue(values map[string]interface{}, path string) (interface{}, bool) {
	segs := strings.Split(path, ".")
	var cur any = values
	for _, s := range segs {
		m, ok := cur.(map[string]interface{})
		if !ok {
			return nil, false
		}
		cur, ok = m[s]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

func marshalRunesetYAMLFragment(v interface{}, indent string) ([]string, error) {
	b, err := yaml.Marshal(v)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	for i := range lines {
		lines[i] = indent + lines[i]
	}
	return lines, nil
}
