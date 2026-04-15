package lockfile

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JSLEEKR/skillpack/internal/skill"
)

func mkSkill(name, version string) *skill.Skill {
	return &skill.Skill{
		Name:       name,
		Version:    version,
		Format:     skill.FormatSkillMD,
		SourcePath: "skills/" + name + "/SKILL.md",
		Hash:       "sha256:abc123",
	}
}

func TestFromSkillsSorted(t *testing.T) {
	in := []*skill.Skill{
		mkSkill("z", "1.0.0"),
		mkSkill("a", "1.0.0"),
		mkSkill("m", "1.0.0"),
	}
	lf := FromSkills(in)
	if lf.Skills[0].Name != "a" || lf.Skills[2].Name != "z" {
		t.Errorf("not sorted: %v", lf.Skills)
	}
}

func TestFromSkillsFillsHash(t *testing.T) {
	s := &skill.Skill{
		Name:    "x",
		Version: "1.0.0",
		Format:  skill.FormatSkillMD,
		Body:    "body\n",
		// Hash intentionally empty
	}
	lf := FromSkills([]*skill.Skill{s})
	if !strings.HasPrefix(lf.Skills[0].Hash, "sha256:") {
		t.Errorf("hash not filled: %q", lf.Skills[0].Hash)
	}
}

func TestMarshalDeterministic(t *testing.T) {
	in := []*skill.Skill{
		mkSkill("z", "1.0.0"),
		mkSkill("a", "1.0.0"),
	}
	a := FromSkills(in)
	b := FromSkills(in)
	d1, _ := Marshal(a)
	d2, _ := Marshal(b)
	if !bytes.Equal(d1, d2) {
		t.Errorf("marshals differ")
	}
}

func TestMarshalTrailingNewline(t *testing.T) {
	lf := FromSkills([]*skill.Skill{mkSkill("a", "1.0.0")})
	data, _ := Marshal(lf)
	if data[len(data)-1] != '\n' {
		t.Errorf("missing trailing LF")
	}
	if bytes.Count(data, []byte("\n\n")) > 0 && bytes.HasSuffix(data, []byte("\n\n")) {
		t.Errorf("double trailing LF")
	}
}

func TestMarshalLFOnly(t *testing.T) {
	lf := FromSkills([]*skill.Skill{mkSkill("a", "1.0.0")})
	data, _ := Marshal(lf)
	if bytes.Contains(data, []byte("\r\n")) {
		t.Errorf("contains CRLF")
	}
}

func TestMarshalIndented(t *testing.T) {
	lf := FromSkills([]*skill.Skill{mkSkill("a", "1.0.0")})
	data, _ := Marshal(lf)
	if !bytes.Contains(data, []byte("  \"version\"")) {
		t.Errorf("missing indentation: %s", string(data))
	}
}

func TestUnmarshalRoundtrip(t *testing.T) {
	lf1 := FromSkills([]*skill.Skill{mkSkill("a", "1.0.0"), mkSkill("b", "2.0.0")})
	data, _ := Marshal(lf1)
	lf2, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(lf2.Skills) != 2 || lf2.Skills[0].Name != "a" {
		t.Errorf("roundtrip lost data: %+v", lf2)
	}
}

func TestUnmarshalMissingVersion(t *testing.T) {
	_, err := Unmarshal([]byte(`{"skills":[]}`))
	if err == nil {
		t.Errorf("expected error on missing version")
	}
}

func TestUnmarshalFutureVersion(t *testing.T) {
	_, err := Unmarshal([]byte(`{"version":999,"skills":[]}`))
	if err == nil {
		t.Errorf("expected error on future version")
	}
}

// G regression: a negative version used to slip through because only
// `version == 0` triggered the missing-version error, and every negative
// int passes the `> CurrentVersion` check. Reject all non-positive values
// since schema versions are always >= 1.
func TestUnmarshalNegativeVersion(t *testing.T) {
	for _, v := range []string{"-1", "-2", "-2147483648"} {
		_, err := Unmarshal([]byte(`{"version":` + v + `,"skills":[]}`))
		if err == nil {
			t.Errorf("expected error on negative version %s", v)
		}
	}
}

