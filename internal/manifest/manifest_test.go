package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	w := Default("my-pack")
	if w.Name != "my-pack" {
		t.Errorf("name = %q", w.Name)
	}
	if w.Version != "0.1.0" {
		t.Errorf("version = %q", w.Version)
	}
	if len(w.Skills) != 1 {
		t.Errorf("skills = %v", w.Skills)
	}
}

func TestMarshalUnmarshalRoundtrip(t *testing.T) {
	w := &Workspace{Name: "x", Version: "1.0.0", Skills: []string{"./a", "./b"}}
	data, err := Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	w2, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if w2.Name != "x" || len(w2.Skills) != 2 {
		t.Errorf("got %+v", w2)
	}
}

func TestMarshalSortsSkills(t *testing.T) {
	w := &Workspace{Name: "x", Version: "1.0.0", Skills: []string{"./z", "./a"}}
	data, _ := Marshal(w)
	// The yaml output should have ./a before ./z.
	idxA := indexOf(data, "./a")
	idxZ := indexOf(data, "./z")
	if idxA < 0 || idxZ < 0 || idxA >= idxZ {
		t.Errorf("not sorted: %s", string(data))
	}
}

func TestMarshalNil(t *testing.T) {
	_, err := Marshal(nil)
	if err == nil {
		t.Errorf("expected error")
	}
}

func TestUnmarshalMissingName(t *testing.T) {
	_, err := Unmarshal([]byte("version: 1.0.0\n"))
	if err == nil {
		t.Errorf("expected error")
	}
}

func TestUnmarshalMissingVersion(t *testing.T) {
	_, err := Unmarshal([]byte("name: x\n"))
	if err == nil {
		t.Errorf("expected error")
	}
}

func TestUnmarshalBadYAML(t *testing.T) {
	_, err := Unmarshal([]byte("bad: : : :\n"))
	if err == nil {
		t.Errorf("expected error")
	}
}

func TestWriteReadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "skillpack.yaml")
	w := Default("x")
	if err := WriteFile(path, w); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "x" {
		t.Errorf("roundtrip lost name: %+v", got)
	}
}

func TestReadFileMissing(t *testing.T) {
	_, err := ReadFile(filepath.Join(t.TempDir(), "nope.yaml"))
	if err == nil {
		t.Errorf("expected error")
	}
}

func TestAddSkillPath(t *testing.T) {
	w := &Workspace{Skills: []string{"./b"}}
	if !w.AddSkillPath("./a") {
		t.Errorf("should have added")
	}
	if w.AddSkillPath("./a") {
		t.Errorf("should not add duplicate")
	}
	if w.Skills[0] != "./a" {
		t.Errorf("not sorted: %v", w.Skills)
	}
}

func TestWriteFileOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "skillpack.yaml")
	if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	w := Default("new")
	if err := WriteFile(path, w); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if indexOf(data, "new") < 0 {
		t.Errorf("not overwritten: %s", string(data))
	}
}

func indexOf(data []byte, s string) int {
	ds := string(data)
	for i := 0; i+len(s) <= len(ds); i++ {
		if ds[i:i+len(s)] == s {
			return i
		}
	}
	return -1
}
