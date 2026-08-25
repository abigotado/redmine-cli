# Security Audit Report

## 1. Security Scoring Breakdown

- Sensitive File Protection: 100/100 (Strong), weight 0.25
- Secret Detection: 100/100 (Strong), weight 0.30
- Dependency Security: 100/100 (Strong), weight 0.20
- Supply Chain Integrity: 100/100 (Strong), weight 0.10
- Security Automation & CI/CD: 40/100 (Critical), weight 0.15
- Overall Score: 91/100 (Strong)
- Formula: round(100*0.25 + 100*0.30 + 100*0.20 + 100*0.10 + 40*0.15) = 91
- Security Posture: Secure

## 2. Executive Summary

- Description: Security analysis of the provider-neutral `redmine-cli` Go application, its macOS Keychain credential boundary, read-only Redmine HTTP client, Agent Skill installer, CI, and Homebrew template.
- Overall Score: 91/100 (Strong)
- Top Findings:
  - [MEDIUM]: The source-built Keychain ACL permits any process running as the same macOS user to decrypt a saved item while that user's Keychain is unlocked. This is a documented non-interactive automation tradeoff.
  - [LOW]: Trivy is not installed, so the optional filesystem scan was skipped.
  - [LOW]: No automated dependency-update bot or pre-commit secret hook is configured.
  - [LOW]: Two direct dependencies and four transitive modules have newer releases, with no reachable vulnerability detected.
  - [LOW]: Destination tag protection and immutable-release settings cannot be verified before a remote repository exists.
- Priority Recommendations:
  1. Use a dedicated macOS account when local same-user process isolation is required.
  2. Add a filesystem or secret scanner to CI before publication if the repository's threat model warrants it.
  3. Review compatible dependency updates before the first tagged release.
- Previous: 91/100, Change: +0 (unchanged)

## 3. Security Automation & CI/CD

- Description: Evaluates automated vulnerability, checksum, pull-request, secret, and filesystem scanning gates.
- Score: 40/100 (Critical)
- Score Breakdown:
  - Base: 0
  - CI vulnerability scanning: +20
  - CI runs on pull requests: +10
  - Lock-file checksum validation in CI: +10
  - Final: 40/100 (Critical)
- Key Findings:
  - [LOW]: Pinned govulncheck, `go mod verify`, vet, build, race tests, Windows-native tests, generation checks, an offline Homebrew dependency build, and a disposable cross-binary Keychain test are present in CI.
  - [LOW]: The source-only release workflow isolates write permission to a no-checkout publication job and binds its bundle to both tag-object and peeled-commit SHAs.
  - [LOW]: No dependency-update bot, pre-commit secret hook, or Trivy job is configured.
  - [LOW]: Destination tag protection and immutable-release settings remain unverified until a remote repository exists.
- Evidence:
  - `.github/workflows/go.yml:3-50`
  - `.github/workflows/release.yml:15-278`
  - `reports/.artifacts/step_05_security_dependency_audit.md`
  - `reports/.artifacts/step_07_security_trivy.md`
- Risks:
  - A newly disclosed dependency or committed secret may wait for a pull request or manual scan before detection.
  - Repository-side tag or release protections cannot contribute until the confirmed destination is configured.
- Recommendations:
  1. Add a dependency-update bot after repository publication policy is decided.
  2. Consider a redacted Gitleaks CI job and Trivy filesystem job before publication.
  3. Configure protected `v*` tags and immutable releases, when available, before pushing the first release tag.

## 4. Secret Detection

- Description: Evaluates hardcoded secret patterns and Gitleaks results for the working tree and available history.
- Score: 100/100 (Strong)
- Score Breakdown:
  - Base: 100
  - HIGH findings: 0
  - MEDIUM findings: 0
  - LOW secret-pattern findings: 0
  - Git-history findings: 0
  - Final: 100/100 (Strong)
- Key Findings:
  - [LOW]: No hardcoded secret pattern or Gitleaks finding was detected.
  - [LOW]: Gitleaks found no secret across the working tree or committed release-candidate history.
- Evidence:
  - `reports/.artifacts/step_03_security_secret_patterns.md`
  - `reports/.artifacts/step_04_security_gitleaks.md`
  - `internal/redmine/models.go:26-44`
- Risks:
  - Future commits can introduce secrets even though the initial working tree is clean.
- Recommendations:
  1. Re-run Gitleaks before any publication or release.
  2. Keep token-bearing values excluded from diagnostics and model DTOs.

