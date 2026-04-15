package parser

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/JSLEEKR/skillpack/internal/exitcode"
	"github.com/JSLEEKR/skillpack/internal/semver"
	"github.com/JSLEEKR/skillpack/internal/skill"
)

// skillYAMLFrontmatter is the pure-YAML manifest shape. Unlike the markdown
// formats, this one has no body — the entire file is structured metadata.
type skillYAMLFrontmatter struct {
	Name        string      `yaml:"name"`
	Version     string      `yaml:"version"`
	Description string      `yaml:"description"`
	License     string      `yaml:"license"`
	Author      string      `yaml:"author"`
	Tools       []string    `yaml:"tools"`
	Requires    interface{} `yaml:"requires"`
	Body        string      `yaml:"body"` // optional inline body
}

func parseSkillYAML(text string) (*skill.Skill, error) {
	var meta skillYAMLFrontmatter
	if err := yaml.Unmarshal([]byte(text), &meta); err != nil {
		return nil, exitcode.Wrap(exitcode.Parse, fmt.Errorf("skill.yaml: invalid YAML: %w", err))
	}
	s := &skill.Skill{
		Name:        meta.Name,
		Version:     semver.Display(meta.Version),
		Description: meta.Description,
		Format:      skill.FormatSkillYAML,
		Body:        normalizeText(meta.Body),
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
	reqs, err := normalizeRequires(meta.Requires)
	if err != nil {
		return nil, exitcode.Wrap(exitcode.Parse, fmt.Errorf("skill.yaml: %w", err))
	}
	s.Requires = reqs
	if err := s.Validate(); err != nil {
		return nil, exitcode.Wrap(exitcode.Parse, fmt.Errorf("skill.yaml: %w", err))
	}
	return s, nil
}
