// Package parser reads skill source files in any of the four supported
// formats and normalizes them into a canonical skill.Skill record.
//
// The four formats:
//
//	SKILL.md       Anthropic-style markdown with YAML frontmatter
//	.cursorrules   Cursor IDE rule file (frontmatter + rules body)
//	AGENT.md       cross-vendor AGENT.md with YAML frontmatter
//	skill.yaml     pure YAML manifest (no body)
//
// The canonical record drops platform-specific quirks so downstream hashing
// is stable across machines and line endings.
package parser

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/JSLEEKR/skillpack/internal/exitcode"
	"github.com/JSLEEKR/skillpack/internal/skill"
)

// ErrUnknownFormat is returned when the filename does not match any supported format.
var ErrUnknownFormat = errors.New("parser: unknown skill format")

// DetectFormat inspects the base filename to decide which parser to dispatch.
// Matching is case-insensitive and robust to absolute/relative paths.
func DetectFormat(path string) skill.Format {
	base := strings.ToLower(filepath.Base(path))
	switch base {
	case "skill.md":
		return skill.FormatSkillMD
	case ".cursorrules":
		return skill.FormatCursorRules
	case "agent.md":
		return skill.FormatAgentMD
	case "skill.yaml", "skill.yml":
		return skill.FormatSkillYAML
	}
	// Allow `NAME.SKILL.md` style (e.g. `code-review.SKILL.md`).
	if strings.HasSuffix(base, ".skill.md") {
		return skill.FormatSkillMD
	}
	if strings.HasSuffix(base, ".agent.md") {
		return skill.FormatAgentMD
	}
	return skill.FormatUnknown
}

// ParseFile dispatches to the appropriate format parser based on the file
// name and returns a canonical Skill record with the source path set.
func ParseFile(path string) (*skill.Skill, error) {
	format := DetectFormat(path)
	if format == skill.FormatUnknown {
		return nil, exitcode.Wrap(exitcode.Parse, fmt.Errorf("%w: %s", ErrUnknownFormat, path))
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, exitcode.Wrap(exitcode.IO, fmt.Errorf("parser: open %s: %w", path, err))
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, exitcode.Wrap(exitcode.IO, fmt.Errorf("parser: read %s: %w", path, err))
	}
	s, err := ParseBytes(format, data)
	if err != nil {
		return nil, err
	}
	s.SourcePath = filepath.ToSlash(path)
	return s, nil
}

// ParseBytes parses the given format from raw bytes without touching the filesystem.
// Useful for unit tests and for parsing content piped from stdin.
func ParseBytes(format skill.Format, data []byte) (*skill.Skill, error) {
	// Always normalize to LF for downstream parsers.
	text := normalizeText(string(data))
	switch format {
	case skill.FormatSkillMD:
		return parseSkillMD(text)
	case skill.FormatCursorRules:
		return parseCursorRules(text)
	case skill.FormatAgentMD:
		return parseAgentMD(text)
	case skill.FormatSkillYAML:
		return parseSkillYAML(text)
	}
	return nil, exitcode.Wrap(exitcode.Parse, fmt.Errorf("%w: %s", ErrUnknownFormat, format))
}

// normalizeText strips UTF-8 BOM, converts CRLF to LF, and ensures a single
// trailing newline. This output is what hashing operates on.
func normalizeText(s string) string {
	// Strip BOM.
	s = strings.TrimPrefix(s, "\ufeff")
	// CRLF -> LF.
	s = strings.ReplaceAll(s, "\r\n", "\n")
	// Lone CR -> LF (classic Mac format).
	s = strings.ReplaceAll(s, "\r", "\n")
	// Trim trailing whitespace / newlines; add exactly one trailing LF.
	s = strings.TrimRight(s, "\n \t")
	if s != "" {
		s += "\n"
	}
	return s
}

// splitFrontmatter separates a YAML frontmatter block (delimited by "---") from
// the body. Returns (fmBody, bodyRest, ok). When the input has no frontmatter,
// ok=false and bodyRest=original content.
//
// The opening "---" must be the very first line (after normalizeText). The
// closing "---" must also be on a line by itself.
func splitFrontmatter(s string) (string, string, bool) {
	if !strings.HasPrefix(s, "---\n") && s != "---" {
		return "", s, false
	}
	rest := strings.TrimPrefix(s, "---\n")
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		// Also tolerate a frontmatter that ends at EOF with "---\n".
		if strings.HasSuffix(rest, "\n---") {
			fm := strings.TrimSuffix(rest, "\n---")
			return fm, "", true
		}
		return "", s, false
	}
	fm := rest[:end]
	body := rest[end+len("\n---\n"):]
	return fm, body, true
}
