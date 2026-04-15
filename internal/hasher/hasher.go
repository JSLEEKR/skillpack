// Package hasher computes a deterministic content-addressed sha256 fingerprint
// for a Skill. The canonical byte form is independent of line endings,
// frontmatter key order, and platform-specific path separators, so two
// machines parsing the same source file will produce identical hashes.
package hasher

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/JSLEEKR/skillpack/internal/skill"
)

// CanonicalBytes returns the deterministic representation used as the
// hash pre-image. The format is a line-oriented protocol where every value
// is rendered via strconv.Quote so newlines, separators, and control bytes
// inside values cannot collide across different inputs.
//
// Format (each line ends with LF):
//
//	format=<quoted>
//	name=<quoted>
//	version=<quoted>
//	description=<quoted>
//	license=<quoted>
//	author=<quoted>
//	tools[<i>]=<quoted>             (one line per tool, sorted)
//	requires[<i>].name=<quoted>     (one line per dep, sorted by name then expr)
//	requires[<i>].expr=<quoted>
//	frontmatter[<key>]=<quoted>     (one line per frontmatter key, sorted, with
//	                                 already-represented scalar keys skipped)
//	body.len=<n>
//	---body---
//	<n bytes of body, LF-normalized>
//
// Why per-element lines instead of comma/pipe-joined lists: a joined form
// like `tools=a,b,c` cannot distinguish `["a,b","c"]` from `["a","b,c"]`.
// Length-prefixing the body and quoting every other value removes every
// ambiguity, so two distinct skills cannot share a canonical preimage.
func CanonicalBytes(s *skill.Skill) []byte {
	if s == nil {
		return nil
	}
	var b strings.Builder
	writeKV := func(k, v string) {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(strconv.Quote(v))
		b.WriteByte('\n')
	}
	writeKV("format", string(s.Format))
	writeKV("name", s.Name)
	writeKV("version", s.Version)
	writeKV("description", s.Description)
	writeKV("license", s.License)
	writeKV("author", s.Author)
	for i, t := range s.SortedTools() {
		writeKV("tools["+strconv.Itoa(i)+"]", t)
	}
	for i, c := range s.SortedRequires() {
		idx := strconv.Itoa(i)
		writeKV("requires["+idx+"].name", c.Name)
		writeKV("requires["+idx+"].expr", c.Expr)
	}
	// Frontmatter: skip keys already represented above to avoid double counting.
	skipFM := map[string]struct{}{
		"name": {}, "version": {}, "description": {}, "license": {}, "author": {},
	}
	for _, k := range s.SortedFrontmatterKeys() {
		if _, skip := skipFM[k]; skip {
			continue
		}
		// Quote the key too — frontmatter keys can in principle contain any
		// rune (YAML map key), and we must distinguish e.g. "k=v" used as a
		// key vs as a value.
		b.WriteString("frontmatter[")
		b.WriteString(strconv.Quote(k))
		b.WriteString("]=")
		b.WriteString(strconv.Quote(s.Frontmatter[k]))
		b.WriteByte('\n')
	}
	body := s.Body
	b.WriteString("body.len=")
	b.WriteString(strconv.Itoa(len(body)))
	b.WriteByte('\n')
	b.WriteString("---body---\n")
	b.WriteString(body)
	return []byte(b.String())
}

// Hash computes the "sha256:<hex>" fingerprint for the skill. The skill's
// Hash field is NOT mutated; callers decide when to cache.
func Hash(s *skill.Skill) string {
	sum := sha256.Sum256(CanonicalBytes(s))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// HashBytes is a convenience for hashing arbitrary byte payloads (e.g. tar
// archives) with the same "sha256:<hex>" prefix format.
func HashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// Equal compares two hashes for equality with constant-time semantics
// proportional to the hex length. Hex comparison is fine because the
// content is an already-digested fingerprint (no secret to leak).
func Equal(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}
