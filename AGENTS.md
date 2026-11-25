# DashBurn project instructions

DashBurn is a read-only terminal dashboard that imports the public Go libraries
of two sibling applications and shows Kimi Code quota beside OpenRouter credits.
It is the sibling of [Kimiburn](../kimiburn) and [Routeburn](../routeburn) and
adapts their Go, Charm, and release patterns.

## Product and credential boundaries

- DashBurn is read-only. It must not send inference requests, purchase credit,
  change quota, or create, rotate, or revoke a credential.
- DashBurn owns no credential of its own. It resolves the credentials the
  siblings already manage, in their documented order: environment variable, OS
  keyring, then a permission-restricted `0600` fallback file. It has no setup or
  logout command and never writes a credential, a cache, or a snapshot to disk.
- A fallback file that is group- or world-readable is refused rather than read.
  Route every user-visible error through `internal/credentials/redact.go`.
- Never expose credentials, authorization headers, raw API response bodies, or
  real account data in logs, errors, JSON, tests, fixtures, screenshots, or
  commits. Real account data may only appear in the rendered terminal.
- Tests are synthetic-only: hand-authored fixtures, fake keyrings, injected
  clocks, and fake sources. They must not reach the real network, keyring, user
  configuration, or a real credential.

## Architecture boundaries

- Module `github.com/acevedomiguel/dashburn`, binary built from
  `./cmd/dashburn`.
- `internal/provider` is the only package that imports the sibling libraries.
  It wraps `kimiburn.Fetch` and `routeburn.Fetch` and exposes one read-only
  source per provider; everything above it depends only on those interfaces.
- `internal/ui` renders pure frames and never fetches. `internal/app` holds the
  state machine, scheduling, and injected clocks. `internal/config` reads the
  siblings' configuration; it never writes it.
- Each provider is scheduled, refreshed, and failed independently. One
  provider's error must never blank or delay the other, and the last good
  snapshot stays visible with an error banner.
- DashBurn keeps no cache. Kimi's runway history lives in a process-lifetime
  `StoredState`; OpenRouter's previous snapshot is passed back to the library so
  an optional analytics failure degrades to a warning.
- Preserve missing-versus-zero: a value the provider did not report is never
  rendered as `0`. Kimi's monthly membership and monthly Code allocation are
  never added together.
- The two providers' figures are never totalled: Kimi reports ratios and
  OpenRouter reports US dollars.

## Terminal UI

- Minimum supported layout is 60x16; smaller terminals show a resize hint.
- Keep exact terminal height, no line overflow, a footer on the final row, and
  no ANSI escapes in no-color mode. Clip the footer escape-safely.
- Providers sit side by side at 96 content columns and above, and stack in one
  scrollable column below it.

## Baseline versions

- Go directive `1.26.0` in `go.mod`; CI also builds current stable Go.
- Charm v2 and supporting modules are pinned to the versions the siblings
  verified. Keep the `kimiburn` and `routeburn` dependencies on their `v1.x.y`
  lines: a `v2` tag on either module path would break the import until both
  sides migrate together.
- Commit messages are Conventional Commits; releases are derived from them, so
  subjects are lowercase and imperative.

## Git workflow

- ALWAYS work in a dedicated git worktree on a feature branch; never edit or
  commit directly on `main` in the primary checkout. Create worktrees with
  `git worktree add .worktrees/<branch> -b <branch>` and remove them after
  merging.
- Branch names follow Conventional Commit types, e.g. `feat/...`, `fix/...`,
  `docs/...`.
- Land changes through a squash-merged pull request; the PR title is the
  release commit subject, so keep it lowercase and imperative.

## Release automation

- `semantic-release` derives the version from Conventional Commits on `main`
  and drives `.goreleaser.yaml`. Do not create tags, edit the generated release
  sections of `CHANGELOG.md`, or hand-publish normal artefacts.
- `.goreleaser.yaml` intentionally publishes a Homebrew **cask** to
  `acevedomiguel/homebrew-dashburn`, not a formula. Keep generation under
  `homebrew_casks` and do not hand-edit the tap from this repository.
- The tap push is skipped when `HOMEBREW_TAP_TOKEN` is empty, so releases still
  succeed with only the default `GITHUB_TOKEN`.
- Preserve `CGO_ENABLED=0`, the declared target exclusions, version injection,
  SHA-256 checksums, and empty-token snapshot behavior.
- Preserve declarative `postflight_steps`, not deprecated `postflight`. The
  quarantine removal exists because binaries are ad-hoc signed; it is a
  documented Gatekeeper trade-off, with signing/notarization preferred.

## Verification

Run checks relevant to the changed area and report anything unavailable:

- Go changes: `make check`; before a PR also run `gofmt -l .` (no output),
  `go test -race ./...`, and `go build ./cmd/dashburn`.
- CLI/build changes: `make build` and `./dashburn --version` without loading
  real credentials.
- GoReleaser changes: `goreleaser check` and
  `goreleaser release --snapshot --clean` with `HOMEBREW_TAP_TOKEN`
  defined-but-empty, followed by `git diff --exit-code` because its hooks run
  `go mod tidy`.
- Release tooling: `npm ci && npm run check:release-notes`.