## 5. Sensitive File Protection

- Description: Evaluates credential-file presence, tracking state, and ignore coverage.
- Score: 100/100 (Strong)
- Score Breakdown:
  - Base: 100
  - Tracked `.env` files: 0
  - Tracked private keys or certificates: 0
  - Missing environment ignore pattern: 0
  - Tracked cloud credential files: 0
  - Final: 100/100 (Strong)
- Key Findings:
  - [LOW]: No credential-bearing file exists in the working tree.
  - [LOW]: `.gitignore` explicitly covers environment files and common private-key formats.
- Evidence:
  - `.gitignore:6-12`
  - `reports/.artifacts/step_02_security_file_analysis.md`
  - `internal/profile/profile.go:37-41`
- Risks:
  - A user can still place an ignored credential file locally; ignore coverage prevents tracking but does not secure that file.
- Recommendations:
  1. Continue storing Redmine tokens only in the native credential backend.
  2. Keep profile metadata limited to name and normalized base URL.

## 6. Dependency Security

- Description: Evaluates reachable vulnerabilities, module age, deprecations, and checksum integrity.
- Score: 100/100 (Strong)
- Score Breakdown:
  - Base: 100
  - Critical, high, medium, or low reachable CVEs: 0
  - More than five outdated direct dependencies: 0
  - Deprecated packages: 0
  - Verified Go checksum lock file: +5
  - Unclamped score: 105
  - Final: 100/100 (Strong)
- Key Findings:
  - [LOW]: Govulncheck reports no reachable vulnerability after `golang.org/x/sys` was upgraded to v0.44.0.
  - [LOW]: Two direct dependencies and four transitive modules have newer releases.
- Dependency Age Analysis:
  - Outdated direct count: 2
  - Deprecated count: 0
  - Top outdated direct dependencies: `golang.org/x/sys` v0.44.0 -> v0.47.0; `golang.org/x/term` v0.32.0 -> v0.45.0.
  - Additional transitive updates: `go-md2man/v2`, `pflag`, `yaml/v3`, and `check.v1`.
  - Deprecated packages: none detected.
- Evidence:
  - `go.mod:3-12`
  - `go.sum`
  - `reports/.artifacts/step_05_security_dependency_audit.md`
  - `reports/.artifacts/step_06_security_dependency_age.md`
- Risks:
  - Delayed minor updates can accumulate compatibility and maintenance cost even without a current CVE.
- Recommendations:
  1. Re-run tests and govulncheck when updating `x/sys` or `x/term` because they touch the credential and filesystem boundaries.
  2. Avoid dependency churn immediately before a release unless it fixes a relevant defect or advisory.

## 7. Supply Chain Integrity

- Description: Evaluates dependency sources, local replacements, module hashes, and build reproducibility controls.
- Score: 100/100 (Strong)
- Score Breakdown:
  - Base: 100
  - Git-sourced dependencies: 0
  - Path-based or replaced dependencies: 0
  - Missing integrity hashes: 0
  - Unknown registry dependencies: 0
  - Official module sources: +10
  - Verified checksums: +5
  - Unclamped score: 115
  - Final: 100/100 (Strong)
- Key Findings:
  - [LOW]: Go dependencies resolve through ordinary module paths with `go.sum` verification and no local replacement.
  - [LOW]: The Homebrew renderer requires the exact canonical release source URL and lowercase SHA-256.
  - [LOW]: Every Go module used by the Formula is a separately checksum-pinned resource, and an empty-module-cache test builds with `GOPROXY=off`, `GOSUMDB=off`, and vendoring enforced.
  - [LOW]: Third-party GitHub Actions are pinned to full commit SHAs with their major tags retained as comments.
  - [LOW]: The source archive is regenerated twice, checksummed with an exact manifest, and bound to a live re-peeled annotated tag before publication.
- Evidence:
  - `go.mod:1-12`
  - `go.sum`
  - `tools/renderformula/main.go:14-61`
  - `packaging/homebrew/redmine-agent-cli.rb.tmpl`
  - `packaging/homebrew/resources.tsv`
  - `tools/release/test-homebrew-offline.sh`
  - `tools/release/create-source-bundle.sh:13-148`
  - `.github/workflows/release.yml:71-278`
- Risks:
  - Future runner-image and pinned-tool version drift remain external supply-chain considerations.
- Recommendations:
  1. Verify the release source asset checksum independently before rendering a Formula.
  2. Review and update pinned action SHAs deliberately.
  3. Re-run generated-reference and Formula dry-run checks before release handoff.

