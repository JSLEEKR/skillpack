// Package bundle produces a deterministic tar.gz archive (".skl") from a
// resolved set of skills. Two runs over the same input must produce
// byte-identical archives so downstream signing and content-addressing work.
//
// Determinism techniques:
//   - Files are emitted in lexicographic name order.
//   - Each header has a fixed mtime, uid/gid 0, mode 0644, format PAX.
//   - The embedded manifest.json is the canonical lockfile bytes (LF newlines).
//   - We use compress/gzip with a fixed name and zero header time.
package bundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/JSLEEKR/skillpack/internal/exitcode"
	"github.com/JSLEEKR/skillpack/internal/lockfile"
	"github.com/JSLEEKR/skillpack/internal/skill"
)

// fixedMTime is epoch + 1 day, chosen to avoid the 1970-01-01 zero-mtime
// quirk that some tar implementations special-case.
var fixedMTime = time.Unix(86400, 0).UTC()

// Bundle bundles the resolved skills into a deterministic tar.gz archive.
//
// The skills slice carries each skill's normalized body. We do NOT re-read
// the original files from disk because the body has already been canonicalized
// during parsing, and re-reading would let CRLF/BOM differences into the tar.
func Bundle(skills []*skill.Skill, lf *lockfile.Lockfile) ([]byte, error) {
	if len(skills) == 0 {
		return nil, exitcode.Wrap(exitcode.Parse, errors.New("bundle: no skills to bundle"))
	}
	// Sort skills by name for deterministic file order.
	sorted := append([]*skill.Skill(nil), skills...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	var gzBuf bytes.Buffer
	gzw, err := gzip.NewWriterLevel(&gzBuf, gzip.BestCompression)
	if err != nil {
		return nil, exitcode.Wrap(exitcode.Internal, err)
	}
	gzw.Header.Name = "skillpack.skl"
	gzw.Header.ModTime = fixedMTime
	gzw.Header.OS = 255 // unknown -> deterministic across platforms

	tw := tar.NewWriter(gzw)

	// Write the manifest.json (lockfile snapshot) first.
	manifest, err := lockfile.Marshal(lf)
	if err != nil {
		return nil, err
	}
	if err := writeFileEntry(tw, "manifest.json", manifest); err != nil {
		return nil, err
	}

	// Write each skill body under skills/<name>/<sourceFilename>.
	for _, s := range sorted {
		if err := s.Validate(); err != nil {
			return nil, exitcode.Wrap(exitcode.Parse, fmt.Errorf("bundle: %w", err))
		}
		entryPath := skillEntryPath(s)
		if err := assertSafePath(entryPath); err != nil {
			return nil, exitcode.Wrap(exitcode.Parse, err)
		}
		if err := writeFileEntry(tw, entryPath, []byte(s.Body)); err != nil {
			return nil, err
		}
	}

	if err := tw.Close(); err != nil {
		return nil, exitcode.Wrap(exitcode.Internal, err)
	}
	if err := gzw.Close(); err != nil {
		return nil, exitcode.Wrap(exitcode.Internal, err)
	}
	return gzBuf.Bytes(), nil
}

// skillEntryPath returns the slash-delimited path inside the tar archive
// for the given skill. Always lowercase / ASCII / no traversal.
func skillEntryPath(s *skill.Skill) string {
	base := "skills/" + s.Name + "/"
	switch s.Format {
	case skill.FormatSkillMD:
		return base + "SKILL.md"
	case skill.FormatCursorRules:
		return base + ".cursorrules"
	case skill.FormatAgentMD:
		return base + "AGENT.md"
	case skill.FormatSkillYAML:
		return base + "skill.yaml"
	}
	return base + "skill"
}

// assertSafePath rejects entries that would escape the archive root.
// Run on every entry name immediately before write. Checks are platform-
// agnostic so they behave identically on Linux, macOS, and Windows.
func assertSafePath(p string) error {
	if p == "" {
		return errors.New("bundle: empty path")
	}
	if strings.ContainsAny(p, "\x00") {
		return fmt.Errorf("bundle: nul byte in path: %q", p)
	}
	// Reject absolute paths in both POSIX and Windows flavors.
	if strings.HasPrefix(p, "/") || strings.HasPrefix(p, "\\") {
		return fmt.Errorf("bundle: absolute path not allowed: %q", p)
	}
	if len(p) >= 2 && p[1] == ':' {
		// e.g. "C:\..." or "C:/..."
		return fmt.Errorf("bundle: drive-absolute path not allowed: %q", p)
	}
	// Check segments without letting filepath.Clean collapse traversal.
	segs := strings.FieldsFunc(p, func(r rune) bool { return r == '/' || r == '\\' })
	if len(segs) == 0 {
		return fmt.Errorf("bundle: empty segments: %q", p)
	}
	for _, seg := range segs {
		if seg == ".." {
			return fmt.Errorf("bundle: traversal path not allowed: %q", p)
		}
		if seg == "." {
			return fmt.Errorf("bundle: single-dot segment not allowed: %q", p)
		}
	}
	return nil
}

// writeFileEntry writes a single file entry with the deterministic header.
func writeFileEntry(tw *tar.Writer, name string, data []byte) error {
	hdr := &tar.Header{
		Name:     name,
		Mode:     0644,
		Size:     int64(len(data)),
		ModTime:  fixedMTime,
		Uid:      0,
		Gid:      0,
		Uname:    "",
		Gname:    "",
		Format:   tar.FormatPAX,
		Typeflag: tar.TypeReg,
		// PAXRecords is intentionally nil so headers stay minimal.
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return exitcode.Wrap(exitcode.Internal, fmt.Errorf("bundle write header %s: %w", name, err))
	}
	if _, err := tw.Write(data); err != nil {
		return exitcode.Wrap(exitcode.Internal, fmt.Errorf("bundle write body %s: %w", name, err))
	}
	return nil
}

// WriteFile serializes the archive to disk atomically.
func WriteFile(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return exitcode.Wrap(exitcode.IO, fmt.Errorf("bundle write %s: %w", path, err))
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(path)
		if err2 := os.Rename(tmp, path); err2 != nil {
			_ = os.Remove(tmp)
			return exitcode.Wrap(exitcode.IO, fmt.Errorf("bundle rename: %w", err2))
		}
	}
	return nil
}

