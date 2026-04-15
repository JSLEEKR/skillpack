// Package lockfile reads and writes skillpack.lock — the deterministic JSON
// snapshot of resolved skills used by `skillpack verify` for drift detection.
package lockfile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/JSLEEKR/skillpack/internal/exitcode"
	"github.com/JSLEEKR/skillpack/internal/hasher"
	"github.com/JSLEEKR/skillpack/internal/skill"
)

// CurrentVersion is the lockfile schema version. Bump on incompatible changes.
const CurrentVersion = 1

// Entry is the per-skill record stored in the lockfile.
type Entry struct {
	Name     string   `json:"name"`
	Version  string   `json:"version"`
	Format   string   `json:"format"`
	Hash     string   `json:"hash"`
	Source   string   `json:"source"`
	Requires []string `json:"requires,omitempty"`
}

// Lockfile is the top-level lockfile structure.
type Lockfile struct {
	Version     int     `json:"version"`
	GeneratedBy string  `json:"generated_by"`
	Skills      []Entry `json:"skills"`
}

// FromSkills builds a deterministic Lockfile from a list of resolved skills.
// The input order is ignored — entries are sorted by name then version.
func FromSkills(skills []*skill.Skill) *Lockfile {
	lf := &Lockfile{
		Version:     CurrentVersion,
		GeneratedBy: "skillpack",
		Skills:      make([]Entry, 0, len(skills)),
	}
	for _, s := range skills {
		if s == nil {
			continue
		}
		h := s.Hash
		if h == "" {
			h = hasher.Hash(s)
		}
		req := make([]string, 0, len(s.Requires))
		for _, c := range s.SortedRequires() {
			req = append(req, c.String())
		}
		lf.Skills = append(lf.Skills, Entry{
			Name:     s.Name,
			Version:  s.Version,
			Format:   string(s.Format),
			Hash:     h,
			Source:   s.SourcePath,
			Requires: req,
		})
	}
	sort.Slice(lf.Skills, func(i, j int) bool {
		if lf.Skills[i].Name != lf.Skills[j].Name {
			return lf.Skills[i].Name < lf.Skills[j].Name
		}
		return lf.Skills[i].Version < lf.Skills[j].Version
	})
	return lf
}

// Marshal returns the canonical JSON representation: sorted keys (via the
// struct tag order which is already canonical), 2-space indent, LF line
// endings, single trailing LF.
//
// We emit via encoding/json with a custom buffer to control trailing newline
// and to enforce LF on Windows (json.Encoder uses \n by default, but we
// double-check to defend against future stdlib changes).
func Marshal(lf *Lockfile) ([]byte, error) {
	if lf == nil {
		return nil, fmt.Errorf("lockfile: nil")
	}
	// Make a copy with a sorted skills slice so callers can't accidentally
	// pass an unsorted lockfile and get a non-deterministic file.
	cp := *lf
	cp.Skills = append([]Entry(nil), lf.Skills...)
	sort.Slice(cp.Skills, func(i, j int) bool {
		if cp.Skills[i].Name != cp.Skills[j].Name {
			return cp.Skills[i].Name < cp.Skills[j].Name
		}
		return cp.Skills[i].Version < cp.Skills[j].Version
	})
	// Sort each entry's Requires for safety (they should already be sorted by
	// FromSkills, but defense in depth).
	for i := range cp.Skills {
		req := append([]string(nil), cp.Skills[i].Requires...)
		sort.Strings(req)
		cp.Skills[i].Requires = req
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(&cp); err != nil {
		return nil, exitcode.Wrap(exitcode.Internal, fmt.Errorf("lockfile marshal: %w", err))
	}
	out := buf.Bytes()
	// json.Encoder adds a trailing LF; normalize to ensure exactly one.
	out = bytes.TrimRight(out, "\n")
	// Force LF (defensive; bytes from json.Encoder are LF already).
	out = bytes.ReplaceAll(out, []byte("\r\n"), []byte("\n"))
	out = append(out, '\n')
	return out, nil
}

// Unmarshal parses the canonical JSON form back into a Lockfile.
func Unmarshal(data []byte) (*Lockfile, error) {
	var lf Lockfile
	if err := json.Unmarshal(data, &lf); err != nil {
		return nil, exitcode.Wrap(exitcode.Parse, fmt.Errorf("lockfile parse: %w", err))
	}
	if lf.Version <= 0 {
		// G fix: previously only `== 0` was rejected, so a hand-edited
		// `"version": -1` slipped through. Reject every non-positive value —
		// schema versions are always >= 1.
		return nil, exitcode.Wrap(exitcode.Parse, fmt.Errorf("lockfile parse: invalid version %d", lf.Version))
	}
	if lf.Version > CurrentVersion {
		return nil, exitcode.Wrap(exitcode.Parse, fmt.Errorf("lockfile parse: unsupported version %d (max %d)", lf.Version, CurrentVersion))
	}
	return &lf, nil
}

// WriteFile marshals the lockfile and writes it atomically to path.
func WriteFile(path string, lf *Lockfile) error {
	data, err := Marshal(lf)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return exitcode.Wrap(exitcode.IO, fmt.Errorf("lockfile write %s: %w", path, err))
	}
	if err := os.Rename(tmp, path); err != nil {
		// Clean up tmp on Windows where Rename can't overwrite.
		_ = os.Remove(path)
		if err2 := os.Rename(tmp, path); err2 != nil {
			_ = os.Remove(tmp)
			return exitcode.Wrap(exitcode.IO, fmt.Errorf("lockfile rename: %w", err2))
		}
	}
	return nil
}

// ReadFile reads and unmarshals a lockfile from disk.
func ReadFile(path string) (*Lockfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, exitcode.Wrap(exitcode.IO, fmt.Errorf("lockfile read %s: %w", path, err))
	}
	return Unmarshal(data)
}

// LookupSkill returns the entry for the given name, or nil if absent.
func (lf *Lockfile) LookupSkill(name string) *Entry {
	for i := range lf.Skills {
		if lf.Skills[i].Name == name {
			return &lf.Skills[i]
		}
	}
	return nil
}

// Names returns the sorted skill names contained in the lockfile.
func (lf *Lockfile) Names() []string {
	out := make([]string, 0, len(lf.Skills))
	for _, e := range lf.Skills {
		out = append(out, e.Name)
	}
	sort.Strings(out)
	return out
}

// FormatHashLine renders one line of the human-readable lockfile summary.
// Useful for `skillpack lock --print`.
func FormatHashLine(e Entry) string {
	short := e.Hash
	if strings.HasPrefix(short, "sha256:") && len(short) > 19 {
		short = short[:19] + "..."
	}
	return fmt.Sprintf("%s@%s [%s]  %s", e.Name, e.Version, e.Format, short)
}
