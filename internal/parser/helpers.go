package parser

import (
	"fmt"
	"sort"
	"strings"

	"github.com/JSLEEKR/skillpack/internal/skill"
)

// normalizeRequires converts the polymorphic `requires:` field into a sorted
// slice of constraints. Accepts:
//
//	requires: ["base ^1.0.0", "logger ~1.2.0"]      (list of strings)
//	requires: { base: "^1.0.0", logger: "~1.2.0" }  (map name->expr)
//	requires: null                                    (none)
func normalizeRequires(raw interface{}) ([]skill.Constraint, error) {
	if raw == nil {
		return nil, nil
	}
	var out []skill.Constraint
	switch v := raw.(type) {
	case []interface{}:
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("requires: list item is not a string: %T", item)
			}
			c, err := skill.ParseConstraint(s)
			if err != nil {
				return nil, fmt.Errorf("requires: %w", err)
			}
			out = append(out, c)
		}
	case map[string]interface{}:
		for name, expr := range v {
			es, ok := expr.(string)
			if !ok {
				return nil, fmt.Errorf("requires[%q]: expression is not a string: %T", name, expr)
			}
			out = append(out, skill.Constraint{Name: name, Expr: es})
		}
	case map[interface{}]interface{}:
		for name, expr := range v {
			ns, ok := name.(string)
			if !ok {
				return nil, fmt.Errorf("requires: key is not a string: %T", name)
			}
			es, ok := expr.(string)
			if !ok {
				return nil, fmt.Errorf("requires[%q]: expression is not a string: %T", ns, expr)
			}
			out = append(out, skill.Constraint{Name: ns, Expr: es})
		}
	case []string:
		for _, s := range v {
			c, err := skill.ParseConstraint(s)
			if err != nil {
				return nil, fmt.Errorf("requires: %w", err)
			}
			out = append(out, c)
		}
	default:
		return nil, fmt.Errorf("requires: unsupported type %T", raw)
	}
	// Sort for determinism.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Expr < out[j].Expr
	})
	return out, nil
}

// dedupSorted returns a sorted, de-duplicated copy of in. Empty/whitespace
// entries are dropped.
func dedupSorted(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// joinSorted is dedupSorted followed by comma-join — used to flatten list-typed
// frontmatter fields into a single canonical string for hashing.
func joinSorted(in []string) string {
	return strings.Join(dedupSorted(in), ",")
}
