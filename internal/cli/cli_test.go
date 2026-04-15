package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JSLEEKR/skillpack/internal/exitcode"
)

func runCLI(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Execute(&stdout, &stderr, args)
	return stdout.String(), stderr.String(), code
}

func setupCLIWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	manData := "name: test\nversion: 1.0.0\nskills:\n  - ./skills\n"
	_ = os.WriteFile(filepath.Join(dir, "skillpack.yaml"), []byte(manData), 0644)
	_ = os.MkdirAll(filepath.Join(dir, "skills", "a"), 0755)
	_ = os.WriteFile(filepath.Join(dir, "skills", "a", "SKILL.md"),
		[]byte("---\nname: a\nversion: 1.0.0\n---\nbody a\n"), 0644)
	_ = os.MkdirAll(filepath.Join(dir, "skills", "b"), 0755)
	_ = os.WriteFile(filepath.Join(dir, "skills", "b", "SKILL.md"),
		[]byte("---\nname: b\nversion: 1.0.0\nrequires:\n  - a ^1.0.0\n---\nbody b\n"), 0644)
	return dir
}

func TestCLIInit(t *testing.T) {
	dir := t.TempDir()
	stdout, _, code := runCLI(t, "init", "--root", dir)
	if code != exitcode.OK {
		t.Errorf("init exit = %d", code)
	}
	if !strings.Contains(stdout, "created") {
		t.Errorf("missing output: %q", stdout)
	}
	if _, err := os.Stat(filepath.Join(dir, "skillpack.yaml")); err != nil {
		t.Errorf("manifest not written")
	}
}

func TestCLIInitNoOverwrite(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "skillpack.yaml"), []byte("name: x\nversion: 1.0.0\nskills: []\n"), 0644)
	_, _, code := runCLI(t, "init", "--root", dir)
	if code != exitcode.Usage {
		t.Errorf("expected usage error, got %d", code)
	}
}

func TestCLIInitForce(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "skillpack.yaml"), []byte("name: x\nversion: 1.0.0\nskills: []\n"), 0644)
	_, _, code := runCLI(t, "init", "--root", dir, "--force")
	if code != exitcode.OK {
		t.Errorf("expected ok, got %d", code)
	}
}

func TestCLIResolveJSON(t *testing.T) {
	dir := setupCLIWorkspace(t)
	stdout, _, code := runCLI(t, "resolve", "--root", dir, "--json")
	if code != exitcode.OK {
		t.Errorf("exit = %d", code)
	}
	var out []map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("not valid json: %v\n%s", err, stdout)
	}
	if len(out) != 2 {
		t.Errorf("got %d skills, want 2", len(out))
	}
}

func TestCLIResolveText(t *testing.T) {
	dir := setupCLIWorkspace(t)
	stdout, _, code := runCLI(t, "resolve", "--root", dir)
	if code != exitcode.OK {
		t.Errorf("exit = %d", code)
	}
	if !strings.Contains(stdout, "Install order") {
		t.Errorf("missing header: %q", stdout)
	}
}

func TestCLIInstall(t *testing.T) {
	dir := setupCLIWorkspace(t)
	_, _, code := runCLI(t, "install", "--root", dir)
	if code != exitcode.OK {
		t.Errorf("install exit = %d", code)
	}
	if _, err := os.Stat(filepath.Join(dir, "skillpack.lock")); err != nil {
		t.Errorf("lockfile not written")
	}
}

func TestCLIVerifyClean(t *testing.T) {
	dir := setupCLIWorkspace(t)
	_, _, code := runCLI(t, "install", "--root", dir)
	if code != exitcode.OK {
		t.Fatalf("install exit = %d", code)
	}
	stdout, _, code := runCLI(t, "verify", "--root", dir)
	if code != exitcode.OK {
		t.Errorf("verify exit = %d, output: %s", code, stdout)
	}
	if !strings.Contains(stdout, "OK") {
		t.Errorf("missing OK: %q", stdout)
	}
}

