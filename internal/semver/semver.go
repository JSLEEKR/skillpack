// Package semver wraps golang.org/x/mod/semver with tilde/caret support.
//
// Accepts versions with or without a "v" prefix. All comparisons normalize
// internally to the "v" prefix form; public output strips the prefix for
// clean display in lockfiles.
package semver

import (
	"fmt"
	"strings"

	xsemver "golang.org/x/mod/semver"
)

// Normalize prepends "v" if missing. Returns the empty string when the input
// is not a syntactically valid semver.
func Normalize(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	if !xsemver.IsValid(v) {
		return ""
	}
	return v
}

// Display returns the version without the "v" prefix for output.
// Returns the original string unchanged when it is not a valid semver.
func Display(v string) string {
	nv := Normalize(v)
	if nv == "" {
		return v
	}
	return strings.TrimPrefix(nv, "v")
}

// IsValid reports whether v is a syntactically valid semver (with or without v-prefix).
func IsValid(v string) bool {
	return Normalize(v) != ""
}

// Compare returns -1/0/1 for a vs b. Panics on invalid input; callers must
// validate with IsValid first.
func Compare(a, b string) int {
	na := Normalize(a)
	nb := Normalize(b)
	if na == "" || nb == "" {
		panic(fmt.Sprintf("semver.Compare: invalid input %q / %q", a, b))
	}
	return xsemver.Compare(na, nb)
}

// Major returns the major version number as a string ("1", "2", ...).
func Major(v string) string {
	nv := Normalize(v)
	if nv == "" {
		return ""
	}
	return strings.TrimPrefix(xsemver.Major(nv), "v")
}

// MajorMinor returns the "major.minor" portion (no v prefix).
func MajorMinor(v string) string {
	nv := Normalize(v)
	if nv == "" {
		return ""
	}
	return strings.TrimPrefix(xsemver.MajorMinor(nv), "v")
}

// Match reports whether the given version satisfies the constraint expression.
//
// Supported expressions:
//
//	"*"       any valid version
//	"1.2.3"   exact match
//	">=1.2.3" ">1.2.3" "<=1.2.3" "<1.2.3"
//	"^1.2.3"  caret: >=1.2.3 <2.0.0-0
//	"~1.2.3"  tilde: >=1.2.3 <1.3.0-0
//	"1.2.x" / "1.x"   x-ranges (treated like caret/minor wildcard)
//
// The empty string and "latest" are treated as "*".
func Match(version, constraint string) (bool, error) {
	version = strings.TrimSpace(version)
	constraint = strings.TrimSpace(constraint)
	if !IsValid(version) {
		return false, fmt.Errorf("semver.Match: invalid version %q", version)
	}
	if constraint == "" || constraint == "*" || constraint == "latest" || constraint == "any" {
		return true, nil
	}
	nv := Normalize(version)

	// x-ranges
	if strings.Contains(constraint, ".x") || strings.Contains(constraint, ".X") {
		return matchXRange(nv, constraint)
	}

	switch {
	case strings.HasPrefix(constraint, "^"):
		return matchCaret(nv, strings.TrimPrefix(constraint, "^"))
	case strings.HasPrefix(constraint, "~"):
		return matchTilde(nv, strings.TrimPrefix(constraint, "~"))
	case strings.HasPrefix(constraint, ">="):
		return matchOp(nv, ">=", strings.TrimPrefix(constraint, ">="))
	case strings.HasPrefix(constraint, "<="):
		return matchOp(nv, "<=", strings.TrimPrefix(constraint, "<="))
	case strings.HasPrefix(constraint, ">"):
		return matchOp(nv, ">", strings.TrimPrefix(constraint, ">"))
	case strings.HasPrefix(constraint, "<"):
		return matchOp(nv, "<", strings.TrimPrefix(constraint, "<"))
	case strings.HasPrefix(constraint, "="):
		return matchOp(nv, "=", strings.TrimPrefix(constraint, "="))
	default:
		// exact
		return matchOp(nv, "=", constraint)
	}
}