## 8. Consolidated Findings by Severity

- [HIGH]: No HIGH severity findings.
- [MEDIUM]: Source-built Keychain items use an allow-any-application decrypt ACL. Source: `internal/auth/keychain_darwin.go:56-80`, documented in `SECURITY.md`.
- [LOW]: Trivy is unavailable, so optional filesystem scanning was skipped. Source: `reports/.artifacts/step_07_security_trivy.md`.
- [LOW]: No automated dependency-update bot or pre-commit secret hook is configured. Source: `reports/.artifacts/step_05_security_dependency_audit.md`.
- [LOW]: Two direct and four transitive modules have newer releases without a reachable vulnerability. Source: `reports/.artifacts/step_06_security_dependency_age.md`.
- [LOW]: Destination tag protection and immutable-release settings are not yet verifiable. Source: `RELEASE.md`.

## 9. Remediation Priority Matrix

1. [MEDIUM]: Use a dedicated OS account where same-user process isolation matters. Effort: Medium. Impact: High.
2. [LOW]: Add redacted secret and filesystem scanning to CI before publication if required. Effort: Low. Impact: Medium.
3. [LOW]: Review compatible dependency updates before the first tag. Effort: Low. Impact: Low.
4. [LOW]: Enable protected `v*` tags and immutable releases at the confirmed destination when supported. Effort: Low. Impact: Medium.

Independent publication review also identified and verified remediation of three
release blockers: Windows cleanup now atomically quarantines the entry and
hashes/deletes the same open handle; Homebrew builds from checksum-pinned module
resources with networking disabled; and the Agent Skill treats all Redmine
content as untrusted data rather than instructions. Its final cycle identified
one additional Windows blocker: the profile registry compared synthesized
Windows mode bits to POSIX `0700/0600` values. That policy is now platform
specific, retains strict Unix modes, accepts only regular files/directories on
Windows, avoids unsupported directory fsync there, and has build-tagged tests.

## 10. Gemini AI Analysis

- Status: Skipped
- Gemini CLI was not installed. Authentication was not inspected and no extension was installed.
- External AI source analysis was outside the authorized review boundary for this task.
- Local static, dependency, credential-boundary, and test reviews were used instead.

## 11. Project Detection Results

- Detected project type: Go
- Project path: `go@.`
- Framework/runtime: Go 1.25.0 module; local validation used Go 1.27.0 on macOS arm64.
- Package manager: Go modules
- Repository structure: single command-line application
- Source files scanned: `.go` production and test files plus shell, workflow, Formula, Markdown, and embedded skill assets.
- Gemini AI analysis: skipped

## 12. Appendix: Evidence Index

- Sensitive files: `.gitignore`, `reports/.artifacts/step_02_security_file_analysis.md`
- Secret patterns: `reports/.artifacts/step_03_security_secret_patterns.md`, `reports/.artifacts/step_04_security_gitleaks.md`
- Dependencies: `go.mod`, `go.sum`, `reports/.artifacts/step_05_security_dependency_audit.md`
- Dependency age: `reports/.artifacts/step_06_security_dependency_age.md`
- SAST: `reports/.artifacts/step_08_security_sast.md`
- Credential boundary: `internal/auth/keychain_darwin.go`, `internal/auth/transaction.go`, `SECURITY.md`
- Profile filesystem boundary: `internal/profile/registry.go`, `internal/profile/registry_permissions_unix.go`, `internal/profile/registry_permissions_windows.go`
- HTTP boundary: `internal/redmine/client.go`, `internal/redmine/models.go`
- Automation: `.github/workflows/go.yml`
- Release supply chain: `.github/workflows/release.yml`, `tools/release/`, `RELEASE.md`

## 13. Scan Metadata

- Scan date: 2026-08-25T20:42:28Z
- Project path: `/Users/Abigotado/StudioProjects/redmine-cli`
- Project type: Go
- Tools used: Go test, race detector, vet, gofmt, go mod verify, govulncheck, Gitleaks, actionlint, ShellCheck, ripgrep, Ruby syntax check, skill quick validator
- Gemini AI: skipped
- Total findings: 5 (0 high, 1 medium, 4 low)
- Generated by: local security-audit skill
- Skill: security-audit

---
Generated by: Somnio CLI vunknown
Skill: security-audit
Date: 2026-08-25
Somnio AI Tools: https://github.com/somnio-software/somnio-ai-tools
