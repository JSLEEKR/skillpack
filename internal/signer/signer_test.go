package signer

import (
	"crypto/ed25519"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateAndRoundtrip(t *testing.T) {
	priv, pub, err := GenerateKeypair()
	if err != nil {
		t.Fatal(err)
	}
	p, err := LoadPrivateKey(priv)
	if err != nil {
		t.Fatalf("load priv: %v", err)
	}
	u, err := LoadPublicKey(pub)
	if err != nil {
		t.Fatalf("load pub: %v", err)
	}
	if len(p) != ed25519.PrivateKeySize || len(u) != ed25519.PublicKeySize {
		t.Errorf("wrong sizes: %d/%d", len(p), len(u))
	}
}

func TestSignVerify(t *testing.T) {
	priv, pub, _ := GenerateKeypair()
	p, _ := LoadPrivateKey(priv)
	u, _ := LoadPublicKey(pub)
	payload := []byte("hello bundle")
	sig := Sign(p, payload)
	if err := Verify(u, payload, sig); err != nil {
		t.Errorf("verify failed: %v", err)
	}
}

func TestVerifyTampered(t *testing.T) {
	priv, pub, _ := GenerateKeypair()
	p, _ := LoadPrivateKey(priv)
	u, _ := LoadPublicKey(pub)
	sig := Sign(p, []byte("original"))
	if err := Verify(u, []byte("tampered"), sig); err == nil {
		t.Errorf("expected verify to fail on tampered payload")
	}
}

func TestVerifyWrongKey(t *testing.T) {
	priv1, _, _ := GenerateKeypair()
	_, pub2, _ := GenerateKeypair()
	p1, _ := LoadPrivateKey(priv1)
	u2, _ := LoadPublicKey(pub2)
	sig := Sign(p1, []byte("payload"))
	if err := Verify(u2, []byte("payload"), sig); err == nil {
		t.Errorf("expected verify to fail on wrong key")
	}
}

func TestLoadPublicKeyWithPrivHeaderFails(t *testing.T) {
	priv, _, _ := GenerateKeypair()
	if _, err := LoadPublicKey(priv); err == nil {
		t.Errorf("should reject private key loaded as public")
	}
}

func TestLoadPrivateKeyWithPubHeaderFails(t *testing.T) {
	_, pub, _ := GenerateKeypair()
	if _, err := LoadPrivateKey(pub); err == nil {
		t.Errorf("should reject public key loaded as private")
	}
}

func TestLoadPrivateKeyTooShort(t *testing.T) {
	if _, err := LoadPrivateKey([]byte("only one line")); err == nil {
		t.Errorf("expected error")
	}
}

func TestLoadPrivateKeyBadBase64(t *testing.T) {
	data := []byte("skillpack-ed25519-private\nnot base64!!!\n")
	if _, err := LoadPrivateKey(data); err == nil {
		t.Errorf("expected error")
	}
}

func TestSignFileRoundtrip(t *testing.T) {
	dir := t.TempDir()
	privPath := filepath.Join(dir, "priv.key")
	pubPath := filepath.Join(dir, "pub.key")
	payloadPath := filepath.Join(dir, "bundle.skl")
	sigPath := filepath.Join(dir, "bundle.skl.sig")
	priv, pub, _ := GenerateKeypair()
	_ = os.WriteFile(privPath, priv, 0644)
	_ = os.WriteFile(pubPath, pub, 0644)
	_ = os.WriteFile(payloadPath, []byte("payload bytes"), 0644)
	if err := SignFile(privPath, payloadPath, sigPath); err != nil {
		t.Fatal(err)
	}
	if err := VerifyFile(pubPath, payloadPath, sigPath); err != nil {
		t.Errorf("verify failed: %v", err)
	}
}

func TestSignFileMissingPriv(t *testing.T) {
	dir := t.TempDir()
	err := SignFile(filepath.Join(dir, "nope"), filepath.Join(dir, "also-nope"), filepath.Join(dir, "sig"))
	if err == nil {
		t.Errorf("expected error")
	}
}

func TestVerifyFileMissingPayload(t *testing.T) {
	dir := t.TempDir()
	_, pub, _ := GenerateKeypair()
	pubPath := filepath.Join(dir, "pub.key")
	_ = os.WriteFile(pubPath, pub, 0644)
	err := VerifyFile(pubPath, filepath.Join(dir, "missing"), filepath.Join(dir, "missing.sig"))
	if err == nil {
		t.Errorf("expected error")
	}
}

func TestCRLFInKeyFile(t *testing.T) {
	priv, _, _ := GenerateKeypair()
	// inject CRLF
	withCRLF := strings.ReplaceAll(string(priv), "\n", "\r\n")
	p, err := LoadPrivateKey([]byte(withCRLF))
	if err != nil {
		t.Fatalf("CRLF should be normalized: %v", err)
	}
	if len(p) != ed25519.PrivateKeySize {
		t.Errorf("size mismatch")
	}
}

func TestSignatureFormat(t *testing.T) {
	priv, _, _ := GenerateKeypair()
	p, _ := LoadPrivateKey(priv)
	sig := Sign(p, []byte("x"))
	if !strings.HasPrefix(string(sig), "skillpack-ed25519-signature") {
		t.Errorf("missing header: %q", sig[:40])
	}
}

func TestVerifyBadSigFormat(t *testing.T) {
	_, pub, _ := GenerateKeypair()
	u, _ := LoadPublicKey(pub)
	if err := Verify(u, []byte("payload"), []byte("totally not a sig")); err == nil {
		t.Errorf("expected error on bad sig format")
	}
}
