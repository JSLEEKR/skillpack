// Package manifest reads and writes skillpack.yaml — the workspace manifest
// that tells skillpack which directories contain skill source files.
//
// A workspace manifest is the human-edited input; the lockfile is the
// machine-generated output. The two files together are skillpack's complete
// state.
package manifest

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/JSLEEKR/skillpack/internal/exitcode"
)

// Workspace is the on-disk shape of skillpack.yaml.
type Workspace struct {
	Name        string   `yaml:"name"`
	Version     string   `yaml:"version"`
	Description string   `yaml:"description,omitempty"`
	Author      string   `yaml:"author,omitempty"`
	License     string   `yaml:"license,omitempty"`
	Skills      []string `yaml:"skills"` // glob patterns or directory paths
}

// Default returns a sensible default skeleton for `skillpack init`.
func Default(name string) *Workspace {
	return &Workspace{
		Name:    name,
		Version: "0.1.0",
		Skills:  []string{"./skills"},
	}
}

// Marshal renders the workspace as canonical YAML with sorted skill paths.
// Uses a 2-space indent to match the README example (yaml.v3's default is 4,
// which would create a latent drift between documentation and reality).
func Marshal(w *Workspace) ([]byte, error) {
	if w == nil {
		return nil, fmt.Errorf("manifest: nil workspace")
	}
	cp := *w
	cp.Skills = append([]string(nil), w.Skills...)
	sort.Strings(cp.Skills)
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&cp); err != nil {
		_ = enc.Close()
		return nil, exitcode.Wrap(exitcode.Internal, fmt.Errorf("manifest marshal: %w", err))
	}
	if err := enc.Close(); err != nil {
		return nil, exitcode.Wrap(exitcode.Internal, fmt.Errorf("manifest marshal close: %w", err))
	}
	return buf.Bytes(), nil
}

// Unmarshal parses skillpack.yaml from raw bytes.
func Unmarshal(data []byte) (*Workspace, error) {
	var w Workspace
	if err := yaml.Unmarshal(data, &w); err != nil {
		return nil, exitcode.Wrap(exitcode.Parse, fmt.Errorf("manifest parse: %w", err))
	}
	if w.Name == "" {
		return nil, exitcode.Wrap(exitcode.Parse, fmt.Errorf("manifest: missing `name`"))
	}
	if w.Version == "" {
		return nil, exitcode.Wrap(exitcode.Parse, fmt.Errorf("manifest: missing `version`"))
	}
	for _, p := range w.Skills {
		if err := ValidateSkillPath(p); err != nil {
			return nil, err
		}
	}
	return &w, nil
}

// ValidateSkillPath rejects `skills:` entries that could escape the workspace
// root. A shared skillpack.yaml is untrusted input (anyone can commit one),
// so we refuse absolute paths, drive-letter paths, and any path containing
// ".." segments before the caller ever touches the filesystem.
//
// The escape check in workspace.Discover is defense-in-depth; this function
// is the first line and fails fast with an exitcode.Parse error.
func ValidateSkillPath(p string) error {
	raw := p
	p = strings.TrimSpace(p)
	if p == "" {
		return exitcode.Wrap(exitcode.Parse, fmt.Errorf("manifest: empty skills entry"))
	}
	if strings.ContainsRune(p, 0) {
		return exitcode.Wrap(exitcode.Parse, fmt.Errorf("manifest: skills entry contains NUL: %q", raw))
	}
	if filepath.IsAbs(p) {
		return exitcode.Wrap(exitcode.Parse, fmt.Errorf("manifest: absolute skills path not allowed: %q", raw))
	}
	// Reject POSIX-absolute paths even on Windows (where filepath.IsAbs
	// treats "/foo" as relative). Otherwise a Linux-authored manifest slips
	// straight to "/etc/passwd" when opened on Windows.
	if strings.HasPrefix(p, "/") || strings.HasPrefix(p, "\\") {
		return exitcode.Wrap(exitcode.Parse, fmt.Errorf("manifest: rooted skills path not allowed: %q", raw))
	}
	// Drive-letter paths ("C:\\..." or "C:/...") — explicitly reject on
	// every platform for consistency.
	if len(p) >= 2 && p[1] == ':' {
		return exitcode.Wrap(exitcode.Parse, fmt.Errorf("manifest: drive-letter skills path not allowed: %q", raw))
	}
	// Split on both slash flavors so "..\\foo" is caught on POSIX too.
	segs := strings.FieldsFunc(p, func(r rune) bool { return r == '/' || r == '\\' })
	for _, seg := range segs {
		if seg == ".." {
			return exitcode.Wrap(exitcode.Parse, fmt.Errorf("manifest: skills path escapes workspace: %q", raw))
		}
	}
	return nil
}

// ReadFile reads and validates a workspace manifest from disk.
func ReadFile(path string) (*Workspace, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, exitcode.Wrap(exitcode.IO, fmt.Errorf("manifest read %s: %w", path, err))
	}
	return Unmarshal(data)
}

// WriteFile writes a workspace manifest to disk atomically.
func WriteFile(path string, w *Workspace) error {
	data, err := Marshal(w)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return exitcode.Wrap(exitcode.IO, fmt.Errorf("manifest write %s: %w", path, err))
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(path)
		if err2 := os.Rename(tmp, path); err2 != nil {
			_ = os.Remove(tmp)
			return exitcode.Wrap(exitcode.IO, fmt.Errorf("manifest rename: %w", err2))
		}
	}
	return nil
}

// AddSkillPath appends a new path to the workspace's skills list, deduping
// and resorting. Returns true if the path was new.
func (w *Workspace) AddSkillPath(path string) bool {
	for _, p := range w.Skills {
		if p == path {
			return false
		}
	}
	w.Skills = append(w.Skills, path)
	sort.Strings(w.Skills)
	return true
}
