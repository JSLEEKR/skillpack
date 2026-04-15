package parser

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/JSLEEKR/skillpack/internal/exitcode"
	"github.com/JSLEEKR/skillpack/internal/semver"
	"github.com/JSLEEKR/skillpack/internal/skill"
)

// agentMDFrontmatter is the AGENT.md cross-vendor frontmatter shape.
// Mirrors the proposed AGENT.md schema circa 2026-02 (vendor-agnostic).
type agentMDFrontmatter struct {
	Name        string      `yaml:"name"`
	Version     string      `yaml:"version"`
	Description string      `yaml:"description"`
	License     string      `yaml:"license"`
	Author      string      `yaml:"author"`
	Tools       []string    `yaml:"tools"`
	Requires    interface{} `yaml:"requires"`
	Vendor      string      `yaml:"vendor"`     // "anthropic", "cursor", "openai", ...
	Models      []string    `yaml:"models"`     // ["claude-3.5", "gpt-4o"]
	Permissions []string    `yaml:"permissions"`
}

func parseAgentMD(text string) (*skill.Skill, error) {
	fm, body, ok := splitFrontmatter(text)
	if !ok {
		return nil, exitcode.Wrap(exitcode.Parse, fmt.Errorf("AGENT.md: missing frontmatter delimiter (---)"))
	}
	var meta agentMDFrontmatter
	if err := yaml.Unmarshal([]byte(fm), &meta); err != nil {
		return nil, exitcode.Wrap(exitcode.Parse, fmt.Errorf("AGENT.md: invalid YAML frontmatter: %w", err))
	}
	s := &skill.Skill{
		Name:        meta.Name,
		Version:     semver.Display(meta.Version),
		Description: meta.Description,
		Format:      skill.FormatAgentMD,
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
	if meta.Vendor != "" {
		s.Frontmatter["vendor"] = meta.Vendor
	}
	if len(meta.Models) > 0 {
		s.Frontmatter["models"] = joinSorted(meta.Models)
	}
	if len(meta.Permissions) > 0 {
		s.Frontmatter["permissions"] = joinSorted(meta.Permissions)
	}
	reqs, err := normalizeRequires(meta.Requires)
	if err != nil {
		return nil, exitcode.Wrap(exitcode.Parse, fmt.Errorf("AGENT.md: %w", err))
	}
	s.Requires = reqs
	if err := s.Validate(); err != nil {
		return nil, exitcode.Wrap(exitcode.Parse, fmt.Errorf("AGENT.md: %w", err))
	}
	return s, nil
}
