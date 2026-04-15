// Package workspace glues the manifest, parser, and resolver into a single
// "load everything from this directory" API. CLI commands use this to avoid
// duplicating the filesystem-walk logic.
package workspace

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/JSLEEKR/skillpack/internal/exitcode"
	"github.com/JSLEEKR/skillpack/internal/manifest"
	"github.com/JSLEEKR/skillpack/internal/parser"
	"github.com/JSLEEKR/skillpack/internal/resolver"
	"github.com/JSLEEKR/skillpack/internal/skill"
)

// Loaded bundles the parsed skills and the manifest that produced them.
type Loaded struct {
	Manifest *manifest.Workspace
	Root     string
	Skills   []*skill.Skill // already resolved (topologically sorted)
	Files    []string       // absolute paths of every skill source file found
}

// Load reads the workspace manifest at <root>/skillpack.yaml, discovers every
// skill file referenced by the manifest's `skills:` globs, parses them, and
// resolves their dependency graph. Returns a Loaded with all results.
func Load(root string) (*Loaded, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, exitcode.Wrap(exitcode.IO, fmt.Errorf("workspace: abs root: %w", err))
	}
	manPath := filepath.Join(root, "skillpack.yaml")
	w, err := manifest.ReadFile(manPath)
	if err != nil {
		// Fall back to skillpack.yml.
		alt := filepath.Join(root, "skillpack.yml")
		if _, err2 := os.Stat(alt); err2 == nil {
			w, err = manifest.ReadFile(alt)
		}
	}
	if err != nil {
		return nil, err
	}
	files, err := Discover(root, w.Skills)
	if err != nil {
		return nil, err
	}
	// Parse every file.
	skills := make([]*skill.Skill, 0, len(files))
	for _, f := range files {
		s, perr := parser.ParseFile(f)
		if perr != nil {
			return nil, perr
		}
		// Make source path workspace-relative and slash-delimited.
		rel, rerr := filepath.Rel(root, f)
		if rerr == nil {
			s.SourcePath = filepath.ToSlash(rel)
		}
		skills = append(skills, s)
	}
	// Resolve (topological order + dep validation).
	resolved, err := resolver.Resolve(skills)
	if err != nil {
		return nil, err
	}
	return &Loaded{
		Manifest: w,
		Root:     root,
		Skills:   resolved,
		Files:    files,
	}, nil
}

// Discover walks the given patterns (relative to root) and returns every
// file that looks like a skill source. Supports both glob patterns
// (e.g. "skills/*") and directory paths (recursive walk).
//
// Every pattern is constrained to the workspace root: patterns that are
// absolute, contain "..", or resolve (post-symlink) outside root are rejected
// with a Parse error. This is the second line of defense after
// manifest.ValidateSkillPath, and it catches symlinks to outside the tree
// that only show up after the filesystem is consulted.
func Discover(root string, patterns []string) ([]string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, exitcode.Wrap(exitcode.IO, fmt.Errorf("workspace: abs root: %w", err))
	}
	// Resolve symlinks in the root so that comparisons below are stable
	// when the workspace itself lives under a symlink (e.g. /tmp on macOS).
	if resolved, err := filepath.EvalSymlinks(absRoot); err == nil {
		absRoot = resolved
	}
	seen := make(map[string]struct{})
	var out []string
	for _, p := range patterns {
		if err := manifest.ValidateSkillPath(p); err != nil {
			return nil, err
		}
		abs := filepath.Join(absRoot, p)
		// Ensure the raw joined path still lives inside root BEFORE touching
		// the filesystem — filepath.Join strips "..", so we re-check with Rel
		// against absRoot.
		if rel, err := filepath.Rel(absRoot, abs); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, exitcode.Wrap(exitcode.Parse, fmt.Errorf("workspace: skills path escapes workspace: %q", p))
		}
		// If it's a literal directory, walk it.
		if fi, err := os.Stat(abs); err == nil && fi.IsDir() {
			files, err := walkSkills(abs, absRoot)
			if err != nil {
				return nil, err
			}
			for _, f := range files {
				if _, dup := seen[f]; !dup {
					seen[f] = struct{}{}
					out = append(out, f)
				}
			}
			continue
		}
		// Otherwise, treat as a glob.
		matches, err := filepath.Glob(abs)
		if err != nil {
			return nil, exitcode.Wrap(exitcode.Parse, fmt.Errorf("workspace: bad glob %q: %w", p, err))
		}
		for _, m := range matches {
			fi, err := os.Stat(m)
			if err != nil {
				continue
			}
			if fi.IsDir() {
				files, err := walkSkills(m, absRoot)
				if err != nil {
					return nil, err
				}
				for _, f := range files {
					if _, dup := seen[f]; !dup {
						seen[f] = struct{}{}
						out = append(out, f)
					}
				}
				continue
			}
			if parser.DetectFormat(m) != skill.FormatUnknown {
				if err := assertInsideRoot(m, absRoot); err != nil {
					return nil, err
				}
				if _, dup := seen[m]; !dup {
					seen[m] = struct{}{}
					out = append(out, m)
				}
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

// assertInsideRoot refuses paths that, after symlink evaluation, sit outside
// the workspace root. Called on every file matched by a glob or discovered by
// the walker — the last line of defense for the B1 class of supply-chain
// probes (symlinks that point at /etc or C:\Windows\System32).
func assertInsideRoot(path, absRoot string) error {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		// If the symlink target does not exist, err here; surface as Parse
		// so callers don't treat it as a bug in the tool.
		return exitcode.Wrap(exitcode.Parse, fmt.Errorf("workspace: cannot resolve symlink %q: %w", path, err))
	}
	rel, err := filepath.Rel(absRoot, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return exitcode.Wrap(exitcode.Parse, fmt.Errorf("workspace: path escapes workspace: %q", path))
	}
	return nil
}

// walkSkills recursively walks a directory and returns every skill source
// file (any supported format). Any discovered file that (after symlink
// resolution) lives outside absRoot is rejected with a Parse error.
func walkSkills(dir, absRoot string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			// Skip hidden dirs and common VCS / tool directories.
			if strings.HasPrefix(name, ".") && name != "." && name != ".." {
				if name == ".git" || name == ".hg" || name == ".svn" || name == "node_modules" {
					return fs.SkipDir
				}
			}
			return nil
		}
		if parser.DetectFormat(path) != skill.FormatUnknown {
			if absRoot != "" {
				if err := assertInsideRoot(path, absRoot); err != nil {
					return err
				}
			}
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		return nil, exitcode.Wrap(exitcode.IO, fmt.Errorf("workspace walk %s: %w", dir, err))
	}
	return out, nil
}
