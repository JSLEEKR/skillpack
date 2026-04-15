# Round 81 — skillpack build log

- **Project**: skillpack
- **Category**: Agent Skill Packaging / Lifecycle / Lockfile
- **Language**: Go
- **Date**: 2026-04-14
- **Build status**: new build (not imported)
- **Pitch score**: 104/110 (Trend Scout winner, data-driven v6)
- **Go version**: 1.21+
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

Total: ~5,350 lines of Go, **188 tests**.

## Dependencies (exactly three, plus pflag transitively from cobra)

- `gopkg.in/yaml.v3 v3.0.1` — YAML frontmatter
- `golang.org/x/mod v0.35.0` — semver primitives
- `github.com/spf13/cobra v1.10.2` — CLI framework

## Quality gate

- `go build ./...` — clean
- `go vet ./...` — clean
- `go test ./...` — all 188 tests pass
- `go test -race ./...` — race-detector clean
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
- The verify command will fail with a *resolver* error (missing dep) if
  the workspace manifest points at skills that have broken deps. This is
  deliberate — we don't want to say "drift OK" when the graph is broken.
  But it means `verify` has two failure modes that both return non-zero
  via `exitcode.Parse`. The exit-code contract is correctly distinct
  (drift=1, parse=2) but worth double-checking in a hostile read.
- `semver.incInt` rolls its own int parser to avoid a strconv import. It
  handles 0-padded inputs like `v01.02.03` (x/mod/semver rejects those
  upstream before we get here), but worth a look.
- The manifest writer uses yaml.v3 default encoding. Two-space indent is
  not explicitly configured; the default happens to match what we want,
  but this is a latent dependency on yaml.v3's default behavior.

No deliberate shortcuts, no skipped tests, no `//nolint` comments.

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
