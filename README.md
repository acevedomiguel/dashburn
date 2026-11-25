# DashBurn

**One read-only terminal dashboard for Kimi Code quota and OpenRouter credits,
side by side.**

DashBurn imports the [Kimiburn](https://github.com/acevedomiguel/kimiburn) and
[RouteBurn](https://github.com/acevedomiguel/routeburn) Go libraries, reads the
credentials those applications already store, and renders both providers on a
single dense screen. It keeps no cache, sends no inference traffic, and never
rotates or revokes a credential.

> DashBurn only reads data. It talks to the Kimi Code and OpenRouter read-only
> endpoints, stores nothing on disk, and never invents a value the provider did
> not report.

## What it does

- Renders both providers in one frame: Kimi Code on the left, OpenRouter on the
  right on a wide terminal, stacked in a single scrollable column on a narrow
  one.
- Refreshes each provider on **its own** schedule (30 seconds by default),
  measured from request completion. One provider being slow, unauthenticated, or
  down never delays or blanks the other.
- Reads the credentials `kimiburn` and `routeburn` already manage — environment
  variables, the OS keyring, then the permission-restricted fallback files —
  without copying or storing its own.
- Honours each provider's existing configuration (`base_url`, `refresh_interval`,
  `keyring`, `theme`, `color`) and the XDG paths the two applications use.
- Keeps Kimi Code's local runway history in memory for the current process, so
  the runway and usage sparkline improve across reads without any cache file.
- Shows one freshness stamp, `updated: HH:MM`, pinned to the right of the title
  row: minute resolution, the most recent successful read across both providers,
  and no per-provider countdown.

### Kimi Code panels

The 5-hour quota and monthly membership always render; the Monthly Code
allocation and the legacy 7-day quota appear only when the API reports them.
Each panel shows the remaining meter, the now-to-reset timeline, the reset
countdown, the locally estimated runway and burn, and a sparkline of the
readings retained this process. The monthly Code allocation is part of the
shared membership and is never added to it.

### OpenRouter panels

Credits and the last-30-day totals, the prompt/completion/cached/reasoning token
split, the cost by model, and a scrollable one-row-per-UTC-day table with a
pinned total. Runway is the balance divided by the average daily spend over the
previous 30 complete UTC days.

## Requirements

- Go 1.26 or newer to build from source.
- A **Kimi Code** token and/or an **OpenRouter management key**, already stored
  by the sibling applications. DashBurn needs no credential of its own:

  ```sh
  kimiburn setup     # stores the Kimi Code token
  routeburn setup    # stores the OpenRouter management key
  ```

  DashBurn runs with either provider configured; the other shows a short
  "not configured" hint and everything else keeps working.

## Installation

### Homebrew (macOS and Linux)

```bash
brew tap acevedomiguel/dashburn
brew install --cask dashburn
dashburn --version
```

Upgrades arrive through the same tap:

```bash
brew update
brew upgrade --cask dashburn
```

The cask downloads a prebuilt binary rather than compiling from source. That
binary is not notarized by Apple, so the cask clears the macOS quarantine flag
after staging; without that step Gatekeeper would kill the binary on first run.
The tap is a third-party tap, so Homebrew asks you to trust it once:

```bash
brew trust acevedomiguel/dashburn
```

### Go

```bash
go install github.com/acevedomiguel/dashburn/cmd/dashburn@latest
```

### Prebuilt binaries

Every release publishes Linux, macOS, and Windows archives plus a
`checksums.txt` file on the GitHub Releases page. Archives contain the binary,
this README, and the LICENSE; Linux and macOS use `.tar.gz`, Windows uses
`.zip`. Verify a download with:

```bash
sha256sum -c checksums.txt --ignore-missing
```

### From source

```sh
git clone https://github.com/acevedomiguel/dashburn.git
cd dashburn
make build
./dashburn --version
```

## Use

```sh
dashburn                    # live unified dashboard
dashburn --refresh 45s      # override both providers' refresh interval
dashburn --refresh 0        # manual refresh only
dashburn --no-color
dashburn --help
dashburn --version
```

### Keys

| Key | Action |
| --- | --- |
| `r` | Refresh both providers now (each is throttled to one attempt per 30 seconds) |
| `↑` `↓` / `k` `j` | Scroll the dashboard, or the focused daily table |
| `PgUp` `PgDn` | Scroll a page |
| `Home` `End` | Jump to the top or bottom |
| `Tab` | Focus the OpenRouter daily table so the scroll keys drive it |
| `?` | Toggle the Help overlay (panels, keys, and current warnings) |
| `q` / `Esc` | Quit (`Esc` and `?` close the Help overlay first) |
| `Ctrl+C` | Quit from anywhere |

## Configuration

DashBurn reads the sibling configuration files and never writes them:

| Platform | Kimi Code | OpenRouter |
| --- | --- | --- |
| macOS | `~/Library/Application Support/kimiburn/config.yaml` | `~/Library/Application Support/routeburn/config.yaml` |
| Linux | `${XDG_CONFIG_HOME:-~/.config}/kimiburn/config.yaml` | `${XDG_CONFIG_HOME:-~/.config}/routeburn/config.yaml` |

The environment overrides the siblings honour also apply here:
`KIMIBURN_BASE_URL`, `KIMIBURN_REFRESH_INTERVAL`, `KIMIBURN_KEYRING`,
`KIMIBURN_THEME`, `KIMIBURN_COLOR`, `ROUTEBURN_REFRESH_INTERVAL`,
`ROUTEBURN_KEYRING`, `ROUTEBURN_THEME`, and `ROUTEBURN_COLOR`. Command-line
flags win over everything. `--refresh 0` disables automatic refresh entirely.

Credential resolution matches the sibling applications:

| Provider | Environment | Keyring | Fallback file |
| --- | --- | --- | --- |
| Kimi Code | `KIMIBURN_TOKEN` | `kimiburn` / `token` | `kimiburn/token` |
| OpenRouter management | `ROUTEBURN_MANAGEMENT_KEY` | `routeburn` / `management` | `routeburn/management-token` |
| OpenRouter API key (optional) | `ROUTEBURN_API_KEY`, `OPENROUTER_API_KEY` | `routeburn` / `api-key` | `routeburn/api-key` |

A fallback file that is group- or world-readable is refused rather than read.

## Privacy and security

- No account data leaves the machine: DashBurn talks only to the Kimi Code and
  OpenRouter read-only endpoints and writes nothing to disk.
- Nothing is cached, so there is no on-disk copy of your usage or credentials.
- Authorization headers and secrets are never logged, rendered, or included in
  an error; both providers' error text is redacted before display.
- `--no-color` and `NO_COLOR` produce byte-identical, escape-free output.

## Development

```sh
make check      # gofmt + go vet + go test -race
make build      # binary with version metadata
```

Tests are synthetic-only: hand-authored fixtures and fake keyrings, clocks, and
sources. They never touch the network, the real keyring, or your real
configuration.

## Releases

Releases are driven by squash-merged Conventional Commits on `main`; the pull
request title is the release commit subject. `semantic-release` derives the
version from those commits, and GoReleaser publishes the archives, the
checksums, the GitHub release, and the Homebrew cask in
[`acevedomiguel/homebrew-dashburn`](https://github.com/acevedomiguel/homebrew-dashburn).

Nothing is versioned by hand: do not create tags, edit generated release
sections of `CHANGELOG.md`, or hand-publish an artefact. The tap push is
skipped when `HOMEBREW_TAP_TOKEN` is absent, so a plain release still succeeds
with only the default `GITHUB_TOKEN`.

Local checks:

```sh
goreleaser check
goreleaser release --snapshot --clean   # library-free snapshot build
npm ci && npm run check:release-notes   # changelog rendering guard
```

## Built on

DashBurn depends on the public Go libraries that Kimiburn and RouteBurn publish
from their module roots — `github.com/acevedomiguel/kimiburn` and
`github.com/acevedomiguel/routeburn`. It reads `kimiburn.Snapshot` and
`routeburn.Snapshot` values and renders them itself; the interactive models and
credential stores of both applications stay internal. Keep both dependencies on
their `v1.x.y` lines (a `v2` tag would require a coordinated module-path
migration).

## License

MIT. See [LICENSE](LICENSE).
