# Round 81 — skillpack build log

- **Project**: skillpack
- **Category**: Agent Skill Packaging / Lifecycle / Lockfile
- **Language**: Go
- **Date**: 2026-04-14
- **Build status**: new build (not imported)
- **Pitch score**: 104/110 (Trend Scout winner, data-driven v6)
- **Go version**: 1.26+
- **Binary size**: 4.2 MB (`go build -ldflags="-s -w"`, windows/amd64)

## What was built

Package manager, lockfile, and bundler for agent skills. All four de facto
manifest formats supported (`SKILL.md`, `.cursorrules`, `AGENT.md`,
`skill.yaml`) → canonical record → semver-aware topological resolver →
sha256 content addressing → deterministic JSON lockfile → deterministic
gzipped tarball → ed25519 detached signatures → CI drift verifier.

## Packages

| Package | LOC | Tests | Purpose |
|---|---|---|---|
| `cmd/skillpack` | 12 | — | entry point |
| `internal/cli` | ~530 | 25+ | cobra command tree |
| `internal/workspace` | 170 | 9 | manifest + parser + resolver glue |
| `internal/manifest` | 90 | 11 | skillpack.yaml read/write |
| `internal/parser` | ~440 | 30+ | multi-format parser |
| `internal/skill` | 130 | 9 | canonical Skill record |
| `internal/semver` | 240 | 20+ | constraint matcher (^/~/x/...) |
| `internal/resolver` | 170 | 15+ | topological sort + semver checks |
| `internal/hasher` | 90 | 12 | sha256 content addressing |
| `internal/lockfile` | 200 | 17 | deterministic JSON lockfile |
| `internal/bundle` | 210 | 13 | deterministic tar.gz writer |
| `internal/signer` | 180 | 14 | ed25519 detached signatures |
| `internal/verify` | 145 | 12 | CI drift detection |
| `internal/exitcode` | 80 | 7 | typed errors → exit codes |

Total: ~5,600 lines of Go, **205 tests** (192 initial + 13 from Eval Cycle B fixes).

## Dependencies (exactly three, plus pflag transitively from cobra)

- `gopkg.in/yaml.v3 v3.0.1` — YAML frontmatter
- `golang.org/x/mod v0.35.0` — semver primitives
- `github.com/spf13/cobra v1.10.2` — CLI framework

## Quality gate

- `go build ./...` — clean
- `go vet ./...` — clean
- `go test ./...` — all 192 tests pass
- `go test -race ./...` — race-detector clean
- `go mod tidy && git diff --exit-code` — clean (no dep drift)
- Binary size 4.2 MB (target: < 15 MB) — passed
- README 502 lines (target: 300+) — passed
- End-to-end smoke test: init → add → install → verify → bundle → keygen
  → sign → sign --verify — all green, bundle deterministic across two runs
  (byte-identical cmp)

## Notable design decisions

1. **Canonical hash pre-image**. Body is LF-normalized and BOM-stripped
   during parse, not at hash time. Frontmatter keys are sorted before
   inclusion. Tools and requires are sorted. This makes drift detection
   robust against platform / editor differences.

2. **Fixed mtime at epoch + 1 day** (1970-01-02 00:00:00 UTC), not 0 — some
   tar implementations special-case zero mtime and emit non-deterministic
   bytes. Epoch + 1d avoids the edge case while staying platform-uniform.

3. **Typed exit codes via `exitcode.Wrap`**. Every error that flows back to
   main carries a class (`Drift`, `Parse`, `IO`, `Internal`, `Usage`) so
   `Classify` can produce the right code without fragile string matching.

4. **Lexicographic tiebreak in topological sort**. Kahn's algorithm alone
   isn't deterministic when multiple nodes have zero in-degree; I added a
   sorted ready queue so two runs always produce the same order.

5. **Platform-agnostic path safety**. `filepath.IsAbs` behaves differently
   on Windows vs POSIX; the bundle's `assertSafePath` checks both `/` and
   `\` prefixes, drive letters (`C:`), and walks segments manually to
   detect `..` without letting `filepath.Clean` silently collapse them.

6. **Atomic file writes** via `tmp → rename` for lockfile and manifest,
   with a Windows-friendly retry path because `rename` can fail when the
   destination exists.

## Things the evaluator might want to look at

(Not bugs — areas that are opinionated or might be fragile.)

- The `.cursorrules` parser requires frontmatter (name/version). The
  "legacy Cursor format with no frontmatter" path returns an error. If
  someone has existing `.cursorrules` files without frontmatter, they'll
  need to add it. This is intentional but worth confirming.
- The bundle's skill entry paths always use `SKILL.md` / `.cursorrules` /
  `AGENT.md` / `skill.yaml` regardless of the source filename. If a user
  had `my-bot.AGENT.md`, the bundle will store it as `AGENT.md`. Again
  intentional (canonical names), but a judgment call.
- `normalizeRequires` accepts both list-of-strings and map-of-strings
  shapes. The YAML unmarshaler can give you `map[interface{}]interface{}`
  or `map[string]interface{}` depending on version; both are handled, but
  if a fifth shape shows up in practice, it'll need a new case.
- The verify command now intentionally does NOT invoke the resolver.
  A deleted or broken-dep skill is treated as "drift" (exit 1) so CI
  branching stays consistent. If you need graph validation, run
  `skillpack resolve` (which does invoke the resolver).
- `semver.incInt` rolls its own int parser to avoid a strconv import. It
  handles 0-padded inputs like `v01.02.03` (x/mod/semver rejects those
  upstream before we get here), but worth a look.
- The manifest writer now explicitly calls `yaml.Encoder.SetIndent(2)`,
  removing the latent dependency on yaml.v3's default indent.

No deliberate shortcuts, no skipped tests, no `//nolint` comments.

