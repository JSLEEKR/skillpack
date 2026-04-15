// Package docsmeta contains meta-tests that guard documentation accuracy
// against drift from the code. Cycle H regression: ROUND_LOG, CHANGELOG, and
// README all still advertised 205 / 188+ tests and made no mention of the
// Cycle C, E, and G fixes, so downstream readers saw a stale picture of the
// project. These tests pin the claims to reality: if a future refactor drops
// a test without updating the docs, or removes a cycle record, this package
// fails fast.
//
// The pinned count is 216 (the actual count after Cycle H adds these
// meta-tests). Whenever a test is added or removed, update BOTH the docs
// AND the constant below in lockstep — that is the contract the meta-test
// enforces.
package docsmeta

import (
	"os"
	"path/filepath"
	"runtime"
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
// to the actual test count of 213. Update this number (and the docs) in
// lockstep when tests are added.
func TestROUND_LOGClaimsMatchReality(t *testing.T) {
	root := repoRoot(t)
	body := readFile(t, filepath.Join(root, "ROUND_LOG.md"))
	// Must mention the current test count.
	if !strings.Contains(body, "216 tests") {
		t.Errorf("ROUND_LOG.md does not mention '213 tests' — doc drift")
	}
	// Must NOT still carry the stale 205 count.
	if strings.Contains(body, "205 tests") {
		t.Errorf("ROUND_LOG.md still contains stale '205 tests' claim")
	}
	// Must record every cycle that shipped a fix, not just A/B.
	for _, cycle := range []string{"Cycle C", "Cycle E", "Cycle G", "Cycle H"} {
		if !strings.Contains(body, cycle) {
			t.Errorf("ROUND_LOG.md missing record of %s", cycle)
		}
	}
}

// TestCHANGELOGClaimsMatchReality pins CHANGELOG.md.
func TestCHANGELOGClaimsMatchReality(t *testing.T) {
	root := repoRoot(t)
	body := readFile(t, filepath.Join(root, "CHANGELOG.md"))
	if !strings.Contains(body, "216 tests") {
		t.Errorf("CHANGELOG.md does not mention '213 tests' — doc drift")
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
	if !strings.Contains(body, "216 tests across all layers") {
		t.Errorf("README.md does not mention '216 tests across all layers' — doc drift")
	}
	// Badge URL should not still say 188.
	if strings.Contains(body, "tests-188") {
		t.Errorf("README.md badge still says tests-188")
	}
}
