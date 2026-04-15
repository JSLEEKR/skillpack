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

### Security hardening (Eval Cycle B)

- Manifest `skills:` entries are now constrained to the workspace root:
  absolute paths, drive-letter paths, `..` segments, and POSIX-rooted
  paths are all rejected with exit code 2 (Parse). Symlinks that point
  outside the workspace are also rejected post-`EvalSymlinks`.
- Skill names `.`, `..`, leading-dot (e.g. `.hidden`), embedded `..`, and
  names with leading/trailing whitespace are rejected at validate time,
  preventing poisoned lockfile entries.
- Private key files require exactly two non-empty lines; trailing garbage
  and multi-line base64 bodies produce clear `malformed key` errors.
- `bundle --list` hardens against tainted archives: caps 10,000 entries,
  rejects non-regular types (symlink/hardlink/device/fifo), enforces
  `assertSafePath` on every entry name.
- Dedicated exit code `Security = 6` for signature tamper / verification
  failure.

### Integrity hardening (Eval Cycle G)

- Canonical hash pre-image rewritten: every string value is now
  `strconv.Quote`-escaped and multi-element fields (`tools`, `requires`)
  emit one indexed line per element. The body is length-prefixed with
  `body.len=N`. This removes three classes of hash collisions (comma-joined
  tools, pipe-joined requires, `=`-separated frontmatter, newline folding)
  that could have let two semantically distinct skills share the same
  sha256 fingerprint.
- `skillpack keygen --priv X --pub X` now refuses (Usage error) before
  writing — previously the second write silently destroyed the first.
  Check compares `filepath.Abs` so `./k` and `k` are also caught.
- `skillpack keygen` refuses to overwrite existing key files unless
  `--force` is passed (Eval Cycle E hardening).
- `lockfile.Unmarshal` now rejects every non-positive `"version"` value.
  Previously only `version == 0` was caught.

### JSON schema consistency (Eval Cycle J)

- `verify --json` now emits snake_case keys (`drifted`, `missing`, `extra`,
  `findings`, `ok` and `name`/`kind`/`want`/`got`/`message` for each
  finding). Previously `verify.Result` and `verify.Finding` had no
  `json:"..."` tags so encoding/json fell back to PascalCase Go field
  names — inconsistent with `resolve --json`, the lockfile, and the
  manifest. Pinned by `TestResultJSONSchemaIsSnakeCase` so it can't
  regress.
- `internal/docsmeta/docsmeta_test.go` error messages had stale `'213
  tests'` references inside `t.Errorf` strings even though the assertion
  bodies checked for `'216 tests'`. Fixed and now pinned by a
  meta-meta-test (`TestDocsmetaTestSelfConsistent`) that scans the source
  and rejects any non-comment `NNN tests` mismatch with the current
  pinned count.

### Lockfile integrity (Eval Cycle K)

- `lockfile.Unmarshal` rejects lockfiles containing two entries with the
  same skill name. Previously accepted silently; the linear-scan
  `LookupSkill` returned only the first match, making the second entry
  invisible to `verify`. The canonical lockfile produced by `FromSkills`
  never contains duplicates (the resolver rejects them upstream), so a
  duplicate on disk is always corruption or a hand-edit and must surface
  as a Parse error. Pinned by `TestUnmarshalRejectsDuplicateSkillNames`.
- `gofmt -w` applied to `internal/hasher/hasher.go`,
  `internal/parser/agentmd.go`, and `internal/parser/parser_test.go` —
  three cosmetic drifts (trailing blank line and two struct-tag column
  alignments) that `gofmt -l` flagged but no prior CI step ran.

### Quality

- **220 tests** across unit and integration layers (192 initial + 28
  across Eval Cycles B through K, including 4 doc-accuracy meta-tests,
  one snake_case JSON schema regression pin, and two lockfile-duplicate
  regression pins).
- `go vet` clean, race-detector clean, `gofmt -l` clean.
- Cross-platform: tested on Windows, Linux, macOS paths.
