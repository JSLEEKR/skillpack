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
type Result struct {
	Drifted  []Finding // skills whose hash or version drifted
	Missing  []Finding // skills in the lockfile but not on disk
	Extra    []Finding // skills on disk but not in the lockfile
	Findings []Finding // union of all of the above (sorted)
	OK       bool      // true if no findings of any kind
}

// Finding is a single mismatch.
type Finding struct {
	Name    string
	Kind    string // "drift" | "missing" | "extra"
	Want    string // expected value (lockfile)
	Got     string // actual value (disk)
	Message string
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
	for _, p := range skillFiles {
		s, err := parser.ParseFile(p)
		if err != nil {
			// Parse errors propagate with their original exit code.
			return nil, err
		}
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
