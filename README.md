# redmine-cli

Agent-first, provider-neutral Redmine CLI for Codex and Claude Code.

`redmine-cli` is a standalone Go binary that exposes a small read-only
Redmine surface through a versioned machine contract. It keeps API tokens in
native macOS Keychain, requires an explicit named profile for every network
call, and embeds one canonical Agent Skill installable for either provider.

The initial external API surface is intentionally read-only. Auth and skill
commands change only local `redmine-cli` state.

## Design goals

- stdout is one stable JSON envelope whenever JSON mode is selected; non-TTY
  output defaults to JSON.
- process exit codes identify the caller's recovery action.
- profiles are arbitrary names such as `work` or `client-a`; company
  names, hosts, and environment variables are never built-in defaults.
- profile metadata contains only name and canonical Redmine base URL.
- API tokens never enter config files, URLs, logs, errors, command flags,
  environment fallback, skill files, or modeled output.
- Codex and Claude Code consume the same `SKILL.md` and generated command /
  contract references.

## Local references used

The first Redmine contract was verified read-only against:

- `trello_sync_bot/sync_innoco.py`: `X-Redmine-API-Key`,
  `GET /issues.json`, direct issue reads, `assigned_to_id`,
  journals, and attachments.
- `delivery_boy_fl/.gitlab-ci.yml` and `dgdapp/.gitlab-ci.yml`:
  the same header plus Redmine upload/project-file routes. Those write routes
  are deliberately not exposed by this MVP.
- `jira-cli` and `trello-cli`: package boundaries, native credential
  storage, JSON envelopes, exit codes, skill packaging, and source-building
  Homebrew policy.

`Innoco` is only the historical company-specific reference name. No
`INNOCO_*` variable, fixed status, project, URL, or user ID is part of this
CLI.

## Build

Go 1.25.14 and the macOS SDK are required for the supported credential backend.

```bash
go build -o ./bin/redmine-cli ./cmd/redmine-cli
./bin/redmine-cli version
./bin/redmine-cli contract
```

The Keychain backend calls Security.framework directly and never executes
`security`. A non-macOS or `CGO_ENABLED=0` build can run local
contract/version/skill commands, but network commands fail with the typed
`KEYCHAIN_UNSUPPORTED` authentication error.

To keep unsigned, source-built binaries usable across rebuilds without an
interactive Keychain prompt, saved items use an allow-any-application decrypt
ACL. This protects tokens at rest and keeps them out of files, process
arguments, and output, but it does not isolate a token from another process
already running as the same macOS user while that user's Keychain is unlocked.
Use a dedicated OS account when that threat model matters.

## Profiles

Each network invocation names its profile. There is no active or default
profile.

```bash
redmine-cli auth login \
  --profile work \
  --url https://redmine.example/redmine \
  --token-stdin
```

Token input is a single bounded line. Terminal input is hidden. Existing
metadata or an orphaned Keychain item requires `--yes` before stdin is
read. Login validates the URL, verifies `GET /users/current.json` through
a secret-omitting DTO, then commits credential and metadata transactionally.

```bash
redmine-cli auth list
redmine-cli auth status --profile work
redmine-cli auth status --profile work --check
redmine-cli auth logout --profile work --yes
```

`auth list` and plain `auth status` never read Keychain.
`--dry-run` login/logout previews metadata without token input, network,
or Keychain access.

## Read commands

```bash
redmine-cli me --profile work --fields id,login,name

redmine-cli projects list --profile work --limit 25
redmine-cli projects get delivery --profile work

redmine-cli issues list --profile work \
  --assigned-to me --status open \
  --sort updated_on:desc --limit 25 \
  --fields id,subject,project,status,assigned_to,updated_on

redmine-cli issues get 123 --profile work \
  --fields id,subject,status,description,journals,attachments
```

Collection limits are 1–100. Follow `meta.next_cursor` with the same
profile, filters, sorting, and includes. Cursors are opaque and bound to that
profile, base URL, resource, and query.

Issue lists support Redmine's documented `attachments` and
`relations` associations. Exact issue reads additionally support
`journals`, `watchers`, and `children`. Issue IDs are exact
positive integers; only project reads accept a textual identifier.

Redmine may return the current user's `api_key` from
`/users/current.json`. The field is structurally absent from the model and
cannot appear even under `-o raw`.

## Machine contract

```json
{"ok":true,"v":1,"data":{"id":123,"subject":"Example"},"meta":{"profile":"work","base_url":"https://redmine.example"}}
{"ok":false,"v":1,"error":{"code":"PROFILE_REQUIRED","message":"an explicit profile is required for every network command"},"hint":"re-run with --profile NAME"}
```

Run `redmine-cli contract` or read [docs/contract.md](docs/contract.md).
The full generated command surface is [docs/commands.md](docs/commands.md).
`-o raw` emits the modeled payload without envelope or projection; it is
not an upstream REST passthrough. Failures remain envelopes.

## Agent Skill

The canonical skill lives under `assets/skills/redmine/` and is embedded
in the binary.

```bash
redmine-cli skills install --provider all --scope user
redmine-cli skills install --provider codex --scope project --project-dir "$PWD"
redmine-cli skills uninstall --provider all --scope user --yes
```

The installer locks destinations, refuses symlink traversal and insecure
ancestors, records file hashes in a manifest, and uses compare-and-swap file
commits so unmanaged or concurrently changed content is preserved rather than
silently replaced. Uninstall requires `--yes` whenever owned files exist.

## Homebrew distribution

macOS distribution is source-built so CGO selects Security.framework and no
unsigned downloaded Darwin executable or cask is required. The Formula is named
`redmine-agent-cli` and installs `redmine-cli`.

This repository contains only a deterministic template. The source-only
release workflow publishes a checksum-pinned source asset after verifying an
annotated tag on `main`. Once its checksum is known, render a Formula locally:

```bash
go run ./tools/renderformula \
  --version 0.1.0 \
  --source-url https://github.com/abigotado/redmine-cli/releases/download/v0.1.0/redmine-cli-0.1.0.tar.gz \
  --sha256 <64-lowercase-hex> \
  --output /absolute/local/path/redmine-agent-cli.rb
```

The renderer accepts only the exact canonical release-asset URL and refuses
alternate owners, query strings, fragments, mismatched tags, invalid checksums,
relative paths, and overwrites. The generated Formula is macOS-only, builds
with `CGO_ENABLED=1`, `-mod=vendor`, and `-trimpath`, injects the version,
and stages every Go module as a separately checksummed Homebrew resource so
the build runs with `GOPROXY=off` and an empty module cache. It verifies the v1 contract and
Security.framework linkage, and rejects a binary containing
`/usr/bin/security`. Publishing a tag or updating a tap is a separate,
explicitly authorized operation and is not performed here.

See `RELEASE.md` for the source-only release contract, tag trust boundary,
partial-draft recovery, and destination checks.

## Validation

```bash
gofmt -l .
go vet ./...
go build ./...
go test -race ./...
go test -tags keychainintegration ./internal/auth
go generate ./...
```

The opt-in Keychain integration test builds two distinct helper binaries and
uses a fresh disposable Keychain only. It never opens the user's default
Keychain and stores only a synthetic non-secret sentinel.
