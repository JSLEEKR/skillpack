// Package signer creates and verifies ed25519 detached signatures over
// arbitrary byte payloads (typically a skillpack bundle).
//
// Key files are base64-encoded with a one-line header that indicates the
// algorithm. This avoids confusion with raw-binary key files and matches the
// shape of common signing tools (cosign, minisign).
package signer

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/JSLEEKR/skillpack/internal/exitcode"
)

const (
	privHeader = "skillpack-ed25519-private"
	pubHeader  = "skillpack-ed25519-public"
)

// GenerateKeypair returns a new ed25519 keypair with skillpack-formatted
// header lines suitable for writing to disk.
func GenerateKeypair() (privPEM, pubPEM []byte, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, exitcode.Wrap(exitcode.Internal, fmt.Errorf("signer: keygen: %w", err))
	}
	return encodeKey(privHeader, priv), encodeKey(pubHeader, pub), nil
}

func encodeKey(header string, key []byte) []byte {
	var b bytes.Buffer
	b.WriteString(header)
	b.WriteByte('\n')
	enc := base64.StdEncoding.EncodeToString(key)
	b.WriteString(enc)
	b.WriteByte('\n')
	return b.Bytes()
}

// LoadPrivateKey parses a private key file in the skillpack format.
func LoadPrivateKey(data []byte) (ed25519.PrivateKey, error) {
	key, header, err := decodeKey(data)
	if err != nil {
		return nil, err
	}
	if header != privHeader {
		return nil, exitcode.Wrap(exitcode.Parse, fmt.Errorf("signer: expected %q header, got %q", privHeader, header))
	}
	if len(key) != ed25519.PrivateKeySize {
		return nil, exitcode.Wrap(exitcode.Parse, fmt.Errorf("signer: invalid private key size: %d", len(key)))
	}
	return ed25519.PrivateKey(key), nil
}

// LoadPublicKey parses a public key file in the skillpack format.
func LoadPublicKey(data []byte) (ed25519.PublicKey, error) {
	key, header, err := decodeKey(data)
	if err != nil {
		return nil, err
	}
	if header != pubHeader {
		return nil, exitcode.Wrap(exitcode.Parse, fmt.Errorf("signer: expected %q header, got %q", pubHeader, header))
	}
	if len(key) != ed25519.PublicKeySize {
		return nil, exitcode.Wrap(exitcode.Parse, fmt.Errorf("signer: invalid public key size: %d", len(key)))
	}
	return ed25519.PublicKey(key), nil
}

// decodeKey parses a skillpack-format key file. The format is EXACTLY two
// non-empty lines:
//
//	<header>\n
//	<base64>\n
//
// Any trailing non-empty line is rejected. This keeps `sign` honest — a
// corrupted or concatenated key file (e.g. `cat garbage >> key.priv`) is a
// hard error, not silently-accepted-with-only-the-first-two-lines.
//
// PEM / OpenSSH-style multi-line base64 bodies are intentionally rejected:
// if a user pastes such a body, the correct fix is to rewrap it to a single
// line, not to pretend we understand PEM.
func decodeKey(data []byte) ([]byte, string, error) {
	// Normalize CRLF so CRLF-terminated key files from Windows editors work.
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	// Collect non-empty lines after trimming whitespace.
	var lines []string
	for _, raw := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		lines = append(lines, trimmed)
	}
	if len(lines) < 2 {
		return nil, "", exitcode.Wrap(exitcode.Parse, errors.New("signer: key file too short: want 2 non-empty lines"))
	}
	if len(lines) > 2 {
		return nil, "", exitcode.Wrap(exitcode.Parse, errors.New("signer: malformed key: unexpected trailing data"))
	}
	header := lines[0]
	body := lines[1]
	key, err := base64.StdEncoding.DecodeString(body)
	if err != nil {
		return nil, "", exitcode.Wrap(exitcode.Parse, fmt.Errorf("signer: bad base64: %w", err))
	}
	return key, header, nil
}

// Sign returns a detached ed25519 signature, base64-encoded with a header line.
func Sign(priv ed25519.PrivateKey, payload []byte) []byte {
	sig := ed25519.Sign(priv, payload)
	return encodeKey("skillpack-ed25519-signature", sig)
}

// Verify checks a signature produced by Sign. Returns nil on success.
func Verify(pub ed25519.PublicKey, payload, sigFile []byte) error {
	sig, header, err := decodeKey(sigFile)
	if err != nil {
		return err
	}
	if header != "skillpack-ed25519-signature" {
		return exitcode.Wrap(exitcode.Parse, fmt.Errorf("signer: not a signature file: %q", header))
	}
	if len(sig) != ed25519.SignatureSize {
		return exitcode.Wrap(exitcode.Parse, fmt.Errorf("signer: invalid signature size: %d", len(sig)))
	}
	if !ed25519.Verify(pub, payload, sig) {
		// A tampered payload or wrong-key signature is a SECURITY event,
		// not lockfile drift. Emit a dedicated exit code so CI pipelines can
		// treat this as a hard-fail instead of a routine "refresh the lock".
		return exitcode.Wrap(exitcode.Security, errors.New("signer: signature does not verify"))
	}
	return nil
}

// SignFile loads a private key and a payload from disk and writes a detached
// signature to outPath. The signature filename convention is "<payload>.sig".
func SignFile(privKeyPath, payloadPath, outPath string) error {
	priv, err := loadPrivFile(privKeyPath)
	if err != nil {
		return err
	}
	payload, err := os.ReadFile(payloadPath)
	if err != nil {
		return exitcode.Wrap(exitcode.IO, fmt.Errorf("signer: read payload: %w", err))
	}
	sig := Sign(priv, payload)
	if err := os.WriteFile(outPath, sig, 0644); err != nil {
		return exitcode.Wrap(exitcode.IO, fmt.Errorf("signer: write signature: %w", err))
	}
	return nil
}

// VerifyFile loads a public key, payload, and signature from disk and verifies.
func VerifyFile(pubKeyPath, payloadPath, sigPath string) error {
	pub, err := loadPubFile(pubKeyPath)
	if err != nil {
		return err
	}
	payload, err := os.ReadFile(payloadPath)
	if err != nil {
		return exitcode.Wrap(exitcode.IO, fmt.Errorf("signer: read payload: %w", err))
	}
	sigData, err := os.ReadFile(sigPath)
	if err != nil {
		return exitcode.Wrap(exitcode.IO, fmt.Errorf("signer: read signature: %w", err))
	}
	return Verify(pub, payload, sigData)
}

func loadPrivFile(path string) (ed25519.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, exitcode.Wrap(exitcode.IO, fmt.Errorf("signer: read private key: %w", err))
	}
	return LoadPrivateKey(data)
}

func loadPubFile(path string) (ed25519.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, exitcode.Wrap(exitcode.IO, fmt.Errorf("signer: read public key: %w", err))
	}
	return LoadPublicKey(data)
}
