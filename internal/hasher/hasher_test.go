package hasher

import (
	"strings"
	"testing"

	"github.com/JSLEEKR/skillpack/internal/skill"
)

func sample() *skill.Skill {
	return &skill.Skill{
		Name:        "code-review",
		Version:     "1.2.0",
		Description: "Review code",
		Format:      skill.FormatSkillMD,
		Body:        "body content\n",
		License:     "MIT",
		Author:      "jslee",
		Tools:       []string{"git", "bash"},
		Requires: []skill.Constraint{
			{Name: "base", Expr: "^1.0.0"},
		},
		Frontmatter: map[string]string{
			"name":    "code-review",
			"version": "1.2.0",
			"vendor":  "anthropic",
		},
	}
}

func TestHashDeterministic(t *testing.T) {
	s := sample()
	h1 := Hash(s)
	h2 := Hash(s)
	if h1 != h2 {
		t.Errorf("hash not deterministic: %q vs %q", h1, h2)
	}
	if !strings.HasPrefix(h1, "sha256:") {
		t.Errorf("missing prefix: %q", h1)
	}
}

func TestHashFrontmatterOrderInsensitive(t *testing.T) {
	a := sample()
	b := sample()
	// Different insertion orders shouldn't change the hash.
	b.Frontmatter = map[string]string{
		"vendor":  "anthropic",
		"version": "1.2.0",
		"name":    "code-review",
	}
	if Hash(a) != Hash(b) {
		t.Errorf("frontmatter ordering changed hash")
	}
}

func TestHashToolsOrderInsensitive(t *testing.T) {
	a := sample()
	b := sample()
	b.Tools = []string{"bash", "git"}
	if Hash(a) != Hash(b) {
		t.Errorf("tools ordering changed hash")
	}
}

func TestHashRequiresOrderInsensitive(t *testing.T) {
	a := sample()
	b := sample()
	a.Requires = []skill.Constraint{
		{Name: "z", Expr: "^1.0.0"},
		{Name: "a", Expr: "^1.0.0"},
	}
	b.Requires = []skill.Constraint{
		{Name: "a", Expr: "^1.0.0"},
		{Name: "z", Expr: "^1.0.0"},
	}
	if Hash(a) != Hash(b) {
		t.Errorf("requires ordering changed hash")
	}
}

func TestHashChangesOnBody(t *testing.T) {
	a := sample()
	b := sample()
	b.Body = "different body\n"
	if Hash(a) == Hash(b) {
		t.Errorf("body change must affect hash")
	}
}

func TestHashChangesOnVersion(t *testing.T) {
	a := sample()
	b := sample()
	b.Version = "1.2.1"
	if Hash(a) == Hash(b) {
		t.Errorf("version change must affect hash")
	}
}

func TestHashChangesOnName(t *testing.T) {
	a := sample()
	b := sample()
	b.Name = "renamed"
	if Hash(a) == Hash(b) {
		t.Errorf("name change must affect hash")
	}
}

func TestHashChangesOnRequires(t *testing.T) {
	a := sample()
	b := sample()
	b.Requires = append(b.Requires, skill.Constraint{Name: "extra", Expr: "*"})
	if Hash(a) == Hash(b) {
		t.Errorf("requires change must affect hash")
	}
}

func TestEqual(t *testing.T) {
	if !Equal("sha256:ABC", "sha256:abc") {
		t.Errorf("Equal should be case-insensitive")
	}
	if !Equal("  sha256:abc  ", "sha256:abc") {
		t.Errorf("Equal should trim whitespace")
	}
	if Equal("sha256:abc", "sha256:def") {
		t.Errorf("different hashes should not be equal")
	}
}

func TestHashBytes(t *testing.T) {
	h := HashBytes([]byte("hello"))
	if !strings.HasPrefix(h, "sha256:") {
		t.Errorf("missing prefix")
	}
	if h != HashBytes([]byte("hello")) {
		t.Errorf("HashBytes not deterministic")
	}
}

func TestHashNilSafe(t *testing.T) {
	if got := CanonicalBytes(nil); got != nil {
		t.Errorf("nil should return nil bytes, got %v", got)
	}
}

func TestCanonicalBytesContainsBody(t *testing.T) {
	s := sample()
	bytes := string(CanonicalBytes(s))
	if !strings.Contains(bytes, "---body---\n") {
		t.Errorf("missing body marker")
	}
	if !strings.Contains(bytes, "body content\n") {
		t.Errorf("missing body text")
	}
}

// G1 regression: the old canonical form comma-joined `tools`, so
// ["a,b","c"] and ["a","b,c"] collapsed to the same "a,b,c" preimage and
// produced identical sha256 fingerprints. Content-addressed integrity is
// broken if such pairs collide, so the canonical form now emits one quoted
// entry per line.
func TestHashToolsDistinctAcrossCommaAmbiguity(t *testing.T) {
	a := sample()
	b := sample()
	a.Tools = []string{"a,b", "c"}
	b.Tools = []string{"a", "b,c"}
	if Hash(a) == Hash(b) {
		t.Errorf("tools collision: ['a,b','c'] and ['a','b,c'] must not share hash")
	}
}

// G2 regression: the old canonical form replaced \n in values with a single
// space, so description "a\nb" and "a b" produced identical preimages.
func TestHashDescriptionDistinctAcrossNewlineSpace(t *testing.T) {
	a := sample()
	b := sample()
	a.Description = "a\nb"
	b.Description = "a b"
	if Hash(a) == Hash(b) {
		t.Errorf("description collision: \"a\\nb\" and \"a b\" must not share hash")
	}
}

// G3 regression: the old canonical form used `=` as the key/value separator
// in frontmatter lines, so `{"k": "v=v"}` and `{"k=v": "v"}` produced
// identical preimages.
func TestHashFrontmatterDistinctAcrossEqualSign(t *testing.T) {
	a := sample()
	b := sample()
	a.Frontmatter = map[string]string{"vendor": "anthropic", "k": "v=v"}
	b.Frontmatter = map[string]string{"vendor": "anthropic", "k=v": "v"}
	if Hash(a) == Hash(b) {
		t.Errorf("frontmatter collision: k/v=v vs k=v/v must not share hash")
	}
}

// G regression: frontmatter value containing a newline must not collide
// with a value where the newline is replaced by text that looks like a
// new canonical line. This is the frontmatter analog of the description
// newline collision above.
func TestHashFrontmatterDistinctAcrossNewlineInjection(t *testing.T) {
	a := sample()
	b := sample()
	a.Frontmatter = map[string]string{"k": "v\nvendor=\"anthropic\""}
	b.Frontmatter = map[string]string{"k": "v vendor=\"anthropic\""}
	if Hash(a) == Hash(b) {
		t.Errorf("frontmatter newline-injection collision")
	}
}

// G regression: requires lists used `|` as separator. A malicious expr
// containing `|` could otherwise forge the hash of a different require set.
func TestHashRequiresDistinctAcrossPipeAmbiguity(t *testing.T) {
	a := sample()
	b := sample()
	a.Requires = []skill.Constraint{{Name: "p", Expr: "1.0|2.0"}, {Name: "q", Expr: "*"}}
	b.Requires = []skill.Constraint{{Name: "p", Expr: "1.0"}, {Name: "2.0 q", Expr: "*"}}
	if Hash(a) == Hash(b) {
		t.Errorf("requires collision across pipe ambiguity")
	}
}