func TestCLIVerifyDrift(t *testing.T) {
	dir := setupCLIWorkspace(t)
	_, _, _ = runCLI(t, "install", "--root", dir)
	// Mutate one of the skill files.
	_ = os.WriteFile(filepath.Join(dir, "skills", "a", "SKILL.md"),
		[]byte("---\nname: a\nversion: 1.0.0\n---\nMUTATED body\n"), 0644)
	_, _, code := runCLI(t, "verify", "--root", dir)
	if code != exitcode.Drift {
		t.Errorf("expected drift code %d, got %d", exitcode.Drift, code)
	}
}

func TestCLIBundle(t *testing.T) {
	dir := setupCLIWorkspace(t)
	out := filepath.Join(dir, "test.skl")
	_, _, code := runCLI(t, "bundle", "--root", dir, "--out", out)
	if code != exitcode.OK {
		t.Errorf("bundle exit = %d", code)
	}
	if fi, err := os.Stat(out); err != nil || fi.Size() == 0 {
		t.Errorf("bundle not written")
	}
}

func TestCLIBundleList(t *testing.T) {
	dir := setupCLIWorkspace(t)
	stdout, _, code := runCLI(t, "bundle", "--root", dir, "--list")
	if code != exitcode.OK {
		t.Errorf("exit = %d", code)
	}
	if !strings.Contains(stdout, "manifest.json") {
		t.Errorf("no manifest in list: %q", stdout)
	}
}

func TestCLIKeygenSignVerify(t *testing.T) {
	dir := setupCLIWorkspace(t)
	priv := filepath.Join(dir, "priv.key")
	pub := filepath.Join(dir, "pub.key")
	_, _, code := runCLI(t, "keygen", "--priv", priv, "--pub", pub)
	if code != exitcode.OK {
		t.Fatalf("keygen exit = %d", code)
	}
	bundlePath := filepath.Join(dir, "test.skl")
	_, _, code = runCLI(t, "bundle", "--root", dir, "--out", bundlePath)
	if code != exitcode.OK {
		t.Fatal("bundle failed")
	}
	_, _, code = runCLI(t, "sign", "--key", priv, bundlePath)
	if code != exitcode.OK {
		t.Errorf("sign exit = %d", code)
	}
	_, _, code = runCLI(t, "sign", "--verify", "--pubkey", pub, bundlePath)
	if code != exitcode.OK {
		t.Errorf("sign --verify exit = %d", code)
	}
}

// TestCLIKeygenRejectsSamePath is the G1 regression: earlier the CLI let
// `--priv X --pub X` through and silently destroyed the private key by
// overwriting it with the public key. Now we refuse with a Usage error
// before any file is written.
func TestCLIKeygenRejectsSamePath(t *testing.T) {
	dir := t.TempDir()
	same := filepath.Join(dir, "k")
	_, stderr, code := runCLI(t, "keygen", "--priv", same, "--pub", same)
	if code != exitcode.Usage {
		t.Errorf("keygen same path: exit = %d, want Usage; stderr=%q", code, stderr)
	}
	// Nothing should have been written to disk.
	if _, err := os.Stat(same); err == nil {
		t.Error("keygen same path: file was written anyway")
	}
	// And the message should mention the conflict so users can debug.
	if !strings.Contains(stderr, "must differ") {
		t.Errorf("expected 'must differ' in stderr, got %q", stderr)
	}

	// The check must also fire for path equivalence (./k vs k) — compare
	// absolute paths, not raw strings.
	same2 := "./" + filepath.Base(same)
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	_, _, code = runCLI(t, "keygen", "--priv", same2, "--pub", filepath.Base(same))
	if code != exitcode.Usage {
		t.Errorf("keygen equivalent path: exit = %d, want Usage", code)
	}
}

func TestCLIKeygenRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	priv := filepath.Join(dir, "k.priv")
	pub := filepath.Join(dir, "k.pub")
	if _, _, code := runCLI(t, "keygen", "--priv", priv, "--pub", pub); code != exitcode.OK {
		t.Fatalf("first keygen exit = %d", code)
	}
	origPriv, err := os.ReadFile(priv)
	if err != nil {
		t.Fatalf("read priv: %v", err)
	}
	_, stderr, code := runCLI(t, "keygen", "--priv", priv, "--pub", pub)
	if code != exitcode.Usage {
		t.Errorf("second keygen without --force: exit = %d, want Usage; stderr=%q", code, stderr)
	}
	afterPriv, err := os.ReadFile(priv)
	if err != nil {
		t.Fatalf("read priv after: %v", err)
	}
	if !bytes.Equal(origPriv, afterPriv) {
		t.Error("private key was overwritten despite refusal exit code")
	}
	if _, _, code := runCLI(t, "keygen", "--priv", priv, "--pub", pub, "--force"); code != exitcode.OK {
		t.Errorf("keygen --force exit = %d", code)
	}
	forcedPriv, err := os.ReadFile(priv)
	if err != nil {
		t.Fatalf("read priv forced: %v", err)
	}
	if bytes.Equal(origPriv, forcedPriv) {
		t.Error("--force did not rewrite the private key")
	}
}

func TestCLISignMissingKey(t *testing.T) {
	dir := setupCLIWorkspace(t)
	bundlePath := filepath.Join(dir, "test.skl")
	_, _, _ = runCLI(t, "bundle", "--root", dir, "--out", bundlePath)
	_, _, code := runCLI(t, "sign", bundlePath)
	if code != exitcode.Usage {
		t.Errorf("expected usage error, got %d", code)
	}
}

func TestCLILock(t *testing.T) {
	dir := setupCLIWorkspace(t)
	_, _, code := runCLI(t, "lock", "--root", dir)
	if code != exitcode.OK {
		t.Errorf("lock exit = %d", code)
	}
}

func TestCLIAdd(t *testing.T) {
	dir := setupCLIWorkspace(t)
	_, _, code := runCLI(t, "add", "./extra", "--root", dir)
	if code != exitcode.OK {
		t.Errorf("add exit = %d", code)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "skillpack.yaml"))
	if !strings.Contains(string(data), "./extra") {
		t.Errorf("path not added: %s", string(data))
	}
}

func TestCLIAddDuplicate(t *testing.T) {
	dir := setupCLIWorkspace(t)
	_, _, _ = runCLI(t, "add", "./extra", "--root", dir)
	stdout, _, code := runCLI(t, "add", "./extra", "--root", dir)
	if code != exitcode.OK {
		t.Errorf("exit = %d", code)
	}
	if !strings.Contains(stdout, "already") {
		t.Errorf("missing dup message: %q", stdout)
	}
}

func TestCLIInvalidCommand(t *testing.T) {
	_, _, code := runCLI(t, "frobnicate")
	if code == exitcode.OK {
		t.Errorf("expected non-zero exit")
	}
}

func TestCLIVersion(t *testing.T) {
	stdout, _, code := runCLI(t, "--version")
	if code != exitcode.OK {
		t.Errorf("exit = %d", code)
	}
	if !strings.Contains(stdout, "skillpack") {
		t.Errorf("missing version output: %q", stdout)
	}
}

func TestCLIResolveMissingManifest(t *testing.T) {
	dir := t.TempDir()
	_, _, code := runCLI(t, "resolve", "--root", dir)
	if code == exitcode.OK {
		t.Errorf("expected error")
	}
}

func TestCLIVerifyJSON(t *testing.T) {
	dir := setupCLIWorkspace(t)
	_, _, _ = runCLI(t, "install", "--root", dir)
	stdout, _, code := runCLI(t, "verify", "--root", dir, "--json")
	if code != exitcode.OK {
		t.Errorf("exit = %d", code)
	}
	var v map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &v); err != nil {
		t.Errorf("not valid json: %v", err)
	}
}

