package resolver

import (
	"errors"
	"testing"

	"github.com/JSLEEKR/skillpack/internal/skill"
)

func mk(name, version string, reqs ...skill.Constraint) *skill.Skill {
	return &skill.Skill{
		Name:     name,
		Version:  version,
		Format:   skill.FormatSkillMD,
		Requires: reqs,
	}
}

func names(skills []*skill.Skill) []string {
	out := make([]string, len(skills))
	for i, s := range skills {
		out[i] = s.Name
	}
	return out
}

func TestResolveEmpty(t *testing.T) {
	got, err := Resolve(nil)
	if err != nil || got != nil {
		t.Errorf("got %v err %v", got, err)
	}
}

func TestResolveNoDeps(t *testing.T) {
	in := []*skill.Skill{
		mk("c", "1.0.0"),
		mk("a", "1.0.0"),
		mk("b", "1.0.0"),
	}
	out, err := Resolve(in)
	if err != nil {
		t.Fatal(err)
	}
	got := names(out)
	want := []string{"a", "b", "c"}
	if !equal(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestResolveLinearChain(t *testing.T) {
	in := []*skill.Skill{
		mk("c", "1.0.0", skill.Constraint{Name: "b", Expr: "^1.0.0"}),
		mk("b", "1.0.0", skill.Constraint{Name: "a", Expr: "^1.0.0"}),
		mk("a", "1.0.0"),
	}
	out, _ := Resolve(in)
	got := names(out)
	want := []string{"a", "b", "c"}
	if !equal(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestResolveDiamond(t *testing.T) {
	// d depends on b and c; b and c depend on a.
	in := []*skill.Skill{
		mk("d", "1.0.0",
			skill.Constraint{Name: "b", Expr: "^1.0.0"},
			skill.Constraint{Name: "c", Expr: "^1.0.0"}),
		mk("b", "1.0.0", skill.Constraint{Name: "a", Expr: "^1.0.0"}),
		mk("c", "1.0.0", skill.Constraint{Name: "a", Expr: "^1.0.0"}),
		mk("a", "1.0.0"),
	}
	out, err := Resolve(in)
	if err != nil {
		t.Fatal(err)
	}
	got := names(out)
	// a must come first; d last; b and c in alphabetical order between.
	want := []string{"a", "b", "c", "d"}
	if !equal(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestResolveDeterministic(t *testing.T) {
	in := []*skill.Skill{
		mk("z", "1.0.0", skill.Constraint{Name: "a", Expr: "^1.0.0"}),
		mk("a", "1.0.0"),
		mk("m", "1.0.0", skill.Constraint{Name: "a", Expr: "^1.0.0"}),
	}
	first, _ := Resolve(in)
	// Run twice to confirm same order.
	second, _ := Resolve(in)
	if !equal(names(first), names(second)) {
		t.Errorf("non-deterministic: %v vs %v", names(first), names(second))
	}
}

func TestResolveMissingDep(t *testing.T) {
	in := []*skill.Skill{
		mk("a", "1.0.0", skill.Constraint{Name: "missing", Expr: "*"}),
	}
	_, err := Resolve(in)
	if err == nil {
		t.Fatal("expected error")
	}
	var rerr *Error
	if !errors.As(err, &rerr) || rerr.Kind != "missing" {
		t.Errorf("got %v", err)
	}
}

func TestResolveVersionConflict(t *testing.T) {
	in := []*skill.Skill{
		mk("a", "1.0.0", skill.Constraint{Name: "b", Expr: "^2.0.0"}),
		mk("b", "1.0.0"),
	}
	_, err := Resolve(in)
	if err == nil {
		t.Fatal("expected conflict")
	}
	var rerr *Error
	if !errors.As(err, &rerr) || rerr.Kind != "conflict" {
		t.Errorf("got %v", err)
	}
}

func TestResolveCycle(t *testing.T) {
	in := []*skill.Skill{
		mk("a", "1.0.0", skill.Constraint{Name: "b", Expr: "*"}),
		mk("b", "1.0.0", skill.Constraint{Name: "a", Expr: "*"}),
	}
	_, err := Resolve(in)
	if err == nil {
		t.Fatal("expected cycle")
	}
	var rerr *Error
	if !errors.As(err, &rerr) || rerr.Kind != "cycle" {
		t.Errorf("got %v", err)
	}
}

func TestResolveSelfCycle(t *testing.T) {
	in := []*skill.Skill{
		mk("a", "1.0.0", skill.Constraint{Name: "a", Expr: "*"}),
	}
	_, err := Resolve(in)
	if err == nil {
		t.Fatal("expected cycle")
	}
}

func TestResolveDuplicate(t *testing.T) {
	in := []*skill.Skill{
		mk("a", "1.0.0"),
		mk("a", "1.1.0"),
	}
	_, err := Resolve(in)
	if err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestResolveMultipleRoots(t *testing.T) {
	in := []*skill.Skill{
		mk("aa", "1.0.0"),
		mk("zz", "1.0.0", skill.Constraint{Name: "aa", Expr: "*"}),
		mk("mm", "1.0.0"),
	}
	out, err := Resolve(in)
	if err != nil {
		t.Fatal(err)
	}
	// Order: aa, mm, zz (zz depends on aa, mm is independent).
	want := []string{"aa", "mm", "zz"}
	if !equal(names(out), want) {
		t.Errorf("got %v want %v", names(out), want)
	}
}

func TestResolveCaretSatisfied(t *testing.T) {
	in := []*skill.Skill{
		mk("a", "1.5.0"),
		mk("b", "1.0.0", skill.Constraint{Name: "a", Expr: "^1.0.0"}),
	}
	out, err := Resolve(in)
	if err != nil {
		t.Fatal(err)
	}
	if names(out)[0] != "a" {
		t.Errorf("got %v", names(out))
	}
}

func TestErrorString(t *testing.T) {
	e := &Error{Kind: "missing", Skill: "x", Message: "y"}
	if e.Error() == "" {
		t.Errorf("Error() should not be empty")
	}
}

func TestInsertSorted(t *testing.T) {
	got := insertSorted([]string{"a", "c", "e"}, "b")
	want := []string{"a", "b", "c", "e"}
	if !equal(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
	got = insertSorted([]string{"b", "c"}, "a")
	if got[0] != "a" {
		t.Errorf("got %v", got)
	}
	got = insertSorted([]string{"a", "b"}, "z")
	if got[len(got)-1] != "z" {
		t.Errorf("got %v", got)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
