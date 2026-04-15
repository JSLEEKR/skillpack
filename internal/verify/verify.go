// Package verify is the CI-mode drift detector. Given a lockfile and a set
// of source files, it re-parses every declared skill, recomputes its hash,
// and compares with the lockfile.
package verify

import (
	"fmt"
	"sort"

	"github.com/JSLEEKR/skillpack/internal/exitcode"
	"github.com/JSLEEKR/skillpack/internal/hasher"
	"github.com/JSLEEKR/skillpack/internal/lockfile"
	"github.com/JSLEEKR/skillpack/internal/parser"
)

// Result is the outcome of a verify run. It always has a populated
// Findings slice (possibly empty), even on success.
//
// JSON field names are snake_case so the public --json schema matches the
// rest of the CLI (resolve, lockfile, manifest). Cycle J fix: previously
// the struct had no tags, so encoding/json fell back to PascalCase Go
// field names — inconsistent with every other JSON surface in the binary.
type Result struct {
	Drifted  []Finding `json:"drifted"`  // skills whose hash or version drifted
	Missing  []Finding `json:"missing"`  // skills in the lockfile but not on disk
	Extra    []Finding `json:"extra"`    // skills on disk but not in the lockfile
	Findings []Finding `json:"findings"` // union of all of the above (sorted)
	OK       bool      `json:"ok"`       // true if no findings of any kind
}

// Finding is a single mismatch.
type Finding struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"` // "drift" | "missing" | "extra"
	Want    string `json:"want"` // expected value (lockfile)
	Got     string `json:"got"`  // actual value (disk)
	Message string `json:"message"`
}

// Run performs the verification. The skillFiles argument is a slice of
// skill source paths discovered by the caller (e.g. via filepath.Glob from
// the workspace manifest). Pass an empty slice to detect "missing" skills
// only.
func Run(lf *lockfile.Lockfile, skillFiles []string) (*Result, error) {
	if lf == nil {
		return nil, exitcode.Wrap(exitcode.Internal, fmt.Errorf("verify: nil lockfile"))
	}
	res := &Result{}
	// Map lockfile by name for lookup.
	lockByName := make(map[string]*lockfile.Entry, len(lf.Skills))
	for i := range lf.Skills {
		lockByName[lf.Skills[i].Name] = &lf.Skills[i]
	}

	// Parse every disk file and check against lockfile.
	seen := make(map[string]bool)
	// Duplicate-name detection: when two disk files declare the same name,
	// the H2 fix bypassed the resolver so this case stopped producing a
	// clear error. We re-add a direct check here so verify flags the
	// collision as drift instead of silently overwriting `seen` and
	// producing a misleading hash mismatch.
	firstPath := make(map[string]string)
	for _, p := range skillFiles {
		s, err := parser.ParseFile(p)
		if err != nil {
			// Parse errors propagate with their original exit code.
			return nil, err
		}
		if prev, dup := firstPath[s.Name]; dup {
			res.Drifted = append(res.Drifted, Finding{
				Name:    s.Name,
				Kind:    "drift",
				Want:    prev,
				Got:     p,
				Message: fmt.Sprintf("duplicate skill name %q on disk (%s and %s)", s.Name, prev, p),
			})
			continue
		}
		firstPath[s.Name] = p
		seen[s.Name] = true
		entry, ok := lockByName[s.Name]
		if !ok {
			res.Extra = append(res.Extra, Finding{
				Name:    s.Name,
				Kind:    "extra",
				Got:     s.SourcePath,
				Message: fmt.Sprintf("skill %q on disk is not declared in lockfile", s.Name),
			})
			continue
		}
		// Recompute hash.
		actualHash := hasher.Hash(s)
		if !hasher.Equal(actualHash, entry.Hash) {
			res.Drifted = append(res.Drifted, Finding{
				Name:    s.Name,
				Kind:    "drift",
				Want:    entry.Hash,
				Got:     actualHash,
				Message: fmt.Sprintf("hash drift for skill %q", s.Name),
			})
			continue
		}
		if s.Version != entry.Version {
			res.Drifted = append(res.Drifted, Finding{
				Name:    s.Name,
				Kind:    "drift",
				Want:    entry.Version,
				Got:     s.Version,
				Message: fmt.Sprintf("version drift for skill %q: lockfile=%s disk=%s", s.Name, entry.Version, s.Version),
			})
		}
	}

	// Find missing skills.
	for name, entry := range lockByName {
		if !seen[name] {
			res.Missing = append(res.Missing, Finding{
				Name:    name,
				Kind:    "missing",
				Want:    entry.Source,
				Message: fmt.Sprintf("skill %q is in lockfile but not found on disk", name),
			})
		}
	}

	// Combined sorted findings for stable output.
	all := make([]Finding, 0, len(res.Drifted)+len(res.Missing)+len(res.Extra))
	all = append(all, res.Drifted...)
	all = append(all, res.Missing...)
	all = append(all, res.Extra...)
	sort.Slice(all, func(i, j int) bool {
		if all[i].Kind != all[j].Kind {
			return all[i].Kind < all[j].Kind
		}
		return all[i].Name < all[j].Name
	})
	res.Findings = all
	res.OK = len(all) == 0
	return res, nil
}

// ExitCode classifies a Result into a CLI exit code.
//
// Note: parse and IO errors are surfaced directly from Run by their typed
// error, so this function only handles successful runs (OK or drift).
func (r *Result) ExitCode() int {
	if r == nil {
		return exitcode.Internal
	}
	if r.OK {
		return exitcode.OK
	}
	return exitcode.Drift
}

// Summary returns a one-line human-readable summary.
func (r *Result) Summary() string {
	if r == nil || r.OK {
		return "skillpack: verify OK"
	}
	return fmt.Sprintf("skillpack: verify FAILED — drift=%d missing=%d extra=%d",
		len(r.Drifted), len(r.Missing), len(r.Extra))
}