// inspectMaxEntries caps the number of tar headers Inspect will read from
// an untrusted bundle. Our own bundler produces at most 1 + N_skills entries
// (manifest.json + one per skill); 10,000 is generous enough for all real
// bundles while bounding a tar-bomb on `bundle --list`.
const inspectMaxEntries = 10000

// inspectMaxEntrySize bounds the size any single entry may claim. Headers
// claiming more than 1 GiB per file are rejected outright — Inspect does
// not read entry bodies, so a huge-size claim is almost certainly a crafted
// bundle trying to mislead downstream tooling.
const inspectMaxEntrySize = 1 << 30

// Inspect returns a human-readable listing of the entries in a bundle, used
// for `skillpack bundle --list`. Hardened against tainted input:
//   - caps total entry count (inspectMaxEntries)
//   - rejects non-regular entries (symlink, hardlink, device, fifo, ...)
//   - runs assertSafePath on every name (rejects absolute paths, "..", drive
//     letters, and NUL bytes)
//   - caps per-entry claimed size
func Inspect(data []byte) ([]string, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, exitcode.Wrap(exitcode.Parse, fmt.Errorf("bundle inspect: %w", err))
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	out := []string{}
	count := 0
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, exitcode.Wrap(exitcode.Parse, fmt.Errorf("bundle inspect: %w", err))
		}
		count++
		if count > inspectMaxEntries {
			return nil, exitcode.Wrap(exitcode.Parse, fmt.Errorf("bundle inspect: too many entries (>%d)", inspectMaxEntries))
		}
		if hdr.Typeflag != tar.TypeReg {
			return nil, exitcode.Wrap(exitcode.Parse, fmt.Errorf("bundle inspect: unsupported entry type %q for %q", hdr.Typeflag, hdr.Name))
		}
		if hdr.Size < 0 || hdr.Size > inspectMaxEntrySize {
			return nil, exitcode.Wrap(exitcode.Parse, fmt.Errorf("bundle inspect: entry %q has invalid size %d", hdr.Name, hdr.Size))
		}
		if err := assertSafePath(hdr.Name); err != nil {
			return nil, exitcode.Wrap(exitcode.Parse, fmt.Errorf("bundle inspect: %w", err))
		}
		out = append(out, fmt.Sprintf("%s (%d bytes)", hdr.Name, hdr.Size))
	}
	return out, nil
}
