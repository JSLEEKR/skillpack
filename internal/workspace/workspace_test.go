package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// skillpack.yaml
	manData := `name: test-pack
version: 1.0.0
skills:
  - ./skills
`
	if err := os.WriteFile(filepath.Join(dir, "skillpack.yaml"), []byte(manData), 0644); err != nil {
		t.Fatal(err)
	}
	// skills/a/SKILL.md
	_ = os.MkdirAll(filepath.Join(dir, "skills", "a"), 0755)
	_ = os.WriteFile(filepath.Join(dir, "skills", "a", "SKILL.md"),
		[]byte("---\nname: a\nversion: 1.0.0\n---\nbody a\n"), 0644)
	// skills/b/SKILL.md depends on a
	_ = os.MkdirAll(filepath.Join(dir, "skills", "b"), 0755)
	_ = os.WriteFile(filepath.Join(dir, "skills", "b", "SKILL.md"),
		[]byte("---\nname: b\nversion: 1.0.0\nrequires:\n  - a ^1.0.0\n---\nbody b\n"), 0644)
	return dir
}

func TestLoadHappy(t *testing.T) {
	dir := setupTestWorkspace(t)
	loaded, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Skills) != 2 {
		t.Errorf("got %d skills, want 2", len(loaded.Skills))
	}
	// `a` must come before `b` (dep order).
	if loaded.Skills[0].Name != "a" || loaded.Skills[1].Name != "b" {
		t.Errorf("wrong order: %v, %v", loaded.Skills[0].Name, loaded.Skills[1].Name)
	}
	// Source paths must be workspace-relative and slash-delimited.
	for _, s := range loaded.Skills {
		if filepath.IsAbs(s.SourcePath) {
			t.Errorf("SourcePath should be relative: %q", s.SourcePath)
		}
	}
}

func TestLoadMissingManifest(t *testing.T) {
	dir := t.TempDir()
	_, err := Load(dir)
	if err == nil {
		t.Errorf("expected error on missing manifest")
	}
}

func TestLoadMissingDep(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "skillpack.yaml"),
		[]byte("name: x\nversion: 1.0.0\nskills: [./skills]\n"), 0644)
	_ = os.MkdirAll(filepath.Join(dir, "skills", "a"), 0755)
	_ = os.WriteFile(filepath.Join(dir, "skills", "a", "SKILL.md"),
		[]byte("---\nname: a\nversion: 1.0.0\nrequires:\n  - missing ^1.0.0\n---\nbody\n"), 0644)
	_, err := Load(dir)
	if err == nil {
		t.Errorf("expected resolver error")
	}
}

func TestDiscoverRecursive(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "a", "b", "c")
	_ = os.MkdirAll(nested, 0755)
	_ = os.WriteFile(filepath.Join(nested, "SKILL.md"), []byte("x"), 0644)
	files, err := Discover(dir, []string{"."})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Errorf("got %d, want 1: %v", len(files), files)
	}
}

func TestDiscoverSkipsGitDir(t *testing.T) {
	dir := t.TempDir()
	// .git dir should be skipped
	git := filepath.Join(dir, ".git")
	_ = os.MkdirAll(git, 0755)
	_ = os.WriteFile(filepath.Join(git, "SKILL.md"), []byte("not a real skill"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: x\nversion: 1.0.0\n---\nbody\n"), 0644)
	files, err := Discover(dir, []string{"."})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Errorf("got %d files, want 1 (git should be skipped): %v", len(files), files)
	}
}

func TestDiscoverBadGlob(t *testing.T) {
	_, err := Discover(t.TempDir(), []string{"["}) // malformed glob
	if err == nil {
		t.Errorf("expected glob error")
	}
}

func TestDiscoverDedup(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("x"), 0644)
	// Two patterns that both match the same file.
	files, err := Discover(dir, []string{".", "./SKILL.md"})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Errorf("not deduped: %v", files)
	}
}

func TestDiscoverUnknownFileIgnored(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "random.txt"), []byte("x"), 0644)
	files, _ := Discover(dir, []string{"."})
	if len(files) != 0 {
		t.Errorf("unknown file picked up: %v", files)
	}
}
