package parser

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/JSLEEKR/skillpack/internal/exitcode"
	"github.com/JSLEEKR/skillpack/internal/semver"
	"github.com/JSLEEKR/skillpack/internal/skill"
)

// cursorFrontmatter is the Cursor IDE .cursorrules frontmatter shape.
// Cursor historically used `globs`, `alwaysApply`, etc. We support a
// skillpack-friendly extension set (name/version/requires) without breaking
// existing fields — unknown keys are preserved in the raw frontmatter map.
type cursorFrontmatter struct {
	Name        string      `yaml:"name"`
	Version     string      `yaml:"version"`
	Description string      `yaml:"description"`
	License     string      `yaml:"license"`
	Author      string      `yaml:"author"`
	Tools       []string    `yaml:"tools"`
	Requires    interface{} `yaml:"requires"`
	Globs       []string    `yaml:"globs"`
	AlwaysApply *bool       `yaml:"alwaysApply"`
}

func parseCursorRules(text string) (*skill.Skill, error) {
	fm, body, ok := splitFrontmatter(text)
	s := &skill.Skill{
		Format:      skill.FormatCursorRules,
		Frontmatter: map[string]string{},
	}
	if ok {
		var meta cursorFrontmatter
		if err := yaml.Unmarshal([]byte(fm), &meta); err != nil {
			return nil, exitcode.Wrap(exitcode.Parse, fmt.Errorf(".cursorrules: invalid YAML frontmatter: %w", err))
		}
		s.Name = meta.Name
		s.Version = semver.Display(meta.Version)
		s.Description = meta.Description
		s.License = meta.License
		s.Author = meta.Author
		s.Tools = dedupSorted(meta.Tools)
		if meta.Name != "" {
			s.Frontmatter["name"] = meta.Name
		}
		if meta.Version != "" {
			s.Frontmatter["version"] = s.Version
		}
		if meta.Description != "" {
			s.Frontmatter["description"] = meta.Description
		}
		if meta.License != "" {
			s.Frontmatter["license"] = meta.License
		}
		if meta.Author != "" {
			s.Frontmatter["author"] = meta.Author
		}
		if len(meta.Globs) > 0 {
			s.Frontmatter["globs"] = strings.Join(dedupSorted(meta.Globs), ",")
		}
		if meta.AlwaysApply != nil {
			if *meta.AlwaysApply {
				s.Frontmatter["alwaysApply"] = "true"
			} else {
				s.Frontmatter["alwaysApply"] = "false"
			}
		}
		reqs, err := normalizeRequires(meta.Requires)
		if err != nil {
			return nil, exitcode.Wrap(exitcode.Parse, fmt.Errorf(".cursorrules: %w", err))
		}
		s.Requires = reqs
		s.Body = body
	} else {
		// Legacy Cursor format: no frontmatter, entire file is the rule body.
		// We synthesize a minimal skill by hashing the file name; callers must
		// supply name/version elsewhere (or they get a validation error).
		s.Body = text
	}
	if s.Name == "" {
		return nil, exitcode.Wrap(exitcode.Parse, fmt.Errorf(".cursorrules: missing `name` field (frontmatter required)"))
	}
	if s.Version == "" {
		return nil, exitcode.Wrap(exitcode.Parse, fmt.Errorf(".cursorrules: missing `version` field"))
	}
	if err := s.Validate(); err != nil {
		return nil, exitcode.Wrap(exitcode.Parse, fmt.Errorf(".cursorrules: %w", err))
	}
	return s, nil
}