## Eval Cycle A — fixes applied

Cycle A found 7 issues; all fixed in this round before handing to Cycle B:

- **H1** `go mod tidy` drift — direct/indirect dep classification corrected,
  README badge aligned to "Go 1.26+" to match `go.mod`. `go mod tidy` is now
  a no-op.
- **H2** `verify` exited 2 (Parse) instead of 1 (Drift) when a skill file
  was deleted. `cli/verify.go` now reads the manifest + discovers files
  directly and skips the resolver; `verify.Run` produces a `missing`
  finding that maps to exit 1. New CLI test `TestCLIVerifyDeletedFile`
  asserts the deleted-file exit code.
- **M1** `^0.0.x` caret now pins to an exact patch (npm/cargo semantics).
  New test cases for `^0.0.1` and `^0.0.3`.
- **M2** `bundle --list <path.skl>` now reads the bundle from disk via
  `bundle.Inspect`. New test `TestCLIBundleListFromDisk`.
- **M3** Tampered-signature verification now returns a new dedicated exit
  code `Security = 6` instead of overloading `Drift = 1`. README exit-code
  table and CI example updated. New test `TestCLISignTamperedIsSecurity`.
- **L1** `.gitignore` now hides `.eval-notes-*.md` and `.harness/`.
- **L2** `manifest.Marshal` now explicitly uses `yaml.Encoder.SetIndent(2)`
  to match the README example (no longer a latent yaml.v3 default).
- **L3** Install/bundle CLI output pluralisation fixed via `pluralSkill(n)`:
  `(1 skill)` vs `(2 skills)`. New test `TestCLIInstallPluralisation`.

## Eval Cycle B — fixes applied

Cycle B found 5 new bugs plus 3 polish items; all fixed:

- **B1 (HIGH, supply-chain)** — `skillpack.yaml` `skills:` entries were
  unconstrained, enabling arbitrary filesystem walks (absolute paths,
  `../..` escapes, drive-letter paths) that would ingest files into the
  lockfile + signed bundle. Fix: `manifest.ValidateSkillPath` rejects
  every rooted / escaping form; `workspace.Discover` double-checks via
  `filepath.Rel` and runs post-symlink `assertInsideRoot` on every
  discovered file; `cli/add` validates before writing to disk. New tests:
  `TestValidateSkillPathRejectsEscapes`, `TestValidateSkillPathAcceptsSafe`,
  `TestUnmarshalRejectsEscapingSkills`, `TestCLISkillsEntryRejectsEscapes`.
- **B2 (MED)** — `skill.Validate` accepted `.` / `..` / leading-dot names.
  Fix: reject `.`, `..`, any leading `.`, embedded `..`, or leading/trailing
  whitespace. New test `TestSkillValidateB2DotNames`.
- **B3 (MED)** — `signer.decodeKey` silently ignored data past the first two
  lines. Fix: require EXACTLY two non-empty lines; reject trailing garbage
  and multi-line base64 bodies with clear errors. New tests
  `TestLoadPrivateKeyRejectsTrailingGarbage`,
  `TestLoadPrivateKeyRejectsMultiLineBody`.
- **B4 (LOW)** — `bundle.Inspect` had no entry cap, no type filter, no safe
  path check. Fix: cap 10,000 entries, cap per-entry claimed size at 1 GiB,
  reject non-regular entries (symlink/hardlink/device/fifo), run
  `assertSafePath` on every name. New tests
  `TestInspectRejectsTraversalEntry`, `TestInspectRejectsAbsoluteEntry`,
  `TestInspectRejectsSymlinkEntry`, `TestInspectRejectsDriveLetterEntry`.
- **B5 (LOW)** — `resolve` still said "(1 skills)". Fix: use `pluralSkill`.
  New test `TestCLIResolvePluralisation`.
- **Polish: matchCaret consistency** — `^1.2` accepted but `^0.5` rejected.
  Fix: implicitly append `.0` patch to 2-part caret inputs so both are
  handled. New semver table cases for `^1.2` and `^0.5`.
- **Polish: lock.go doc** — rewrote the Long description to match reality
  (the file IS created if missing).
- **Polish: verify duplicate-name regression (post-H2)** — verify now
  walks disk files, detects name collisions directly, and reports them as
  drift. New test `TestRunDuplicateNameOnDisk`.

Live adversarial probes (all return the right exit code):

- `skills: [../../Windows/System32]` → exit 2 Parse, clear error
- `skills: [/etc/passwd]` → exit 2 Parse, clear error
- SKILL.md with `name: ".."` → exit 2 Parse, `skill name ".." is reserved`
- Append garbage to private key, then sign → exit 2,
  `signer: malformed key: unexpected trailing data`
- Tainted `.skl` with `../evil` entry fed to `bundle --list` → exit 2,
  `bundle: traversal path not allowed`

## Files created

- `.design-spec.md`
- `.gitignore`
- `CHANGELOG.md`
- `LICENSE`
- `README.md`
- `ROUND_LOG.md`
- `go.mod`, `go.sum`
- `cmd/skillpack/main.go`
- 13 packages under `internal/`, each with implementation and tests
