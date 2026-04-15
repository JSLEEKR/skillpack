// Package docsmeta contains meta-tests that guard documentation accuracy
// against drift from the code. Cycle H regression: ROUND_LOG, CHANGELOG, and
// README all still advertised 205 / 188+ tests and made no mention of the
// Cycle C, E, and G fixes, so downstream readers saw a stale picture of the
// project. These tests pin the claims to reality: if a future refactor drops
// a test without updating the docs, or removes a cycle record, this package
// fails fast.
//
// The pinned count is 221 (the actual count after Cycle L adds one
// per-package-table-sum regression test on top of Cycle K's 220).
// Whenever a test is added or removed, update BOTH the docs AND the
// constants below in lockstep — that is the contract the meta-tests
// enforce. TestDocsmetaTestSelfConsistent adds a second layer: even the
// drift-detector's own error messages can't drift from its assertions.
package docsmeta

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// repoRoot returns the repository root by walking up from this test file.
// We avoid runtime callers so cross-OS path handling stays uniform.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// .../internal/docsmeta/docsmeta_test.go -> .../
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// TestROUND_LOGClaimsMatchReality pins the ROUND_LOG test count statement
// to the actual test count of 221. Update this number (and the docs) in
// lockstep when tests are added.
func TestROUND_LOGClaimsMatchReality(t *testing.T) {
	root := repoRoot(t)
	body := readFile(t, filepath.Join(root, "ROUND_LOG.md"))
	// Must mention the current test count.
	if !strings.Contains(body, "221 tests") {
		t.Errorf("ROUND_LOG.md does not mention '221 tests' — doc drift")
	}
	// Must NOT still carry the stale 205 count.
	if strings.Contains(body, "205 tests") {
		t.Errorf("ROUND_LOG.md still contains stale '205 tests' claim")
	}
	// Must record every cycle that shipped a fix, not just A/B.
	for _, cycle := range []string{"Cycle C", "Cycle E", "Cycle G", "Cycle H", "Cycle J", "Cycle K", "Cycle L"} {
		if !strings.Contains(body, cycle) {
			t.Errorf("ROUND_LOG.md missing record of %s", cycle)
		}
	}
}

// TestCHANGELOGClaimsMatchReality pins CHANGELOG.md.
func TestCHANGELOGClaimsMatchReality(t *testing.T) {
	root := repoRoot(t)
	body := readFile(t, filepath.Join(root, "CHANGELOG.md"))
	if !strings.Contains(body, "221 tests") {
		t.Errorf("CHANGELOG.md does not mention '221 tests' — doc drift")
	}
	if strings.Contains(body, "**205 tests**") {
		t.Errorf("CHANGELOG.md still contains stale '205 tests' claim")
	}
}

// TestREADMEClaimsMatchReality pins README.md. We check both the headline
// test-count sentence and the per-package table so the summary and the
// detail can't drift apart.
func TestREADMEClaimsMatchReality(t *testing.T) {
	root := repoRoot(t)
	body := readFile(t, filepath.Join(root, "README.md"))
	if !strings.Contains(body, "221 tests across all layers") {
		t.Errorf("README.md does not mention '221 tests across all layers' — doc drift")
	}
	// Badge URL should not still say 188.
	if strings.Contains(body, "tests-188") {
		t.Errorf("README.md badge still says tests-188")
	}
	// L1 fix: the shields.io tests badge must pin to the CURRENT count,
	// not a stale number. Cycle L found the badge still at 216 three
	// cycles after the real count moved to 220; earlier meta-tests
	// only blocklisted `tests-188` and whitelisted the headline sentence.
	// Pin the badge positively so every future test-count bump must update
	// it (or this test fails fast).
	if !strings.Contains(body, "tests-221-brightgreen") {
		t.Errorf("README.md tests badge is not 'tests-221-brightgreen' — badge drifted from actual count")
	}
}

// TestROUND_LOGPerPackageTableMatchesTotal pins the per-package test-count
// table in ROUND_LOG.md: the column values must sum to the project total
// declared in the headline. Cycle L found the table still carried
// `lockfile | 17` and `verify | 11` three cycles after Cycles J/K added
// one and two tests respectively. The headline `221 tests` was correct,
// but the table beneath it was stale — because the earlier meta-tests
// only pinned the headline string, not the table rows. This test scrapes
// every "| internal/<pkg> | LOC | N |" row and asserts the N values
// sum to 221.
func TestROUND_LOGPerPackageTableMatchesTotal(t *testing.T) {
	root := repoRoot(t)
	body := readFile(t, filepath.Join(root, "ROUND_LOG.md"))
	const wantTotal = 221
	// Match: `| `internal/<name>` | <loc> | <count> |`
	// loc may have a leading ~; count is a plain int.
	row := regexp.MustCompile(`\|\s*` + "`internal/[^`]+`" + `\s*\|\s*~?\d+\s*\|\s*(\d+)\s*\|`)
	matches := row.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		t.Fatal("ROUND_LOG.md has no internal/* rows — table format changed?")
	}
	sum := 0
	for _, m := range matches {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			t.Fatalf("parse count %q: %v", m[1], err)
		}
		sum += n
	}
	if sum != wantTotal {
		t.Errorf("ROUND_LOG.md per-package table sums to %d, want %d (headline says %d tests — per-row numbers are stale)", sum, wantTotal, wantTotal)
	}
}

// TestDocsmetaTestSelfConsistent is a meta-meta-test. The docsmeta package
// EXISTS to catch doc drift; Cycle J caught its own error messages drifting
// (`strings.Contains(body, "216 tests")` but the error said "'213 tests'").
// This test reads the docsmeta_test.go source and asserts that every
// `NNN tests` occurrence on a non-comment line names either the current
// pinned count or a still-rejected historical value.
func TestDocsmetaTestSelfConsistent(t *testing.T) {
	root := repoRoot(t)
	src := readFile(t, filepath.Join(root, "internal", "docsmeta", "docsmeta_test.go"))
	// The asserted constant; if this ever moves, update the source too.
	const currentCount = "221"
	// Any line must only reference `currentCount tests`, `currentCount tests across all layers`,
	// or the explicitly-rejected historicals below.
	rejectedHistoricals := map[string]bool{
		"205": true, // rejected by ROUND_LOG / CHANGELOG assertions
	}
	re := regexp.MustCompile(`(\d+)\s+tests`)
	lines := strings.Split(src, "\n")
	// window returns the previous, current, and next line concatenated —
	// so the "stale" keyword can live on the comment above the if, the if
	// itself, or the following t.Errorf.
	window := func(i int) string {
		var prev, next string
		if i > 0 {
			prev = lines[i-1]
		}
		if i+1 < len(lines) {
			next = lines[i+1]
		}
		return prev + "\n" + lines[i] + "\n" + next
	}
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		matches := re.FindAllStringSubmatch(line, -1)
		for _, m := range matches {
			num := m[1]
			if num == currentCount {
				continue
			}
			if rejectedHistoricals[num] {
				// Only allowed inside a "stale" check; the keyword can sit on
				// any of prev / current / next line.
				if !strings.Contains(window(i), "stale") {
					t.Errorf("docsmeta_test.go:%d references historical count %q outside a stale-check context", i+1, num)
				}
				continue
			}
			t.Errorf("docsmeta_test.go:%d has count %q tests; want %q or an allowed historical", i+1, num, currentCount)
		}
	}
}
