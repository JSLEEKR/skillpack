package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JSLEEKR/skillpack/internal/skill"
)

func TestDetectFormat(t *testing.T) {
	tests := map[string]skill.Format{
		"SKILL.md":                  skill.FormatSkillMD,
		"skill.md":                  skill.FormatSkillMD,
		"path/to/SKILL.md":          skill.FormatSkillMD,
		"code-review.SKILL.md":      skill.FormatSkillMD,
		".cursorrules":              skill.FormatCursorRules,
		"foo/.cursorrules":          skill.FormatCursorRules,
		"AGENT.md":                  skill.FormatAgentMD,
		"agent.md":                  skill.FormatAgentMD,
		"my-bot.AGENT.md":           skill.FormatAgentMD,
		"skill.yaml":                skill.FormatSkillYAML,
		"skill.yml":                 skill.FormatSkillYAML,
		"random.txt":                skill.FormatUnknown,
		"foo.md":                    skill.FormatUnknown,
	}
	for name, want := range tests {
		if got := DetectFormat(name); got != want {
			t.Errorf("DetectFormat(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestNormalizeText(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"a\r\nb\r\n", "a\nb\n"},
		{"a\rb\r", "a\nb\n"},
		{"\ufeffhello\n", "hello\n"},
		{"abc", "abc\n"},
		{"abc\n\n\n", "abc\n"},
		{"", ""},
		{"   \n\n", ""},
	}
	for _, tc := range tests {
		if got := normalizeText(tc.in); got != tc.want {
			t.Errorf("normalizeText(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSplitFrontmatter(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		wantFm    string
		wantBody  string
		wantOK    bool
	}{
		{"happy", "---\nname: x\n---\nbody\n", "name: x", "body\n", true},
		{"empty fm", "---\n\n---\nbody\n", "", "body\n", true},
		{"no fm", "no frontmatter here\n", "", "no frontmatter here\n", false},
		{"fm at eof", "---\nname: x\n---", "name: x", "", true},
		{"only opening", "---\nname: x\n", "", "---\nname: x\n", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fm, body, ok := splitFrontmatter(tc.in)
			if fm != tc.wantFm || body != tc.wantBody || ok != tc.wantOK {
				t.Errorf("got fm=%q body=%q ok=%v, want fm=%q body=%q ok=%v",
					fm, body, ok, tc.wantFm, tc.wantBody, tc.wantOK)
			}
		})
	}
}

func TestParseSkillMDHappy(t *testing.T) {
	src := `---
name: code-review
version: 1.2.0
description: Review code for issues
license: MIT
author: jslee
tools:
  - git
  - bash
requires:
  - base ^1.0.0
  - logger ~1.2.0
---
# Body

Look at the diff.
`
	s, err := ParseBytes(skill.FormatSkillMD, []byte(src))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s.Name != "code-review" || s.Version != "1.2.0" || s.License != "MIT" {
		t.Errorf("metadata mismatch: %+v", s)
	}
	if len(s.Requires) != 2 {
		t.Errorf("requires len = %d, want 2", len(s.Requires))
	}
	// Sorted alphabetically.
	if s.Requires[0].Name != "base" || s.Requires[1].Name != "logger" {
		t.Errorf("requires not sorted: %+v", s.Requires)
	}
	if !strings.Contains(s.Body, "Look at the diff") {
		t.Errorf("body lost: %q", s.Body)
	}
}

func TestParseSkillMDMissingFrontmatter(t *testing.T) {
	_, err := ParseBytes(skill.FormatSkillMD, []byte("just body\n"))
	if err == nil {
		t.Errorf("expected error on missing frontmatter")
	}
}

func TestParseSkillMDBadYAML(t *testing.T) {
	src := "---\nname: x\nversion: : :\n---\nbody\n"
	_, err := ParseBytes(skill.FormatSkillMD, []byte(src))
	if err == nil {
		t.Errorf("expected YAML error")
	}
}

func TestParseSkillMDMissingName(t *testing.T) {
	src := "---\nversion: 1.0.0\n---\nbody\n"
	_, err := ParseBytes(skill.FormatSkillMD, []byte(src))
	if err == nil {
		t.Errorf("expected validate error")
	}
}

func TestParseSkillMDMissingVersion(t *testing.T) {
	src := "---\nname: x\n---\nbody\n"
	_, err := ParseBytes(skill.FormatSkillMD, []byte(src))
	if err == nil {
		t.Errorf("expected validate error")
	}
}

func TestParseSkillMDCRLF(t *testing.T) {
	src := "---\r\nname: x\r\nversion: 1.0.0\r\n---\r\nbody\r\n"
	s, err := ParseBytes(skill.FormatSkillMD, []byte(src))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if strings.Contains(s.Body, "\r") {
		t.Errorf("body still contains CR: %q", s.Body)
	}
}

func TestParseSkillMDBOM(t *testing.T) {
	src := "\ufeff---\nname: x\nversion: 1.0.0\n---\nbody\n"
	if _, err := ParseBytes(skill.FormatSkillMD, []byte(src)); err != nil {
		t.Errorf("BOM should be tolerated: %v", err)
	}
}

func TestParseSkillMDVPrefixVersion(t *testing.T) {
	src := "---\nname: x\nversion: v1.2.3\n---\nbody\n"
	s, err := ParseBytes(skill.FormatSkillMD, []byte(src))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s.Version != "1.2.3" {
		t.Errorf("v-prefix not stripped: %q", s.Version)
	}
}

func TestParseSkillMDRequiresMap(t *testing.T) {
	src := `---
name: x
version: 1.0.0
requires:
  base: ^1.0.0
  logger: ~1.2.0
---
body
`
	s, err := ParseBytes(skill.FormatSkillMD, []byte(src))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(s.Requires) != 2 {
		t.Errorf("got %d requires, want 2", len(s.Requires))
	}
}

func TestParseCursorRulesHappy(t *testing.T) {
	src := `---
name: cursor-style
version: 0.3.0
description: Code style rules
globs:
  - "**/*.ts"
  - "**/*.tsx"
alwaysApply: true
---
Always use tabs.
`
	s, err := ParseBytes(skill.FormatCursorRules, []byte(src))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s.Frontmatter["globs"] != "**/*.ts,**/*.tsx" {
		t.Errorf("globs frontmatter = %q", s.Frontmatter["globs"])
	}
	if s.Frontmatter["alwaysApply"] != "true" {
		t.Errorf("alwaysApply = %q", s.Frontmatter["alwaysApply"])
	}
}

func TestParseCursorRulesNoFrontmatter(t *testing.T) {
	_, err := ParseBytes(skill.FormatCursorRules, []byte("just rules\n"))
	if err == nil {
		t.Errorf("expected error on missing frontmatter (no name)")
	}
}

func TestParseAgentMDHappy(t *testing.T) {
	src := `---
name: bot
version: 2.0.0
vendor: anthropic
models:
  - claude-3.5-sonnet
permissions:
  - filesystem
  - network
---
Body of the agent.
`
	s, err := ParseBytes(skill.FormatAgentMD, []byte(src))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s.Frontmatter["vendor"] != "anthropic" {
		t.Errorf("vendor = %q", s.Frontmatter["vendor"])
	}
	if s.Frontmatter["models"] != "claude-3.5-sonnet" {
		t.Errorf("models = %q", s.Frontmatter["models"])
	}
	if s.Frontmatter["permissions"] != "filesystem,network" {
		t.Errorf("permissions = %q", s.Frontmatter["permissions"])
	}
}

func TestParseAgentMDMissingFrontmatter(t *testing.T) {
	_, err := ParseBytes(skill.FormatAgentMD, []byte("body only\n"))
	if err == nil {
		t.Errorf("expected error")
	}
}

func TestParseSkillYAMLHappy(t *testing.T) {
	src := `name: pure-yaml
version: 1.0.0
description: A pure YAML skill
tools:
  - bash
  - git
body: |
  This is the body.
`
	s, err := ParseBytes(skill.FormatSkillYAML, []byte(src))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s.Name != "pure-yaml" {
		t.Errorf("name = %q", s.Name)
	}
	if !strings.Contains(s.Body, "This is the body") {
		t.Errorf("body = %q", s.Body)
	}
	if len(s.Tools) != 2 {
		t.Errorf("tools = %v", s.Tools)
	}
}

func TestParseSkillYAMLMissingName(t *testing.T) {
	_, err := ParseBytes(skill.FormatSkillYAML, []byte("version: 1.0.0\n"))
	if err == nil {
		t.Errorf("expected error")
	}
}

func TestParseFileEndToEnd(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	src := "---\nname: e2e\nversion: 1.0.0\n---\nbody\n"
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	s, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if s.Name != "e2e" || s.Format != skill.FormatSkillMD {
		t.Errorf("got %+v", s)
	}
	if !strings.HasSuffix(s.SourcePath, "SKILL.md") {
		t.Errorf("SourcePath = %q", s.SourcePath)
	}
}

func TestParseFileUnknownFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "random.txt")
	if err := os.WriteFile(path, []byte("hi"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := ParseFile(path)
	if err == nil {
		t.Errorf("expected error")
	}
}

func TestParseFileMissing(t *testing.T) {
	_, err := ParseFile(filepath.Join(t.TempDir(), "nope", "SKILL.md"))
	if err == nil {
		t.Errorf("expected IO error")
	}
}

func TestNormalizeRequiresList(t *testing.T) {
	got, err := normalizeRequires([]interface{}{"a ^1.0.0", "b ~2.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "a" || got[1].Name != "b" {
		t.Errorf("got %v", got)
	}
}

func TestNormalizeRequiresMap(t *testing.T) {
	got, err := normalizeRequires(map[string]interface{}{"z": "^1.0.0", "a": "^2.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "a" || got[1].Name != "z" {
		t.Errorf("not sorted: %v", got)
	}
}

func TestNormalizeRequiresNil(t *testing.T) {
	got, err := normalizeRequires(nil)
	if err != nil || got != nil {
		t.Errorf("got %v err %v", got, err)
	}
}

func TestNormalizeRequiresBadType(t *testing.T) {
	_, err := normalizeRequires(42)
	if err == nil {
		t.Errorf("expected error")
	}
}

func TestDedupSorted(t *testing.T) {
	got := dedupSorted([]string{"b", "a", "b", "  ", "c", " a "})
	want := []string{"a", "b", "c"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestJoinSorted(t *testing.T) {
	if got := joinSorted([]string{"b", "a", "c"}); got != "a,b,c" {
		t.Errorf("joinSorted = %q", got)
	}
}
