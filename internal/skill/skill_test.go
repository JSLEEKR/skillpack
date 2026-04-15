package skill

import (
	"reflect"
	"testing"
)

func TestFormatValid(t *testing.T) {
	tests := []struct {
		f    Format
		want bool
	}{
		{FormatSkillMD, true},
		{FormatCursorRules, true},
		{FormatAgentMD, true},
		{FormatSkillYAML, true},
		{FormatUnknown, false},
		{Format(""), false},
		{Format("nope"), false},
	}
	for _, tc := range tests {
		if got := tc.f.Valid(); got != tc.want {
			t.Errorf("Format(%q).Valid() = %v, want %v", tc.f, got, tc.want)
		}
	}
}

func TestParseConstraint(t *testing.T) {
	tests := []struct {
		in      string
		want    Constraint
		wantErr bool
	}{
		{"base ^1.2.3", Constraint{Name: "base", Expr: "^1.2.3"}, false},
		{"base", Constraint{Name: "base", Expr: "*"}, false},
		{"  base   ~1.2.3  ", Constraint{Name: "base", Expr: "~1.2.3"}, false},
		{"base >= 1.0.0", Constraint{Name: "base", Expr: ">=1.0.0"}, false},
		{"", Constraint{}, true},
		{"   ", Constraint{}, true},
	}
	for _, tc := range tests {
		got, err := ParseConstraint(tc.in)
		if (err != nil) != tc.wantErr {
			t.Errorf("ParseConstraint(%q) err=%v wantErr=%v", tc.in, err, tc.wantErr)
			continue
		}
		if !tc.wantErr && got != tc.want {
			t.Errorf("ParseConstraint(%q) = %+v, want %+v", tc.in, got, tc.want)
		}
	}
}

func TestConstraintString(t *testing.T) {
	c := Constraint{Name: "base", Expr: "^1.0.0"}
	if got := c.String(); got != "base ^1.0.0" {
		t.Errorf("String() = %q, want %q", got, "base ^1.0.0")
	}
}

func TestSkillValidate(t *testing.T) {
	tests := []struct {
		name    string
		s       *Skill
		wantErr bool
	}{
		{"happy", &Skill{Name: "x", Version: "1.0.0", Format: FormatSkillMD}, false},
		{"nil", nil, true},
		{"missing name", &Skill{Version: "1.0.0", Format: FormatSkillMD}, true},
		{"missing version", &Skill{Name: "x", Format: FormatSkillMD}, true},
		{"bad format", &Skill{Name: "x", Version: "1.0.0", Format: FormatUnknown}, true},
		{"name with space", &Skill{Name: "a b", Version: "1.0.0", Format: FormatSkillMD}, true},
		{"name with slash", &Skill{Name: "a/b", Version: "1.0.0", Format: FormatSkillMD}, true},
		{"whitespace name", &Skill{Name: "   ", Version: "1.0.0", Format: FormatSkillMD}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.s.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() err=%v wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

func TestSortedFrontmatterKeys(t *testing.T) {
	s := &Skill{Frontmatter: map[string]string{"z": "1", "a": "2", "m": "3"}}
	got := s.SortedFrontmatterKeys()
	want := []string{"a", "m", "z"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SortedFrontmatterKeys = %v, want %v", got, want)
	}
}

func TestSortedTools(t *testing.T) {
	s := &Skill{Tools: []string{"git", "bash", "curl"}}
	got := s.SortedTools()
	want := []string{"bash", "curl", "git"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SortedTools = %v, want %v", got, want)
	}
	// original must not be mutated
	if !reflect.DeepEqual(s.Tools, []string{"git", "bash", "curl"}) {
		t.Errorf("original Tools mutated: %v", s.Tools)
	}
}

func TestSortedRequires(t *testing.T) {
	s := &Skill{Requires: []Constraint{
		{Name: "z", Expr: "^1.0.0"},
		{Name: "a", Expr: "^2.0.0"},
		{Name: "a", Expr: "^1.0.0"},
	}}
	got := s.SortedRequires()
	want := []Constraint{
		{Name: "a", Expr: "^1.0.0"},
		{Name: "a", Expr: "^2.0.0"},
		{Name: "z", Expr: "^1.0.0"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SortedRequires = %v, want %v", got, want)
	}
}
