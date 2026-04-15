// Package resolver turns a set of skills and their requires-constraints into a
// deterministic install order. It is the dependency-graph half of the tool.
//
// The resolver is pure; it operates on already-parsed skills. It does NOT
// fetch, install, or hash. Those concerns live elsewhere.
package resolver

import (
	"fmt"
	"sort"

	"github.com/JSLEEKR/skillpack/internal/exitcode"
	"github.com/JSLEEKR/skillpack/internal/semver"
	"github.com/JSLEEKR/skillpack/internal/skill"
)

// Error is the typed error returned by the resolver. Callers can use
// errors.As to unpack details.
type Error struct {
	Kind    string // "missing", "cycle", "conflict"
	Skill   string
	Message string
}

func (e *Error) Error() string {
	return fmt.Sprintf("resolver [%s] skill=%s: %s", e.Kind, e.Skill, e.Message)
}

// Resolve accepts a slice of skills and returns them in a deterministic
// topological install order (dependencies before dependents).
//
// Semantics:
//   - Missing constraint target -> Error{Kind: "missing"}
//   - Constraint target fails semver match -> Error{Kind: "conflict"}
//   - Cycle in the dependency graph -> Error{Kind: "cycle"}
//
// When multiple topological orderings are valid, we pick the one that sorts
// siblings lexicographically by name — so two machines always produce the
// same lockfile regardless of map iteration order.
func Resolve(skills []*skill.Skill) ([]*skill.Skill, error) {
	if len(skills) == 0 {
		return nil, nil
	}
	// Build index by name.
	byName := make(map[string]*skill.Skill, len(skills))
	for _, s := range skills {
		if s == nil {
			continue
		}
		if _, dup := byName[s.Name]; dup {
			return nil, exitcode.Wrap(exitcode.Parse, &Error{
				Kind: "duplicate", Skill: s.Name,
				Message: "multiple skills with the same name",
			})
		}
		byName[s.Name] = s
	}
	// Validate requires against the index.
	for _, s := range skills {
		for _, req := range s.Requires {
			dep, ok := byName[req.Name]
			if !ok {
				return nil, exitcode.Wrap(exitcode.Parse, &Error{
					Kind: "missing", Skill: s.Name,
					Message: fmt.Sprintf("requires %q which is not in the workspace", req.Name),
				})
			}
			matched, err := semver.Match(dep.Version, req.Expr)
			if err != nil {
				return nil, exitcode.Wrap(exitcode.Parse, &Error{
					Kind: "conflict", Skill: s.Name,
					Message: fmt.Sprintf("requires %q %s: %v", req.Name, req.Expr, err),
				})
			}
			if !matched {
				return nil, exitcode.Wrap(exitcode.Parse, &Error{
					Kind: "conflict", Skill: s.Name,
					Message: fmt.Sprintf("requires %q %s but found %s", req.Name, req.Expr, dep.Version),
				})
			}
		}
	}
	// Topological sort.
	order, err := topoSort(skills)
	if err != nil {
		return nil, exitcode.Wrap(exitcode.Parse, err)
	}
	// Map names back to skills.
	out := make([]*skill.Skill, 0, len(order))
	for _, name := range order {
		out = append(out, byName[name])
	}
	return out, nil
}

// topoSort uses Kahn's algorithm with a lexicographic priority queue over
// available-ready nodes to guarantee deterministic output.
func topoSort(skills []*skill.Skill) ([]string, error) {
	inDegree := make(map[string]int)
	dependents := make(map[string][]string) // dep -> [who depends on dep]
	names := make([]string, 0, len(skills))
	for _, s := range skills {
		names = append(names, s.Name)
		if _, ok := inDegree[s.Name]; !ok {
			inDegree[s.Name] = 0
		}
	}
	sort.Strings(names)
	for _, s := range skills {
		for _, r := range s.Requires {
			inDegree[s.Name]++
			dependents[r.Name] = append(dependents[r.Name], s.Name)
		}
	}
	// Sort dependents for determinism.
	for k := range dependents {
		sort.Strings(dependents[k])
	}

	// Priority queue = sorted slice of "ready" names.
	ready := make([]string, 0)
	for _, n := range names {
		if inDegree[n] == 0 {
			ready = append(ready, n)
		}
	}
	sort.Strings(ready)

	out := make([]string, 0, len(names))
	for len(ready) > 0 {
		// Pop smallest-name.
		n := ready[0]
		ready = ready[1:]
		out = append(out, n)
		for _, d := range dependents[n] {
			inDegree[d]--
			if inDegree[d] == 0 {
				// Insert into ready in sorted position.
				ready = insertSorted(ready, d)
			}
		}
	}
	if len(out) != len(names) {
		// Reconstruct the cycle for better error messages.
		remaining := make([]string, 0)
		for _, n := range names {
			if inDegree[n] > 0 {
				remaining = append(remaining, n)
			}
		}
		sort.Strings(remaining)
		return nil, &Error{
			Kind: "cycle", Skill: firstOrEmpty(remaining),
			Message: fmt.Sprintf("dependency cycle involving: %v", remaining),
		}
	}
	return out, nil
}

func insertSorted(slice []string, v string) []string {
	i := sort.SearchStrings(slice, v)
	slice = append(slice, "")
	copy(slice[i+1:], slice[i:])
	slice[i] = v
	return slice
}

func firstOrEmpty(s []string) string {
	if len(s) == 0 {
		return ""
	}
	return s[0]
}
