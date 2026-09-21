# Step 08 - Security SAST Analysis

- `PROJECT_DETECTION_RESULTS`: `go@.`
- Language and scan scope: Go production and test code under `cmd/`, `internal/`, `assets/`, and `tools/`; release-boundary shell and GitHub Actions under `tools/release/` and `.github/workflows/` were also triaged for command and path injection.
- Method: repository-wide `rg` pattern scans followed by manual source/sink review. `go vet ./...` was run with isolated temporary Go caches and exited successfully; its only output was compiler deprecation warnings for macOS Security.framework APIs.
- Credential files and credential contents were not read.

## Required category results

| Category | Confirmed findings | Severity | Sample finding |
|---|---:|---|---|
| SQL injection | 0 | None | N/A |
| XSS | 0 | None | N/A |
| Path traversal | 0 | None | N/A |
| Eval/code injection | 0 | None | N/A |

### SQL injection: 0 findings

- No `database/sql` import, SQL driver, SQL statement construction, or database query/exec sink exists in the module (`go.mod` has no database dependency).
- The 13 broad `Query` pattern matches are URL-query operations, not SQL. Representative examples are `internal/redmine/resources.go:41-55` and `internal/redmine/resources.go:78-130`, which build `url.Values` after validating paging, numeric IDs, filters, sorting, and includes.
- Redmine requests pass those values as typed URL query data (`internal/redmine/client.go:174-179`); no string-built SQL sink exists.

### XSS: 0 findings

- No `html/template`, raw HTML trust type, `innerHTML`, `document.write`, or equivalent browser sink was found.
- This is a machine-oriented CLI with JSON/text output and no browser-rendering surface, so XSS is not applicable to the detected project.

### Path traversal: 0 findings

All filesystem candidates were reviewed; no request-derived or manifest-derived path can escape its authorized root:

- Explicit `--dest` is an intentional operator-selected filesystem root. It is normalized to an absolute path and must already be a non-symlink directory (`internal/skills/skills.go:247-275`).
- Untrusted manifest paths reject empty, absolute, and `..` components, then verify the final `filepath.Rel` remains below the skill root (`internal/skills/skills.go:769-784`).
- Reads are root-relative through `os.OpenRoot`, reject symlinks/non-regular files, re-stat the opened handle, and enforce byte limits (`internal/skills/skills.go:787-848`). Writes repeat containment/symlink checks and use same-directory temporary files plus compare-and-swap commit logic (`internal/skills/skills.go:922-946`, `internal/skills/commit_unix.go:17-83`, `internal/skills/commit_windows.go:12-43`).
- The complete target ancestry is checked for symlinks and unsafe shared-writable directories before mutation (`internal/skills/skills.go:978-1044`).
- `tools/renderformula` accepts an explicit absolute output path by design and uses `O_EXCL`, preventing overwrite of an existing path (`tools/renderformula/main.go:35-68`). Its template input is a fixed repository-relative file.
- Release asset names derive only from strict stable SemVer; commits require a full lowercase SHA, output variables are quoted, archive layout is allowlisted, and publish revalidates exact regular-file names (`tools/release/create-source-bundle.sh:9-57`, `tools/release/create-source-bundle.sh:89-105`, `.github/workflows/release.yml:151-176`).

### Eval/code injection: 0 findings

- No production import of `os/exec`, `syscall.Exec`/`ForkExec`, `os.StartProcess`, `plugin.Open`, dynamic evaluator, `eval`, `bash -c`, or `sh -c` was found.
- Five `exec.Command`/`exec.CommandContext` candidates are test-only: two architecture tests invoke the fixed `go` executable with argument arrays (`internal/arch/arch_test.go:16`, `internal/arch/arch_test.go:25`), and three disposable-Keychain integration-test calls invoke Go-built helper paths with separate arguments (`internal/auth/keychain_integration_darwin_test.go:25`, `internal/auth/keychain_integration_darwin_test.go:48`, `internal/auth/keychain_integration_darwin_test.go:57`). None uses a shell or interpolated command string.
- Release workflow and shell commands pass variables as quoted arguments. Tag and commit values crossing the release trust boundary are restricted before use (`tools/release/create-source-bundle.sh:13-38`, `.github/workflows/release.yml:75-97`).

## Relevant Go risk triage

- `unsafe` is confined to Security.framework CGo boundaries and the opt-in disposable-Keychain integration helper. Credential reads verify the CoreFoundation type and enforce `0 < length <= MaxTokenBytes` before copying into Go memory (`internal/auth/keychain_darwin.go:191-213`); writes validate the credential first and pass explicit lengths (`internal/auth/keychain_darwin.go:219-247`). CoreFoundation references have paired releases.
- `go vet ./...` reported no Go analyzer finding. C compilation emitted deprecation warnings for legacy Security.framework Keychain/ACL APIs in `internal/auth/keychain_darwin.go`; this is an informational compatibility/maintenance signal, not evidence of SQL injection, path traversal, code injection, memory corruption, or credential disclosure.
- No `panic` on request-controlled data, production subprocess execution, runtime plugin loading, or mutable global security state was identified by this step.

## Consolidated SAST findings

- MEDIUM findings: 0
- LOW findings: 0
- Informational observations: 1 (deprecated macOS Security.framework API surface; no demonstrated vulnerability in this scan)
- SAST result: no release-blocking finding from the required categories.
