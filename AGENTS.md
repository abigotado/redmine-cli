# redmine-cli agent rules

`redmine-cli` is a single Go binary designed primarily for AI agents. Its CLI,
JSON envelope, exit codes, profile selection, and credential boundaries are a
public machine contract.

## Architecture

Dependency direction:

```text
cli -> {auth, profile, output, skills, redmine}
auth -> profile
{profile, skills} -> lockfile
skills -> assets
{auth, profile, output, skills, redmine} -> errx
```

`internal/redmine` must not import auth or profile. `internal/cli` must not
import `net/http`. `internal/errx` imports no other internal package.

## Contract and security

- Every network command requires an explicit `--profile`; there is no active or
  default profile.
- JSON mode writes exactly one envelope to stdout. Logs and prompts go only to
  stderr. Help, explicit text, and explicit raw output are exceptions.
- Never print, log, persist outside Keychain, or accept a Redmine API token as a
  flag or environment variable.
- macOS credentials use Security.framework directly. Never invoke `security`,
  another credential subprocess, or allow invisible Keychain UI on ordinary
  operations.
- Redmine redirects are refused. Response bodies are bounded and upstream error
  bodies never reach user-visible errors.
- The external command surface is read-only. Local auth and skill installation
  may mutate only their own state and honor `--dry-run`/`--yes` gates.
- `GET /users/current.json` may return `api_key`; only the allowlisted SafeUser
  DTO may cross the HTTP boundary.

## Go and tests

Follow `~/.agents/rules/go.md`. Use `context.Context` first for I/O, wrap errors,
close resources immediately with `defer`, avoid mutable globals, and use
table-driven tests. Tests use `httptest`, fake credential stores, and
`t.TempDir`; they never contact a real Redmine or the user's default Keychain.

Before finishing run `gofmt -l .`, `go vet ./...`, `go build ./...`,
`go test -race ./...`, and `go generate ./...` followed by a clean generated
diff check.
