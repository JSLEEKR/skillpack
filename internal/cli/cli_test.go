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