func matchOp(nv, op, raw string) (bool, error) {
	target := Normalize(raw)
	if target == "" {
		return false, fmt.Errorf("semver.Match: invalid constraint version %q", raw)
	}
	cmp := xsemver.Compare(nv, target)
	switch op {
	case "=":
		return cmp == 0, nil
	case ">":
		return cmp > 0, nil
	case ">=":
		return cmp >= 0, nil
	case "<":
		return cmp < 0, nil
	case "<=":
		return cmp <= 0, nil
	}
	return false, fmt.Errorf("semver.Match: unknown op %q", op)
}

func matchCaret(nv, raw string) (bool, error) {
	lower := Normalize(raw)
	if lower == "" {
		return false, fmt.Errorf("semver.Match: invalid caret %q", raw)
	}
	// Upper bound: next major (or next minor if major is 0).
	majorStr := strings.TrimPrefix(xsemver.Major(lower), "v")
	var upper string
	if majorStr == "0" {
		// ^0.y.z -> >=0.y.z <0.(y+1).0 for SemVer2 compatibility
		mm := strings.TrimPrefix(xsemver.MajorMinor(lower), "v")
		parts := strings.Split(mm, ".")
		if len(parts) < 2 {
			return false, fmt.Errorf("semver.Match: malformed caret lower bound %q", raw)
		}
		nextMinor, err := incInt(parts[1])
		if err != nil {
			return false, err
		}
		upper = "v" + parts[0] + "." + nextMinor + ".0"
	} else {
		nextMajor, err := incInt(majorStr)
		if err != nil {
			return false, err
		}
		upper = "v" + nextMajor + ".0.0"
	}
	if xsemver.Compare(nv, lower) < 0 {
		return false, nil
	}
	if xsemver.Compare(nv, upper) >= 0 {
		return false, nil
	}
	return true, nil
}

func matchTilde(nv, raw string) (bool, error) {
	lower := Normalize(raw)
	if lower == "" {
		return false, fmt.Errorf("semver.Match: invalid tilde %q", raw)
	}
	mm := strings.TrimPrefix(xsemver.MajorMinor(lower), "v")
	parts := strings.Split(mm, ".")
	if len(parts) < 2 {
		return false, fmt.Errorf("semver.Match: malformed tilde %q", raw)
	}
	nextMinor, err := incInt(parts[1])
	if err != nil {
		return false, err
	}
	upper := "v" + parts[0] + "." + nextMinor + ".0"
	if xsemver.Compare(nv, lower) < 0 {
		return false, nil
	}
	if xsemver.Compare(nv, upper) >= 0 {
		return false, nil
	}
	return true, nil
}

func matchXRange(nv, constraint string) (bool, error) {
	// Normalize .X to .x and split.
	constraint = strings.ToLower(constraint)
	parts := strings.Split(constraint, ".")
	// Replace trailing .x with caret-style range.
	// "1.x"   -> ^1.0.0
	// "1.2.x" -> ~1.2.0
	if len(parts) == 2 && parts[1] == "x" {
		return matchCaret(nv, parts[0]+".0.0")
	}
	if len(parts) == 3 && parts[2] == "x" {
		return matchTilde(nv, parts[0]+"."+parts[1]+".0")
	}
	return false, fmt.Errorf("semver.Match: unsupported x-range %q", constraint)
}

func incInt(s string) (string, error) {
	// Manual int parse — avoid importing strconv for such a narrow call.
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return "", fmt.Errorf("semver.Match: bad int %q", s)
		}
		n = n*10 + int(r-'0')
	}
	n++
	// Reconstruct.
	if n == 0 {
		return "0", nil
	}
	out := make([]byte, 0, 4)
	for n > 0 {
		out = append([]byte{byte('0' + n%10)}, out...)
		n /= 10
	}
	return string(out), nil
}

// BestMatch returns the highest version from the candidates that satisfies
// the constraint, or the empty string if none qualify. The candidates are
// evaluated independently of their input order.
func BestMatch(candidates []string, constraint string) (string, error) {
	best := ""
	for _, c := range candidates {
		ok, err := Match(c, constraint)
		if err != nil {
			return "", err
		}
		if !ok {
			continue
		}
		if best == "" || Compare(c, best) > 0 {
			best = c
		}
	}
	return best, nil
}
