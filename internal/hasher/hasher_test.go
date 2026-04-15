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
