# Changelog

All notable changes to skillpack will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-04-14

### Added

- **Multi-format parser** for `SKILL.md` (Anthropic), `.cursorrules` (Cursor),
  `AGENT.md` (cross-vendor), and `skill.yaml`. Every format normalizes into a
  single canonical `Skill` record.
- **Semver resolver** supporting caret (`^1.2.3`), tilde (`~1.2.3`),
  comparators (`>=`, `<=`, `>`, `<`, `=`), exact, x-ranges (`1.2.x`), and
  wildcard (`*`). Deterministic topological install order.
- **Content-addressed hashing** via sha256 over a canonical pre-image
  (LF-normalized body, sorted frontmatter keys, sorted tools, sorted requires).
- **Deterministic lockfile** (`skillpack.lock`) — sorted keys, LF line endings,
  trailing newline, no timestamps. Two runs over the same input produce
  byte-identical lockfiles.
- **Deterministic tarball bundler** (`*.skl`) — sorted file order, fixed
  mtime, uid/gid 0, PAX format, reproducible across platforms.
- **ed25519 signer** for detached bundle signatures with base64-encoded key
  files and skillpack-branded headers.
- **CI verify mode** with distinct exit codes (0 pass / 1 drift / 2 parse /
  3 IO / 4 internal / 5 usage).
- **CLI commands**: `init`, `add`, `resolve`, `install`, `verify`, `bundle`,
  `sign`, `keygen`, `lock`.
- Single static Go binary under 5 MB.

### Quality

- 188+ tests across unit and integration layers.
- `go vet` clean, race-detector clean.
- Cross-platform: tested on Windows, Linux, macOS paths.
