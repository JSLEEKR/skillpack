package verify

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/JSLEEKR/skillpack/internal/exitcode"
	"github.com/JSLEEKR/skillpack/internal/hasher"
	"github.com/JSLEEKR/skillpack/internal/lockfile"
	"github.com/JSLEEKR/skillpack/internal/parser"
	"github.com/JSLEEKR/skillpack/internal/skill"
)

const sampleSkillMD = `---
name: sample
version: 1.0.0
description: a sample
---
body
`

func writeSkill(t *testing.T, dir, content string) string {
	t.Helper()
	p := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRunClean(t *testing.T) {
	dir := t.TempDir()
	path := writeSkill(t, dir, sampleSkillMD)
	s, err := parser.ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s.Hash = hasher.Hash(s)
	lf := lockfile.FromSkills([]*skill.Skill{s})
	res, err := Run(lf, []string{path})
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Errorf("expected OK, got findings: %+v", res.Findings)
	}
	if res.ExitCode() != exitcode.OK {
		t.Errorf("exit = %d", res.ExitCode())
	}
}

func TestRunDriftHash(t *testing.T) {
	dir := t.TempDir()
	path := writeSkill(t, dir, sampleSkillMD)
	s, _ := parser.ParseFile(path)
	s.Hash = hasher.Hash(s)
	lf := lockfile.FromSkills([]*skill.Skill{s})
	// Mutate the lockfile hash to simulate drift.
	lf.Skills[0].Hash = "sha256:deadbeef"
	res, err := Run(lf, []string{path})
	if err != nil {
		t.Fatal(err)
	}
	if res.OK {
		t.Errorf("expected drift")
	}
	if len(res.Drifted) != 1 {
		t.Errorf("drift = %d", len(res.Drifted))
	}
	if res.ExitCode() != exitcode.Drift {
		t.Errorf("exit = %d", res.ExitCode())
	}
}

func TestRunDriftVersion(t *testing.T) {
	dir := t.TempDir()
	path := writeSkill(t, dir, sampleSkillMD)
	s, _ := parser.ParseFile(path)
	lf := lockfile.FromSkills([]*skill.Skill{s})
	lf.Skills[0].Version = "9.9.9"
	// Force hash equality so only version drifts.
	lf.Skills[0].Hash = hasher.Hash(s)
	res, _ := Run(lf, []string{path})
	if res.OK {
		t.Errorf("expected version drift")
	}
}

func TestRunMissing(t *testing.T) {
	// Lockfile mentions a skill that is NOT on disk.
	lf := lockfile.FromSkills([]*skill.Skill{{
		Name: "missing-skill", Version: "1.0.0", Format: skill.FormatSkillMD,
		Body: "x\n",
	}})
	res, err := Run(lf, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.OK || len(res.Missing) != 1 {
		t.Errorf("expected missing finding: %+v", res.Findings)
	}
}

func TestRunExtra(t *testing.T) {
	dir := t.TempDir()
	path := writeSkill(t, dir, sampleSkillMD)
	// Empty lockfile.
	lf := &lockfile.Lockfile{Version: 1}
	res, _ := Run(lf, []string{path})
	if res.OK || len(res.Extra) != 1 {
		t.Errorf("expected extra: %+v", res.Findings)
	}
}

func TestRunNilLockfile(t *testing.T) {
	_, err := Run(nil, nil)
	if err == nil {
		t.Errorf("expected error")
	}
}

func TestRunParseError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	_ = os.WriteFile(path, []byte("not a valid skill"), 0644)
	lf := &lockfile.Lockfile{Version: 1}
	_, err := Run(lf, []string{path})
	if err == nil {
		t.Errorf("expected parse error")
	}
}

func TestSummary(t *testing.T) {
	r := &Result{OK: true}
	if r.Summary() == "" {
		t.Errorf("empty summary")
	}
	r2 := &Result{Drifted: []Finding{{Name: "x"}}}
	if r2.Summary() == "" {
		t.Errorf("empty summary")
	}
}

func TestExitCodeNil(t *testing.T) {
	var r *Result
	if r.ExitCode() != exitcode.Internal {
		t.Errorf("nil result should be internal")
	}
}

// Eval Cycle B — duplicate-name regression. After H2 removed the resolver
// from verify's path, two disk files declaring the same name stopped
// producing a clear duplicate finding. Re-establish the contract.
func TestRunDuplicateNameOnDisk(t *testing.T) {
	dir := t.TempDir()
	pathA := filepath.Join(dir, "a")
	_ = os.MkdirAll(pathA, 0755)
	pathAFile := filepath.Join(pathA, "SKILL.md")
	_ = os.WriteFile(pathAFile, []byte("---\nname: shared\nversion: 1.0.0\n---\nbody1\n"), 0644)
	pathB := filepath.Join(dir, "b")
	_ = os.MkdirAll(pathB, 0755)
	pathBFile := filepath.Join(pathB, "SKILL.md")
	_ = os.WriteFile(pathBFile, []byte("---\nname: shared\nversion: 1.0.0\n---\nbody2\n"), 0644)
	lf := &lockfile.Lockfile{Version: 1}
	res, err := Run(lf, []string{pathAFile, pathBFile})
	if err != nil {
		t.Fatal(err)
	}
	if res.OK {
		t.Errorf("expected duplicate-name drift, got OK")
	}
	foundDup := false
	for _, f := range res.Findings {
		if f.Kind == "drift" && f.Name == "shared" {
			foundDup = true
		}
	}
	if !foundDup {
		t.Errorf("missing duplicate-name drift finding: %+v", res.Findings)
	}
}

func TestRunFindingsSorted(t *testing.T) {
	dir := t.TempDir()
	pathA := filepath.Join(dir, "a")
	_ = os.MkdirAll(pathA, 0755)
	pathAFile := filepath.Join(pathA, "SKILL.md")
	_ = os.WriteFile(pathAFile, []byte("---\nname: a\nversion: 1.0.0\n---\nbody\n"), 0644)
	pathB := filepath.Join(dir, "b")
	_ = os.MkdirAll(pathB, 0755)
	pathBFile := filepath.Join(pathB, "SKILL.md")
	_ = os.WriteFile(pathBFile, []byte("---\nname: b\nversion: 1.0.0\n---\nbody\n"), 0644)
	// Empty lockfile -> both are "extra".
	lf := &lockfile.Lockfile{Version: 1}
	res, _ := Run(lf, []string{pathBFile, pathAFile})
	if len(res.Findings) != 2 {
		t.Fatalf("got %d findings", len(res.Findings))
	}
	if res.Findings[0].Name != "a" || res.Findings[1].Name != "b" {
		t.Errorf("not sorted: %+v", res.Findings)
	}
}