// TestCLIVerifyDeletedFile covers H2: when a skill file is deleted from disk
// after install, `verify` must exit with the Drift code (1), NOT the Parse
// code (2). Deleting a file is a textbook drift case — CI should route it to
// "lockfile drift, open a PR", not "broken config, block merge".
func TestCLIVerifyDeletedFile(t *testing.T) {
	dir := setupCLIWorkspace(t)
	_, _, code := runCLI(t, "install", "--root", dir)
	if code != exitcode.OK {
		t.Fatalf("install exit = %d", code)
	}
	// Delete one of the skill files (skill "a" is depended on by "b", so the
	// resolver would normally flag [missing] as a Parse error — this test
	// verifies verify bypasses that path).
	if err := os.Remove(filepath.Join(dir, "skills", "a", "SKILL.md")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	stdout, _, code := runCLI(t, "verify", "--root", dir)
	if code != exitcode.Drift {
		t.Errorf("expected drift code %d, got %d, output: %s", exitcode.Drift, code, stdout)
	}
	if !strings.Contains(stdout, "missing") {
		t.Errorf("expected 'missing' finding, got: %s", stdout)
	}
}

// TestCLIBundleListFromDisk covers M2: `bundle --list <path.skl>` must
// inspect the on-disk bundle, not re-resolve the workspace. Without this
// fix, the positional argument was silently ignored.
func TestCLIBundleListFromDisk(t *testing.T) {
	dir := setupCLIWorkspace(t)
	out := filepath.Join(dir, "test.skl")
	_, _, code := runCLI(t, "bundle", "--root", dir, "--out", out)
	if code != exitcode.OK {
		t.Fatalf("bundle exit = %d", code)
	}
	// Move to a different workspace dir that is empty — if --list read the
	// workspace, it would fail; reading the bundle path must succeed.
	emptyDir := t.TempDir()
	manData := "name: other\nversion: 1.0.0\nskills: []\n"
	_ = os.WriteFile(filepath.Join(emptyDir, "skillpack.yaml"), []byte(manData), 0644)
	stdout, _, code := runCLI(t, "bundle", "--root", emptyDir, "--list", out)
	if code != exitcode.OK {
		t.Errorf("exit = %d, out: %s", code, stdout)
	}
	if !strings.Contains(stdout, "manifest.json") {
		t.Errorf("no manifest in list: %q", stdout)
	}
	// Must also contain entries for skills a and b from the bundled workspace.
	if !strings.Contains(stdout, "a") || !strings.Contains(stdout, "b") {
		t.Errorf("expected skills a and b in list, got: %s", stdout)
	}
}

// TestCLISignTamperedIsSecurity covers M3: a tampered bundle must exit with
// the Security code (6), not Drift (1). A signature-verification failure is a
// security event and should never be confused with a routine lockfile drift.
func TestCLISignTamperedIsSecurity(t *testing.T) {
	dir := setupCLIWorkspace(t)
	priv := filepath.Join(dir, "priv.key")
	pub := filepath.Join(dir, "pub.key")
	if _, _, code := runCLI(t, "keygen", "--priv", priv, "--pub", pub); code != exitcode.OK {
		t.Fatalf("keygen exit = %d", code)
	}
	bundlePath := filepath.Join(dir, "test.skl")
	if _, _, code := runCLI(t, "bundle", "--root", dir, "--out", bundlePath); code != exitcode.OK {
		t.Fatal("bundle failed")
	}
	if _, _, code := runCLI(t, "sign", "--key", priv, bundlePath); code != exitcode.OK {
		t.Fatalf("sign exit = %d", code)
	}
	// Tamper with the bundle body.
	f, err := os.OpenFile(bundlePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("open tamper: %v", err)
	}
	_, _ = f.Write([]byte("TAMPER"))
	_ = f.Close()
	_, stderr, code := runCLI(t, "sign", "--verify", "--pubkey", pub, bundlePath)
	if code != exitcode.Security {
		t.Errorf("expected Security code %d, got %d, stderr: %s", exitcode.Security, code, stderr)
	}
}

// TestCLIInstallPluralisation covers L3: ensure the CLI uses the singular
// form "(1 skill)" rather than the incorrect "(1 skills)" when exactly one
// skill is installed.
func TestCLIInstallPluralisation(t *testing.T) {
	dir := t.TempDir()
	manData := "name: solo\nversion: 1.0.0\nskills:\n  - ./skills\n"
	_ = os.WriteFile(filepath.Join(dir, "skillpack.yaml"), []byte(manData), 0644)
	_ = os.MkdirAll(filepath.Join(dir, "skills", "only"), 0755)
	_ = os.WriteFile(filepath.Join(dir, "skills", "only", "SKILL.md"),
		[]byte("---\nname: only\nversion: 1.0.0\n---\njust one\n"), 0644)
	stdout, _, code := runCLI(t, "install", "--root", dir)
	if code != exitcode.OK {
		t.Fatalf("install exit = %d", code)
	}
	if !strings.Contains(stdout, "(1 skill)") {
		t.Errorf("expected singular '(1 skill)', got: %q", stdout)
	}
	if strings.Contains(stdout, "(1 skills)") {
		t.Errorf("unexpected plural '(1 skills)' in output: %q", stdout)
	}
}

// Eval Cycle B — B5. `skillpack resolve` had a hard-coded "(%d skills)"
// format string that L3's pluralSkill helper didn't reach. Regression test
// makes sure `resolve` says "(1 skill)" for a single-skill workspace and
// that "(1 skills)" never appears in the output.
func TestCLIResolvePluralisation(t *testing.T) {
	dir := t.TempDir()
	manData := "name: solo\nversion: 1.0.0\nskills:\n  - ./skills\n"
	_ = os.WriteFile(filepath.Join(dir, "skillpack.yaml"), []byte(manData), 0644)
	_ = os.MkdirAll(filepath.Join(dir, "skills", "only"), 0755)
	_ = os.WriteFile(filepath.Join(dir, "skills", "only", "SKILL.md"),
		[]byte("---\nname: only\nversion: 1.0.0\n---\njust one\n"), 0644)
	stdout, _, code := runCLI(t, "resolve", "--root", dir)
	if code != exitcode.OK {
		t.Fatalf("resolve exit = %d", code)
	}
	if !strings.Contains(stdout, "(1 skill)") {
		t.Errorf("expected singular '(1 skill)', got: %q", stdout)
	}
	if strings.Contains(stdout, "(1 skills)") {
		t.Errorf("unexpected plural '(1 skills)' in output: %q", stdout)
	}
}

// Eval Cycle B — B1. A `skillpack.yaml` whose `skills:` entry escapes the
// workspace root (absolute path, `..`, drive letter) must be rejected with
// a Parse exit code before any filesystem walk happens. This is the
// adversarial probe from the Cycle B notes baked into the CLI test suite.
func TestCLISkillsEntryRejectsEscapes(t *testing.T) {
	cases := []string{
		"name: x\nversion: 1.0.0\nskills:\n  - ../../etc/passwd\n",
		"name: x\nversion: 1.0.0\nskills:\n  - /etc/passwd\n",
		"name: x\nversion: 1.0.0\nskills:\n  - \"C:/Windows\"\n",
	}
	for _, manData := range cases {
		dir := t.TempDir()
		_ = os.WriteFile(filepath.Join(dir, "skillpack.yaml"), []byte(manData), 0644)
		_, _, code := runCLI(t, "install", "--root", dir)
		if code != exitcode.Parse {
			t.Errorf("install with %q: expected Parse (%d), got %d", manData, exitcode.Parse, code)
		}
	}
}
