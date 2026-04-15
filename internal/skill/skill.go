// Package skill defines the canonical skill record shared by every format
// parser, the resolver, the hasher, and the lockfile writer.
package skill

import (
	"fmt"
	"sort"
	"strings"
)

// Format identifies the on-disk format the skill was read from.
type Format string

const (
	FormatSkillMD     Format = "skill.md"
	FormatCursorRules Format = ".cursorrules"
	FormatAgentMD     Format = "agent.md"
	FormatSkillYAML   Format = "skill.yaml"
	FormatUnknown     Format = "unknown"
)

// Valid reports whether the format is one of the supported formats.
func (f Format) Valid() bool {
	switch f {
	case FormatSkillMD, FormatCursorRules, FormatAgentMD, FormatSkillYAML:
		return true
	}
	return false
}

// Constraint is a semver constraint on another skill (by name).
type Constraint struct {
	Name string
	Expr string // "^1.2.3", "~1.2.3", ">=1.2.3", "1.2.3", "*"
}

// String renders the constraint canonically as "name expr" (space-separated).
func (c Constraint) String() string {
	return c.Name + " " + c.Expr
}

// ParseConstraint parses "name expr" into a Constraint.
// Accepts either a single space or any amount of whitespace between fields.
// The name may not be empty; the expression defaults to "*" when absent.
func ParseConstraint(s string) (Constraint, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Constraint{}, fmt.Errorf("empty constraint")
	}
	fields := strings.Fields(s)
	switch len(fields) {
	case 1:
		return Constraint{Name: fields[0], Expr: "*"}, nil
	case 2:
		return Constraint{Name: fields[0], Expr: fields[1]}, nil
	default:
		// allow "name op version" style, e.g. "base >= 1.0.0"
		name := fields[0]
		expr := strings.Join(fields[1:], "")
		return Constraint{Name: name, Expr: expr}, nil
	}
}

// Skill is the canonical record produced by every parser.
type Skill struct {
	Name        string
	Version     string // semver, normalized (no leading v)
	Description string
	Format      Format
	SourcePath  string            // file path relative to workspace root
	Body        string            // canonicalized body (LF, trailing LF)
	Frontmatter map[string]string // normalized key/value
	Requires    []Constraint
	Tools       []string
	License     string
	Author      string
	Hash        string // "sha256:..." (hex); set by hasher
}

// Validate checks the minimum-viable invariants every format must satisfy.
//
// Name rules (B2 hardening): must be non-empty, must not equal "." or "..",
// must not start with "." (reserves leading-dot for hidden/system files),
// must not contain path separators, must not contain whitespace, and must
// not carry leading/trailing whitespace. These rules are applied here so
// that a poisoned `name:` never reaches the lockfile — the bundler's
// assertSafePath is defense-in-depth, not the primary gate.
func (s *Skill) Validate() error {
	if s == nil {
		return fmt.Errorf("nil skill")
	}
	if s.Name == "" {
		return fmt.Errorf("skill name is required")
	}
	if strings.TrimSpace(s.Name) != s.Name {
		return fmt.Errorf("skill name has leading/trailing whitespace: %q", s.Name)
	}
	if s.Name == "." || s.Name == ".." {
		return fmt.Errorf("skill name %q is reserved", s.Name)
	}
	if strings.HasPrefix(s.Name, ".") {
		return fmt.Errorf("skill name may not start with '.': %q", s.Name)
	}
	if strings.Contains(s.Name, "..") {
		return fmt.Errorf("skill name may not contain '..': %q", s.Name)
	}
	if strings.ContainsAny(s.Name, " \t\r\n\\/") {
		return fmt.Errorf("skill name contains invalid characters: %q", s.Name)
	}
	if strings.TrimSpace(s.Version) == "" {
		return fmt.Errorf("skill %q: version is required", s.Name)
	}
	if !s.Format.Valid() {
		return fmt.Errorf("skill %q: invalid format %q", s.Name, s.Format)
	}
	return nil
}

// SortedFrontmatterKeys returns frontmatter keys in sorted order for
// deterministic serialization.
func (s *Skill) SortedFrontmatterKeys() []string {
	keys := make([]string, 0, len(s.Frontmatter))
	for k := range s.Frontmatter {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// SortedTools returns tools in sorted order.
func (s *Skill) SortedTools() []string {
	tools := make([]string, len(s.Tools))
	copy(tools, s.Tools)
	sort.Strings(tools)
	return tools
}

// SortedRequires returns requires constraints sorted by name, then by expr.
func (s *Skill) SortedRequires() []Constraint {
	out := make([]Constraint, len(s.Requires))
	copy(out, s.Requires)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Expr < out[j].Expr
	})
	return out
}