func TestUnmarshalBadJSON(t *testing.T) {
	_, err := Unmarshal([]byte(`{bad json`))
	if err == nil {
		t.Errorf("expected error on bad JSON")
	}
}

func TestWriteAndReadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "skillpack.lock")
	lf := FromSkills([]*skill.Skill{mkSkill("a", "1.0.0")})
	if err := WriteFile(path, lf); err != nil {
		t.Fatal(err)
	}
	lf2, err := ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if lf2.Skills[0].Name != "a" {
		t.Errorf("read lost data")
	}
}

func TestReadFileMissing(t *testing.T) {
	_, err := ReadFile("/nope/skillpack.lock")
	if err == nil {
		t.Errorf("expected error")
	}
}

func TestLookupSkill(t *testing.T) {
	lf := FromSkills([]*skill.Skill{mkSkill("a", "1.0.0"), mkSkill("b", "2.0.0")})
	if lf.LookupSkill("a") == nil {
		t.Errorf("lookup failed")
	}
	if lf.LookupSkill("nope") != nil {
		t.Errorf("lookup should miss")
	}
}

func TestNames(t *testing.T) {
	lf := FromSkills([]*skill.Skill{mkSkill("z", "1.0.0"), mkSkill("a", "1.0.0")})
	got := lf.Names()
	if got[0] != "a" || got[1] != "z" {
		t.Errorf("got %v", got)
	}
}

func TestFormatHashLine(t *testing.T) {
	e := Entry{Name: "a", Version: "1.0.0", Format: "skill.md", Hash: "sha256:abcdef1234567890"}
	line := FormatHashLine(e)
	if !strings.Contains(line, "a@1.0.0") {
		t.Errorf("missing name@version: %q", line)
	}
}

func TestWriteFileOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "skillpack.lock")
	if err := os.WriteFile(path, []byte("old content"), 0644); err != nil {
		t.Fatal(err)
	}
	lf := FromSkills([]*skill.Skill{mkSkill("a", "1.0.0")})
	if err := WriteFile(path, lf); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if bytes.Contains(data, []byte("old")) {
		t.Errorf("overwrite failed: %s", string(data))
	}
}

// Cycle K regression. A hand-edited lockfile containing two entries with
// the same skill name must be rejected at parse time. Previously Unmarshal
// accepted the file silently, the linear-scan LookupSkill returned only the
// first matching entry, and the second entry became invisible — verify could
// not detect drift in the hidden skill. The canonical lockfile produced by
// FromSkills never contains duplicates, so any duplicate on disk is
// corruption or hand-edit and must surface as a Parse error.
func TestUnmarshalRejectsDuplicateSkillNames(t *testing.T) {
	data := []byte(`{"version":1,"generated_by":"skillpack","skills":[
		{"name":"alpha","version":"1.0.0","format":"skill.md","hash":"sha256:aaa","source":"./a/SKILL.md"},
		{"name":"alpha","version":"2.0.0","format":"skill.md","hash":"sha256:bbb","source":"./b/SKILL.md"}
	]}`)
	_, err := Unmarshal(data)
	if err == nil {
		t.Fatal("expected duplicate-name error, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate skill name") {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), `"alpha"`) {
		t.Errorf("error should name the duplicate skill: %v", err)
	}
}

// Baseline: a well-formed lockfile with distinct names still parses.
func TestUnmarshalAcceptsDistinctNames(t *testing.T) {
	data := []byte(`{"version":1,"generated_by":"skillpack","skills":[
		{"name":"alpha","version":"1.0.0","format":"skill.md","hash":"sha256:aaa","source":"./a/SKILL.md"},
		{"name":"beta","version":"1.0.0","format":"skill.md","hash":"sha256:bbb","source":"./b/SKILL.md"}
	]}`)
	lf, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(lf.Skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(lf.Skills))
	}
}
