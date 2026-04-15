package parser

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/JSLEEKR/skillpack/internal/exitcode"
	"github.com/JSLEEKR/skillpack/internal/semver"
	"github.com/JSLEEKR/skillpack/internal/skill"
)

// skillMDFrontmatter mirrors the Anthropic SKILL.md frontmatter shape.
// Fields are intentionally string-or-list permissive; we normalize after parse.
type skillMDFrontmatter struct {
	Name        string      `yaml:"name"`
	Version     string      `yaml:"version"`
	Description string      `yaml:"description"`
	License     string      `yaml:"license"`
	Author      string      `yaml:"author"`
	Tools       []string    `yaml:"tools"`
	Requires    interface{} `yaml:"requires"` // []string OR map[name]constraint
	Tags        []string    `yaml:"tags"`
}

func parseSkillMD(text string) (*skill.Skill, error) {
	fm, body, ok := splitFrontmatter(text)
	if !ok {
		return nil, exitcode.Wrap(exitcode.Parse, fmt.Errorf("SKILL.md: missing frontmatter delimiter (---)"))
	}
	var meta skillMDFrontmatter
	if err := yaml.Unmarshal([]byte(fm), &meta); err != nil {
		return nil, exitcode.Wrap(exitcode.Parse, fmt.Errorf("SKILL.md: invalid YAML frontmatter: %w", err))
	}
	s := &skill.Skill{
		Name:        meta.Name,
		Version:     semver.Display(meta.Version),
		Description: meta.Description,
		Format:      skill.FormatSkillMD,
		Body:        body,
		Frontmatter: map[string]string{},
		Tools:       dedupSorted(meta.Tools),
		License:     meta.License,
		Author:      meta.Author,
	}
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
	if reqs, err := normalizeRequires(meta.Requires); err == nil {
		s.Requires = reqs
	} else {
		return nil, exitcode.Wrap(exitcode.Parse, fmt.Errorf("SKILL.md: %w", err))
	}
	if err := s.Validate(); err != nil {
		return nil, exitcode.Wrap(exitcode.Parse, fmt.Errorf("SKILL.md: %w", err))
	}
	return s, nil
}
