package bundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JSLEEKR/skillpack/internal/lockfile"
	"github.com/JSLEEKR/skillpack/internal/skill"
)

func mkSkill(name, version string) *skill.Skill {
	return &skill.Skill{
		Name:       name,
		Version:    version,
		Format:     skill.FormatSkillMD,
		SourcePath: "skills/" + name + "/SKILL.md",
		Body:       "# " + name + "\nbody for " + name + "\n",
	}
}

func TestBundleDeterministic(t *testing.T) {
	skills := []*skill.Skill{mkSkill("a", "1.0.0"), mkSkill("b", "2.0.0")}
	lf := lockfile.FromSkills(skills)
	b1, err := Bundle(skills, lf)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := Bundle(skills, lf)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b1, b2) {
		t.Errorf("bundles differ between runs (len %d vs %d)", len(b1), len(b2))
	}
}

func TestBundleDeterministicRegardlessOfInputOrder(t *testing.T) {
	a := []*skill.Skill{mkSkill("a", "1.0.0"), mkSkill("b", "2.0.0")}
	b := []*skill.Skill{mkSkill("b", "2.0.0"), mkSkill("a", "1.0.0")}
	lf1 := lockfile.FromSkills(a)
	lf2 := lockfile.FromSkills(b)
	r1, _ := Bundle(a, lf1)
	r2, _ := Bundle(b, lf2)
	if !bytes.Equal(r1, r2) {
		t.Errorf("input order affected output")
	}
}

func TestBundleEmptyFails(t *testing.T) {
	_, err := Bundle(nil, nil)
	if err == nil {
		t.Errorf("expected error on empty skills")
	}
}

func TestBundleRejectsInvalidSkill(t *testing.T) {
	s := &skill.Skill{Name: "", Format: skill.FormatSkillMD, Version: "1.0.0"}
	lf := &lockfile.Lockfile{Version: 1}
	_, err := Bundle([]*skill.Skill{s}, lf)
	if err == nil {
		t.Errorf("expected validation error")
	}
}

func TestBundleContainsManifest(t *testing.T) {
	skills := []*skill.Skill{mkSkill("a", "1.0.0")}
	lf := lockfile.FromSkills(skills)
	data, err := Bundle(skills, lf)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := readEntries(data)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := entries["manifest.json"]; !ok {
		t.Errorf("missing manifest.json in bundle: %v", keys(entries))
	}
}

func TestBundleContainsSkillBody(t *testing.T) {
	skills := []*skill.Skill{mkSkill("a", "1.0.0")}
	lf := lockfile.FromSkills(skills)
	data, _ := Bundle(skills, lf)
	entries, _ := readEntries(data)
	body, ok := entries["skills/a/SKILL.md"]
	if !ok {
		t.Fatalf("missing skill body: %v", keys(entries))
	}
	if !strings.Contains(string(body), "body for a") {
		t.Errorf("wrong body: %q", body)
	}
}

func TestBundleHeadersDeterministic(t *testing.T) {
	skills := []*skill.Skill{mkSkill("a", "1.0.0")}
	lf := lockfile.FromSkills(skills)
	data, _ := Bundle(skills, lf)
	gz, _ := gzip.NewReader(bytes.NewReader(data))
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if hdr.Uid != 0 || hdr.Gid != 0 {
			t.Errorf("%s: uid/gid not zero: %d/%d", hdr.Name, hdr.Uid, hdr.Gid)
		}
		if hdr.ModTime.Unix() != fixedMTime.Unix() {
			t.Errorf("%s: mtime epoch = %d, want %d", hdr.Name, hdr.ModTime.Unix(), fixedMTime.Unix())
		}
		if hdr.Mode != 0644 {
			t.Errorf("%s: mode = %o, want 0644", hdr.Name, hdr.Mode)
		}
	}
}

func TestAssertSafePath(t *testing.T) {
	safe := []string{"a.txt", "a/b.txt", "skills/x/SKILL.md"}
	for _, p := range safe {
		if err := assertSafePath(p); err != nil {
			t.Errorf("safe path rejected: %q: %v", p, err)
		}
	}
	unsafe := []string{"", "/abs/path", "a/../b", "../evil", "..", ".", "a\x00b"}
	for _, p := range unsafe {
		if err := assertSafePath(p); err == nil {
			t.Errorf("unsafe path accepted: %q", p)
		}
	}
}

func TestWriteFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.skl")
	skills := []*skill.Skill{mkSkill("a", "1.0.0")}
	lf := lockfile.FromSkills(skills)
	data, _ := Bundle(skills, lf)
	if err := WriteFile(path, data); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Size() == 0 {
		t.Errorf("empty file")
	}
}

func TestInspect(t *testing.T) {
	skills := []*skill.Skill{mkSkill("a", "1.0.0")}
	lf := lockfile.FromSkills(skills)
	data, _ := Bundle(skills, lf)
	lines, err := Inspect(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) < 2 {
		t.Errorf("expected at least 2 entries, got %v", lines)
	}
}

func TestInspectBadData(t *testing.T) {
	_, err := Inspect([]byte("not a gzip"))
	if err == nil {
		t.Errorf("expected error")
	}
}

func TestBundleMultipleFormats(t *testing.T) {
	skills := []*skill.Skill{
		{Name: "a", Version: "1.0.0", Format: skill.FormatSkillMD, Body: "a\n"},
		{Name: "b", Version: "1.0.0", Format: skill.FormatCursorRules, Body: "b\n"},
		{Name: "c", Version: "1.0.0", Format: skill.FormatAgentMD, Body: "c\n"},
		{Name: "d", Version: "1.0.0", Format: skill.FormatSkillYAML, Body: "d\n"},
	}
	lf := lockfile.FromSkills(skills)
	data, err := Bundle(skills, lf)
	if err != nil {
		t.Fatal(err)
	}
	entries, _ := readEntries(data)
	wantPaths := []string{
		"skills/a/SKILL.md",
		"skills/b/.cursorrules",
		"skills/c/AGENT.md",
		"skills/d/skill.yaml",
	}
	for _, p := range wantPaths {
		if _, ok := entries[p]; !ok {
			t.Errorf("missing %q in bundle", p)
		}
	}
}

// Eval Cycle B — B4. Inspect on a tainted bundle must refuse traversal
// names, non-regular entry types, and unreasonably large entry counts.
func TestInspectRejectsTraversalEntry(t *testing.T) {
	bad := makeTarGz(t, []tar.Header{
		{Name: "../evil", Size: 0, Mode: 0644, Typeflag: tar.TypeReg},
	})
	if _, err := Inspect(bad); err == nil {
		t.Errorf("Inspect accepted ../evil entry")
	}
}

func TestInspectRejectsAbsoluteEntry(t *testing.T) {
	bad := makeTarGz(t, []tar.Header{
		{Name: "/etc/passwd", Size: 0, Mode: 0644, Typeflag: tar.TypeReg},
	})
	if _, err := Inspect(bad); err == nil {
		t.Errorf("Inspect accepted absolute path entry")
	}
}

func TestInspectRejectsSymlinkEntry(t *testing.T) {
	bad := makeTarGz(t, []tar.Header{
		{Name: "link", Size: 0, Mode: 0644, Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"},
	})
	if _, err := Inspect(bad); err == nil {
		t.Errorf("Inspect accepted symlink entry")
	}
}

func TestInspectRejectsDriveLetterEntry(t *testing.T) {
	bad := makeTarGz(t, []tar.Header{
		{Name: `C:\Windows\System32\evil`, Size: 0, Mode: 0644, Typeflag: tar.TypeReg},
	})
	if _, err := Inspect(bad); err == nil {
		t.Errorf("Inspect accepted drive-letter entry")
	}
}

// makeTarGz builds a tar.gz with the given headers and zero-length bodies.
// Used only to produce tainted inputs for Inspect tests.
func makeTarGz(t *testing.T, hdrs []tar.Header) []byte {
	t.Helper()
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)
	for i := range hdrs {
		h := hdrs[i]
		if err := tw.WriteHeader(&h); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func readEntries(data []byte) (map[string][]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	out := map[string][]byte{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		b, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		out[hdr.Name] = b
	}
	return out, nil
}

func keys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
