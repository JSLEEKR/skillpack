# skillpack

![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?style=for-the-badge&logo=go&logoColor=white)
![Tests](https://img.shields.io/badge/tests-221-brightgreen?style=for-the-badge)
![License](https://img.shields.io/badge/license-MIT-blue?style=for-the-badge)
![Status](https://img.shields.io/badge/status-stable-success?style=for-the-badge)
![Binary](https://img.shields.io/badge/binary-%3C5MB-orange?style=for-the-badge)
![Deps](https://img.shields.io/badge/runtime%20deps-zero-blueviolet?style=for-the-badge)

> **Package manager, lockfile, and bundler for Claude Code / Cursor / AGENT.md agent skills.**
> Resolve dependencies. Pin sha256 hashes. Bundle and sign. Single Go binary, zero runtime deps.

```bash
skillpack init
skillpack add ./skills
skillpack install   # writes skillpack.lock
skillpack verify    # CI: exit 1 if anything drifted
skillpack bundle    # produces a deterministic *.skl tarball
skillpack sign --key priv.key mypack.skl
```

---

## Why This Exists

The agent-skill ecosystem is exploding. In the last six months, vendors and
the community have shipped:

- **Anthropic SKILL.md** — the canonical Claude Code skill format with YAML
  frontmatter
- **Cursor `.cursorrules`** — Cursor IDE rule files driving the editor's
  inline assistant
- **AGENT.md** — the cross-vendor proposal for a portable agent manifest
  (used by opencli, googleworkspace/cli, and others)
- **`skill.yaml`** — pure-YAML manifests for tooling-first skill catalogs

There are at least **six trending GitHub repositories** with combined stars
above **100,000** that cluster around this wedge:

- `googleworkspace/cli` (22K stars) — "40+ agent skills included"
- `antigravity-awesome-skills` (28K stars) — curated catalog of skills
- `awesome-claude-code` (33K stars) — the canonical "list of skills" repo
- `opencli` (8K stars, 632 stars/day) — runtime dispatcher reading AGENT.md
- `claude-code-ultimate-guide` — docs site for the ecosystem
- `awesome-openclaw-agents` — community-curated agent registry

And yet — **zero of these repositories solve the lifecycle problem**. They
all answer "what skills exist?" but none answer:

- How do I pin the exact version of a skill across machines?
- How do I detect when a skill on disk has drifted from what my CI expects?
- How do I bundle 20 skills into a single distributable artifact?
- How do I sign that artifact so my team can trust it?
- How do I declare that skill A depends on skill B at version `^1.2.0`?

`opencli` is a runtime dispatcher (different layer). `googleworkspace/cli`
ships its own bundle (vendor-locked). `awesome-claude-code` is a hand-curated
markdown list (no tooling). The lifecycle hole is wide open, and skillpack
fills it.

**skillpack is the npm/cargo/pip of agent skills** — except it works across
every vendor format from day one.

---

## What skillpack Does

| Layer | What it does |
|---|---|
| **Parse** | Read `SKILL.md`, `.cursorrules`, `AGENT.md`, and `skill.yaml`; normalize all four into a single canonical record |
| **Resolve** | Honor `requires:` semver constraints between skills; produce a deterministic install order via topological sort |
| **Hash** | Compute a content-addressed sha256 fingerprint per skill (LF-normalized, frontmatter-sorted, key-canonicalized) |
| **Lock** | Write `skillpack.lock` — a deterministic JSON file with no timestamps and stable ordering |
| **Bundle** | Produce a `*.skl` tarball (gzipped, fixed mtime, sorted file order, PAX format) — byte-identical across two runs |
| **Sign** | Detached ed25519 signatures over the bundle bytes |
| **Verify** | CI mode that exits non-zero on hash drift, version drift, or missing skills |

Every byte of output is deterministic. Two machines parsing the same source
files produce **byte-identical** lockfiles and tarballs. This is what makes
skillpack a real package manager and not a glorified zip script.

---

## Installation

### From source

```bash
go install github.com/JSLEEKR/skillpack/cmd/skillpack@latest
```

### Build manually

```bash
git clone https://github.com/JSLEEKR/skillpack.git
cd skillpack
go build -ldflags="-s -w" -o skillpack ./cmd/skillpack
```

The resulting binary is **under 5 MB** with zero runtime dependencies.
Drop it in your `$PATH` and you're done.

### Pre-built binaries

Download from the [Releases](https://github.com/JSLEEKR/skillpack/releases) page.

---

## Quick Start

### 1. Initialize a workspace

```bash
mkdir my-pack && cd my-pack
skillpack init --name my-pack
```

This creates a `skillpack.yaml` workspace manifest:

```yaml
name: my-pack
version: 0.1.0
skills:
  - ./skills
```

### 2. Add some skills

Drop a `SKILL.md` (or `.cursorrules`, `AGENT.md`, `skill.yaml`) under `./skills`:

```markdown
---
name: code-review
version: 1.2.0
description: Carefully review code changes for issues
license: MIT
author: jslee
tools:
  - git
  - bash
requires:
  - base-agent ^1.0.0
---

# Code Review Skill

When asked to review code, follow these steps:
1. ...
```

### 3. Resolve and install

```bash
$ skillpack resolve
Install order (2 skills):
  1. base-agent@1.0.0 [skill.md] — skills/base-agent/SKILL.md
  2. code-review@1.2.0 [skill.md] — skills/code-review/SKILL.md

$ skillpack install
skillpack: wrote skillpack.lock (2 skills)
```

The resulting `skillpack.lock`:

```json
{
  "version": 1,
  "generated_by": "skillpack",
  "skills": [
    {
      "name": "base-agent",
      "version": "1.0.0",
      "format": "skill.md",
      "hash": "sha256:b6a8...",
      "source": "skills/base-agent/SKILL.md"
    },
    {
      "name": "code-review",
      "version": "1.2.0",
      "format": "skill.md",
      "hash": "sha256:f3c0...",
      "source": "skills/code-review/SKILL.md",
      "requires": ["base-agent ^1.0.0"]
    }
  ]
}
```

### 4. Verify in CI

```bash
$ skillpack verify
skillpack: verify OK
```

Exit code 0. Wire it into your pipeline:

```yaml
# .github/workflows/ci.yml
- name: Verify skill lockfile
  run: skillpack verify
```

If any skill drifts (someone edited a body, bumped a version, deleted a file),
`verify` exits with code 1 and prints exactly what changed.

### 5. Bundle and sign

```bash
$ skillpack bundle -o my-pack.skl
skillpack: wrote my-pack.skl (4218 bytes, 2 skills)
  hash: sha256:5e9d...

$ skillpack keygen --priv priv.key --pub pub.key
$ skillpack sign --key priv.key my-pack.skl
skillpack: wrote signature my-pack.skl.sig

$ skillpack sign --verify --pubkey pub.key my-pack.skl
skillpack: signature OK for my-pack.skl
```

Distribute `my-pack.skl` + `my-pack.skl.sig` + `pub.key` and your users can
verify provenance with one command.

---

## Supported Formats

skillpack speaks all four of the de facto agent-skill manifest formats:

### SKILL.md (Anthropic)

```markdown
---
name: my-skill
version: 1.0.0
description: What this skill does
license: MIT
author: alice
tools:
  - bash
  - git
requires:
  - other-skill ^1.0.0
---

The body of the skill goes here.
```

### .cursorrules (Cursor)

```markdown
---
name: my-cursor-rules
version: 0.3.0
description: Code style for our team
globs:
  - "**/*.ts"
  - "**/*.tsx"
alwaysApply: true
---

Always use tabs.
Never use any.
Prefer named exports.
```

### AGENT.md (Cross-vendor)

```markdown
---
name: portable-bot
version: 2.0.0
description: A bot that works in any vendor's runtime
vendor: anthropic
models:
  - claude-3.5-sonnet
permissions:
  - filesystem
  - network
tools:
  - bash
---

This agent helps you...
```

### skill.yaml (Pure YAML)

```yaml
name: yaml-only
version: 1.0.0
description: A skill defined entirely in YAML
license: Apache-2.0
tools:
  - curl
  - jq
body: |
  This skill talks to APIs.
```

skillpack normalizes all four into the same canonical record, so you can mix
and match formats inside a single workspace and the lockfile will treat them
identically.

---

## Determinism Guarantees

Every byte of skillpack's output is deterministic. The following invariants
are tested:

| Invariant | Test |
|---|---|
| `skillpack install` produces byte-identical lockfile across two runs | `lockfile.TestMarshalDeterministic` |
| `skillpack bundle` produces byte-identical tarball across two runs | `bundle.TestBundleDeterministic` |
| Tarball is identical regardless of input skill order | `bundle.TestBundleDeterministicRegardlessOfInputOrder` |
| Hash is identical regardless of frontmatter key order | `hasher.TestHashFrontmatterOrderInsensitive` |
| Hash is identical regardless of tools/requires order | `hasher.TestHashToolsOrderInsensitive` |
| Hash is identical regardless of CRLF vs LF line endings | `parser.TestParseSkillMDCRLF` + canonical normalization |
| Hash is identical regardless of UTF-8 BOM presence | `parser.TestParseSkillMDBOM` |
| Resolver order is identical across runs (lexicographic tiebreak) | `resolver.TestResolveDeterministic` |

How we achieve this:

- **Lockfile**: `encoding/json` with sorted struct fields, manually sorted
  skills slice, manually sorted requires within each entry, LF line endings,
  trailing newline.
- **Tarball**: sorted file order, fixed `mtime` (1970-01-02 to dodge zero-mtime
  quirks), `uid/gid = 0`, `Uname/Gname = ""`, `tar.FormatPAX`, gzip with fixed
  header name.
- **Hashing**: canonical pre-image is line-oriented `key=value\n` form with
  sorted keys, body normalized to LF + single trailing newline.
- **No timestamps anywhere** in any hashable code path.

---

## Exit Codes

skillpack uses distinct exit codes so CI pipelines can branch on the failure
mode:

| Code | Meaning | When |
|---|---|---|
| `0` | OK | Operation succeeded |
| `1` | Drift | `verify` found a hash or version mismatch (expected failure mode) |
| `2` | Parse error | A skill file is malformed (YAML, frontmatter, manifest) |
| `3` | IO error | Filesystem, permission, or missing-file error |
| `4` | Internal error | An unexpected bug in skillpack itself |
| `5` | Usage error | Invalid CLI flags or missing required arguments |
| `6` | Security | `sign --verify` failed (tampered bundle or wrong key) — treat as a hard-fail, never a routine lock refresh |

CI example:

```bash
skillpack verify
case $? in
  0) echo "All skills clean" ;;
  1) echo "Drift detected — open a PR to update skillpack.lock" ; exit 1 ;;
  2) echo "Skill file is broken — block merge" ; exit 1 ;;
  6) echo "SIGNATURE TAMPER — do not merge, investigate" ; exit 1 ;;
  *) echo "skillpack itself failed — investigate" ; exit 1 ;;
esac
```

---

## Architecture

skillpack is intentionally small: ~5,300 lines of Go split across 13 internal
packages. Each package has a single responsibility and a clean interface.

```
cmd/skillpack/main.go         entry point (5 lines)

internal/
├── cli/                      cobra command tree
│   ├── root.go               root command + Execute
│   ├── init.go               skillpack init
│   ├── add.go                skillpack add
│   ├── resolve.go            skillpack resolve
│   ├── install.go            skillpack install
│   ├── verify.go             skillpack verify
│   ├── bundle.go             skillpack bundle
│   ├── sign.go               skillpack sign / keygen
│   └── lock.go               skillpack lock
├── workspace/                manifest + parser + resolver glue
├── manifest/                 skillpack.yaml read/write
├── parser/                   multi-format parser
│   ├── skillmd.go            SKILL.md
│   ├── cursorrules.go        .cursorrules
│   ├── agentmd.go            AGENT.md
│   ├── skillyaml.go          skill.yaml
│   └── helpers.go            normalizeRequires, dedupSorted
├── skill/                    canonical Skill record
├── semver/                   constraint matching (^/~/x/...)
├── resolver/                 topological sort with semver checks
├── hasher/                   sha256 content addressing
├── lockfile/                 deterministic JSON lockfile
├── bundle/                   deterministic tar.gz writer
├── signer/                   ed25519 detached signatures
├── verify/                   CI drift detection
└── exitcode/                 typed errors -> exit codes
```

Dependency direction always flows downward — `cli` depends on `workspace`,
`workspace` depends on `parser` + `resolver` + `manifest`, and so on. There
are no cycles, and the leaf packages (`skill`, `semver`, `exitcode`) have no
internal dependencies.

---

## Test Coverage

221 tests across all layers:

| Package | Tests | What it covers |
|---|---|---|
| `parser` | 32 | All four formats, CRLF, BOM, missing fields, bad YAML, requires (list/map), v-prefix versions |
| `cli` | 27 | init, add, resolve, install, verify (clean/drift), bundle, sign, keygen, lock, JSON output, error paths |
| `lockfile` | 19 | Roundtrip, sort order, LF only, trailing newline, missing/negative/future version, atomic write, duplicate-name rejection (Cycle K) |
| `semver` | 17 | Caret, tilde, comparators, x-ranges, BestMatch, normalize, edge cases |
| `hasher` | 17 | Determinism, frontmatter order, tools/requires order, body/version/name sensitivity, collision-resistance across comma/pipe/`=`/newline ambiguity |
| `signer` | 16 | Generate, sign, verify, tampered, wrong key, CRLF in key file, file roundtrip, trailing garbage, multi-line body |
| `bundle` | 16 | Determinism, multiple formats, header validation, path safety, list mode, tainted-archive hardening |
| `skill` | 16 | Canonical record validation, name rules (`.`/`..`/leading-dot/whitespace), constraint parsing, sorting |
| `resolver` | 14 | Linear chain, diamond, cycle, self-cycle, missing dep, version conflict, deterministic ordering, duplicates |
| `manifest` | 14 | Roundtrip, sort, missing fields, bad YAML, write/read, skills-path validation |
| `verify` | 12 | Clean, drift hash, drift version, missing, extra, sorted findings, parse error, JSON snake_case schema pin (Cycle J) |
| `exitcode` | 8 | Wrap/Classify, nil-safe, layered wrap preservation |
| `workspace` | 8 | Load happy, missing manifest, missing dep, recursive discover, dedup, ignore .git |
| `docsmeta` | 5 | Doc-accuracy meta-tests: ROUND_LOG/CHANGELOG/README test-count pins + badge pin + per-package table sum + meta-meta self-consistency (Cycle J, L) |

Run them yourself:

```bash
go test ./...                 # all green, ~5 seconds
go test -race ./...           # race detector clean
go vet ./...                  # vet clean
```

---

## Comparison with Existing Tools

| Tool | Layer | Format support | Lockfile | Hashing | Bundling | Signing | Status |
|---|---|---|---|---|---|---|---|
| **skillpack** | Lifecycle | All 4 | ✅ | ✅ sha256 | ✅ deterministic | ✅ ed25519 | This project |
| `opencli` | Runtime dispatch | AGENT.md | ❌ | ❌ | ❌ | ❌ | Different layer |
| `googleworkspace/cli` | Vendor bundle | Vendor-specific | ❌ | ❌ | Bundled at build | ❌ | Vendor-locked |
| `awesome-claude-code` | Curation | Markdown list | ❌ | ❌ | ❌ | ❌ | No tooling |
| `claude-code-ultimate-guide` | Docs | Docs | ❌ | ❌ | ❌ | ❌ | Docs site |
| npm/cargo/pip | Code packages | N/A | ✅ | ✅ | ✅ | Sometimes | Wrong domain |

skillpack is the **only** tool that closes the lifecycle loop for agent skills.

---

## Roadmap

Things that are explicitly out of scope for v1.0 (and may land in 1.x):

- **Federated registry** — v1.0 has no centralized registry. Skills come from
  local paths, file:// URIs, http(s):// URLs, and git URLs only. A federated
  index is V2 and only if traction warrants.
- **Skill audit** — security analysis of skill bundles (prompt injection,
  shell calls, oversized contexts). See the runner-up `skillaudit` proposal.
- **Skill catalog UI** — local index + search over installed skills.
- **Schema validation** — strict JSON Schema validation per format.
- **Lockfile resolution caching** — for very large workspaces.

---

## Contributing

Bug reports, feature ideas, and pull requests welcome at
[github.com/JSLEEKR/skillpack/issues](https://github.com/JSLEEKR/skillpack/issues).

Before opening a PR:

```bash
go test -race ./...    # must be green
go vet ./...           # must be clean
```

The Generator/Evaluator separation in our build pipeline means every change
goes through an independent eval pass before merging.

---

## License

MIT © 2026 JSLEEKR. See [LICENSE](./LICENSE) for the full text.

---

## Acknowledgements

- The Anthropic Claude Code team for shipping `SKILL.md` as a parseable format
- The Cursor team for `.cursorrules`
- The opencli, googleworkspace/cli, and awesome-claude-code communities for
  proving the demand for skill tooling
- `golang.org/x/mod/semver` for the rock-solid semver primitives
- `gopkg.in/yaml.v3` for YAML parsing
- `github.com/spf13/cobra` for the CLI framework

If skillpack saves your team a single deployment headache, that's payment enough.
