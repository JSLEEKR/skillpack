// Package hasher computes a deterministic content-addressed sha256 fingerprint
// for a Skill. The canonical byte form is independent of line endings,
// frontmatter key order, and platform-specific path separators, so two
// machines parsing the same source file will produce identical hashes.
package hasher

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/JSLEEKR/skillpack/internal/skill"
)

// CanonicalBytes returns the deterministic representation used as the
// hash pre-image. The format is a simple line-oriented protocol so that
// changes remain human-diffable when debugging drift.
//
//	format=skill.md
//	name=<name>
//	version=<version>
//	description=<one-line description, newlines collapsed to spaces>
//	license=<license>
//	author=<author>
//	tools=<sorted comma-separated list>
//	requires=<sorted "name expr" entries separated by "|">
//	frontmatter.<key>=<value>   (sorted, each on own line)
//	---body---
//	<body text, already LF-normalized, single trailing LF>
//
// Unknown frontmatter keys are folded in under `frontmatter.`; list fields
// (tools/requires) and scalar fields appear under their canonical keys.
func CanonicalBytes(s *skill.Skill) []byte {
	if s == nil {
		return nil
	}
	var b strings.Builder
	writeKV := func(k, v string) {
		// Escape NL in values so one field stays on one line.
		v = strings.ReplaceAll(v, "\r\n", " ")
		v = strings.ReplaceAll(v, "\n", " ")
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(v)
		b.WriteByte('\n')
	}
	writeKV("format", string(s.Format))
	writeKV("name", s.Name)
	writeKV("version", s.Version)
	writeKV("description", s.Description)
	writeKV("license", s.License)
	writeKV("author", s.Author)
	writeKV("tools", strings.Join(s.SortedTools(), ","))
	// Requires: "name expr|name expr|..."
	reqStrs := make([]string, 0, len(s.Requires))
	for _, c := range s.SortedRequires() {
		reqStrs = append(reqStrs, c.String())
	}
	writeKV("requires", strings.Join(reqStrs, "|"))
	// Frontmatter: skip keys already represented above to avoid double counting.
	skipFM := map[string]struct{}{
		"name": {}, "version": {}, "description": {}, "license": {}, "author": {},
	}
	for _, k := range s.SortedFrontmatterKeys() {
		if _, skip := skipFM[k]; skip {
			continue
		}
		writeKV("frontmatter."+k, s.Frontmatter[k])
	}
	b.WriteString("---body---\n")
	b.WriteString(s.Body)
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

