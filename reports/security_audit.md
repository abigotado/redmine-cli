# Security Audit Report

## 1. Security Scoring Breakdown

- Sensitive File Protection: 100/100 (Strong), weight 0.25
- Secret Detection: 100/100 (Strong), weight 0.30
- Dependency Security: 95/100 (Strong), weight 0.20
- Supply Chain Integrity: 100/100 (Strong), weight 0.10
- Security Automation & CI/CD: 40/100 (Critical), weight 0.15
- Overall Score: 90/100 (Strong)
- Formula: round(100*0.25 + 100*0.30 + 95*0.20 + 100*0.10 + 40*0.15) = 90
- Security Posture: Secure

## 2. Executive Summary

- Description: Comprehensive security analysis of the `redmine-cli` Go command-line application, including its macOS Keychain credential boundary, read-only Redmine client, local profile registry, skill installer, CI, and release supply chain.
- Overall Score: 90/100 (Strong)
- Previous: 91/100, Change: -1 (declining)
- Top Findings:
  - [MEDIUM]: The source-built Keychain ACL permits any process running as the same macOS user to decrypt a saved item while that user's Keychain is unlocked. This is a documented non-interactive automation tradeoff.
  - [LOW]: Trivy is not installed, so the optional second filesystem scan was skipped.
  - [LOW]: No automated dependency-update bot or pre-commit security hook is configured.
  - [LOW]: Six resolved modules have newer releases, although the pinned Go-native vulnerability scan found no reachable vulnerability.
  - [LOW]: The destination has no repository rulesets or `main` branch protection, and immutable releases are not enabled or available.
- Verified release-fix controls:
  - The selected profile is checked before per-profile lock creation and re-read while the lock is held; client construction uses the refreshed URL, and concurrent profile deletion stops before credential loading.
  - Invalid issue-list filters, malformed pagination cursors, and missing profiles fail locally before a Keychain credential read or network-client construction.
  - Oversized numeric `Retry-After` values are clamped without duration overflow, and an advertised delay beyond the request deadline returns the structured `RATE_LIMITED` error without sleeping.
- Priority Recommendations:
  1. Use a dedicated macOS account where isolation from other same-user processes is required.
  2. Add an automated dependency-update service and a redacted secret or filesystem scanner if the publication threat model requires them.
  3. Review the three outdated direct modules, then regenerate and verify release dependency pins.
  4. Configure protected `main` and `v*` refs before merging or tagging this release; enable immutable releases when the repository plan supports them.

## 3. Security Automation & CI/CD

- Description: Evaluates automated dependency maintenance, vulnerability scanning, pull-request gates, secret hooks, checksum validation, and additional scanners.
- Score: 40/100 (Critical)
- Score Breakdown:
  - Base: 0
  - CI vulnerability scanning: +20
  - CI runs on pull requests: +10
  - Lock-file checksum validation in CI: +10
  - Dependabot or Renovate: +0
  - Snyk: +0
  - Pre-commit security hooks: +0
  - Additional scanner: +0
  - Final: 40/100 (Critical)
- Key Findings:
  - [LOW]: Both CI workflows run pinned `govulncheck@v1.1.4` and `go mod verify`; the primary workflow runs for pull requests and pushes to `main`.
  - [LOW]: No Dependabot, Renovate, Snyk, or pre-commit security hook is configured.
  - [LOW]: Trivy is not installed, so no additional-scanner bonus applies.
  - [LOW]: Third-party workflow actions are pinned to full commit SHAs.
- Evidence:
  - `.github/workflows/go.yml:3-50`
  - `.github/workflows/release.yml:15-57`
  - `reports/.artifacts/step_05_security_dependency_audit.md`
  - `reports/.artifacts/step_07_security_trivy.md`
- Risks:
  - Dependency and secret regressions can wait for a pull request, a release run, or a manual scan before detection.
  - A single Go-native vulnerability scanner provides less ecosystem breadth than an independent filesystem scanner.
- Recommendations:
  1. Configure Dependabot or Renovate for Go modules and pinned GitHub Actions.
  2. Add a redacted Gitleaks CI job or equivalent pre-commit hook.
  3. Add Trivy only if an independent filesystem and configuration scan is required by the release threat model.

## 4. Dependency Security

- Description: Evaluates reachable vulnerabilities, module age, deprecations, and checksum-backed resolution.
- Score: 95/100 (Strong)
- Score Breakdown:
  - Base: 100
  - Critical CVEs: 0
  - High CVEs: 0
  - Medium CVEs: 0
  - Low CVEs: 0
  - More than five outdated dependencies: -10
  - Deprecated packages: 0
  - Verified checksum lock file: +5
  - Final: 95/100 (Strong)
- Key Findings:
  - [LOW]: Pinned `govulncheck@v1.1.4` completed against the release toolchain target and found no reachable vulnerability.
  - [LOW]: Six of ten external modules have newer releases: three direct and three indirect.
  - [LOW]: No resolved module is deprecated or retracted.
- Dependency Age Analysis:
  - Outdated count: 6 total, consisting of 3 direct and 3 indirect dependencies.
  - Deprecated count: 0.
  - Top outdated dependencies:
    1. `github.com/spf13/pflag` v1.0.9 -> v1.0.10
    2. `golang.org/x/sys` v0.44.0 -> v0.47.0
    3. `golang.org/x/term` v0.32.0 -> v0.45.0
    4. `github.com/cpuguy83/go-md2man/v2` v2.0.6 -> v2.0.7
    5. `go.yaml.in/yaml/v3` v3.0.4 -> v3.0.5
    6. `gopkg.in/check.v1` v0.0.0-20161208181325-20d25e280405 -> v1.0.0-20201130134442-10cb98267c6c
  - Deprecated packages: none.
- Evidence:
  - `go.mod:3-13`
  - `go.sum`
  - `reports/.artifacts/step_05_security_dependency_audit.md`
  - `reports/.artifacts/step_06_security_dependency_age.md`
- Risks:
  - Deferred minor and patch updates can accumulate compatibility and maintenance cost even without a current reachable CVE.
- Recommendations:
  1. Review `pflag`, `x/sys`, and `x/term` updates before release.
  2. Re-run the full release gate and regenerate checksum-pinned Homebrew resources after dependency changes.
  3. Update indirect dependencies through their direct parents where practical.

## 5. Secret Detection

- Description: Evaluates tracked source for hardcoded credential patterns and scans the working tree and Git history with Gitleaks.
- Score: 100/100 (Strong)
- Score Breakdown:
  - Base: 100
  - HIGH findings: 0
  - MEDIUM findings: 0
  - LOW findings: 0
  - Git-history findings: 0
  - Pre-commit secret-hook bonus: +0
  - `.gitleaks.toml` bonus: +0
  - Final: 100/100 (Strong)
- Key Findings:
  - [LOW]: No hardcoded secret pattern was confirmed in production source or tracked supplemental text.
  - [LOW]: Gitleaks found no secret in the working directory or Git history.
  - [LOW]: Invalid issue-list filters and malformed cursors fail before profile lookup, credential loading, or network-client construction; missing profiles also fail before credential loading.
- Evidence:
  - `reports/.artifacts/step_03_security_secret_patterns.md`
  - `reports/.artifacts/step_04_security_gitleaks.md`
  - `internal/cli/commands.go:363-379`
  - `internal/cli/cli_test.go:242-307`
  - `internal/cli/cli_test.go:311-334`
- Risks:
  - Future commits can introduce secrets even though the present source and history scans are clean.
- Recommendations:
  1. Re-run redacted Gitleaks scans before publication and tagged releases.
  2. Preserve validation-before-credential-loading tests when adding commands.

## 6. Sensitive File Protection

- Description: Evaluates sensitive-file tracking, environment and key ignore coverage, local profile metadata, and the Keychain access boundary.
- Score: 100/100 (Strong)
- Score Breakdown:
  - Base: 100
  - Tracked `.env` files: 0
  - Tracked private keys or certificates: 0
  - Missing environment ignore pattern: 0
  - Tracked cloud credential files: 0
  - Safe `.env.example` bonus: +0
  - Multi-directory `.gitignore` bonus: +0
  - Final: 100/100 (Strong)
- Key Findings:
  - [LOW]: No sensitive or credential-bearing file is tracked, and `.gitignore` protects environment files and common private-key formats.
  - [MEDIUM]: The documented allow-any-application Keychain decrypt ACL protects tokens at rest but does not isolate them from other same-user processes while the Keychain is unlocked.
  - [LOW]: Client construction performs a preflight profile check and then re-reads the profile under its per-profile lock, so a concurrent URL change is honored and a concurrent deletion fails before credential loading.
- Evidence:
  - `.gitignore:1-15`
  - `reports/.artifacts/step_02_security_file_analysis.md`
  - `SECURITY.md:7-17`
  - `internal/auth/keychain_darwin.go:56-80`
  - `internal/cli/root.go:174-212`
  - `internal/cli/cli_test.go:311-389`
- Risks:
  - A compromised process running as the same macOS user can read stored Redmine tokens while the user's Keychain is unlocked.
  - Optional generic ignore patterns for certificate, cloud-credential, and credential-directory names are absent, although no matching file exists now.
- Recommendations:
  1. Use a dedicated OS account where local same-user process isolation is required.
  2. Keep Redmine tokens in the native credential backend and profile files limited to normalized non-secret metadata.
  3. Consider adding the optional generic ignore patterns before accepting broader repository content.

## 7. Supply Chain Integrity

- Description: Evaluates dependency sources, local replacements, integrity hashes, workflow pinning, and reproducible release controls.
- Score: 100/100 (Strong)
- Score Breakdown:
  - Base: 100
  - Git-sourced dependencies: 0
  - Path-based dependencies: 0
  - Missing lock file: 0
  - Missing integrity hashes: 0
  - Unknown registry dependencies: 0
  - Dependency tree depth greater than six: 0
  - Circular dependencies: 0
  - Official registry bonus: +10
  - Verified checksums bonus: +5
  - Unclamped score: 115
  - Final: 100/100 (Strong)
- Key Findings:
  - [LOW]: All external dependencies resolve as versioned Go modules with `go.sum` hashes and no replacement directive.
  - [LOW]: `go mod verify` passes, and release workflows verify module checksums before building.
  - [LOW]: Homebrew resources and third-party workflow actions are pinned by cryptographic identifiers.
  - [LOW]: Release publication is gated by stable annotated-tag validation, destination-`main` ancestry, exact asset allowlisting, and digest comparison.
- Evidence:
  - `go.mod:1-13`
  - `go.sum`
  - `.github/workflows/go.yml:18-30`
  - `.github/workflows/release.yml:24-57`
  - `.github/workflows/release.yml:71-109`
  - `reports/.artifacts/step_05_security_dependency_audit.md`
- Risks:
  - Runner images, tool releases, and repository-side protection settings remain external trust dependencies.
- Recommendations:
  1. Continue reviewing pinned action SHAs and tool versions deliberately.
  2. Re-run checksum, deterministic-bundle, and offline Homebrew validation before release handoff.
  3. Verify repository-side tag and immutable-release policy at the confirmed destination.

## 8. Consolidated Findings by Severity

### HIGH - Immediate Action Required

1. [HIGH]: No HIGH severity findings.

### MEDIUM - Address Soon

1. [MEDIUM]: The source-built Keychain item uses an allow-any-application decrypt ACL, so same-user processes can decrypt it while the Keychain is unlocked. Source: `internal/auth/keychain_darwin.go:56-80` and `SECURITY.md:12-17`.

### LOW - Best Effort

1. [LOW]: Trivy is unavailable, so optional independent filesystem scanning was skipped. Source: `reports/.artifacts/step_07_security_trivy.md`.
2. [LOW]: No automated dependency-update bot or pre-commit security hook is configured. Source: `reports/.artifacts/step_05_security_dependency_audit.md`.
3. [LOW]: Six resolved modules have newer releases without a currently reachable vulnerability. Source: `reports/.artifacts/step_06_security_dependency_age.md`.
4. [LOW]: Direct GitHub API checks found no repository rulesets or `main` branch protection, and no enabled immutable-release setting. Source: `reports/.artifacts/step_05_security_dependency_audit.md`.

### Verified Release-Fix Conclusions

- Profile URL changes are refreshed under the per-profile lock, and concurrent deletion prevents a credential read. Evidence: `internal/cli/root.go:188-212`, `internal/cli/cli_test.go:336-389`.
- Invalid issue-list filters, malformed cursors, and missing local profiles do not cause Keychain credential reads. Evidence: `internal/cli/commands.go:363-385`, `internal/cli/cli_test.go:273-335`.
- Rate-limit delay parsing clamps numeric overflow; a valid `Retry-After` beyond the context deadline returns `RATE_LIMITED` with `retry_after` preserved and does not sleep. Evidence: `internal/redmine/client.go:179-218`, `internal/redmine/client.go:306-345`, `internal/redmine/client_test.go:155-206`.

## 9. Remediation Priority Matrix

1. [MEDIUM]: Use a dedicated OS account where same-user process isolation matters. Effort: Medium. Impact: High.
2. [LOW]: Add automated dependency updates and a redacted secret scanner. Effort: Low. Impact: Medium.
3. [LOW]: Review compatible direct dependency updates and regenerate release pins. Effort: Low. Impact: Medium.
4. [LOW]: Add Trivy if independent filesystem scanning is required. Effort: Low. Impact: Medium.
5. [LOW]: Configure protected `main` and `v*` refs; enable immutable releases when supported. Effort: Low. Impact: Medium.

## 10. Gemini AI Analysis

- Status: Skipped
- Gemini CLI was not installed.
- Authentication and extension state were not inspected or changed.
- External AI source analysis was outside the authorized review boundary; local static, dependency, and credential-boundary evidence was used instead.

## 11. Project Detection Results

- Detected project type: Go command-line application
- Project path: `go@.`
- Framework/runtime: Go module targeting Go 1.25.14; local scan used Go 1.27.0 on macOS arm64.
- Package manager: Go modules with `go.mod` and `go.sum`.
- Repository structure: Single application.
- Source files scanned: Tracked `.go`, `.sh`, `.md`, `.yaml`, and `.rb` files plus GitHub Actions and release assets.
- Gemini AI analysis: Skipped.

## 12. Appendix: Evidence Index

- Sensitive files: `.gitignore:1-15`, `reports/.artifacts/step_02_security_file_analysis.md`
- Secret patterns: `reports/.artifacts/step_03_security_secret_patterns.md`, `reports/.artifacts/step_04_security_gitleaks.md`
- Dependencies: `go.mod:1-13`, `go.sum`, `reports/.artifacts/step_05_security_dependency_audit.md`
- Dependency age: `reports/.artifacts/step_06_security_dependency_age.md`
- Additional scanner: `reports/.artifacts/step_07_security_trivy.md`
- SAST: `reports/.artifacts/step_08_security_sast.md`
- Gemini status: `reports/.artifacts/step_09_security_gemini_analysis.md`
- Credential boundary: `internal/auth/keychain_darwin.go:56-80`, `SECURITY.md:7-17`
- Profile and validation fixes: `internal/cli/root.go:174-212`, `internal/cli/commands.go:354-410`, `internal/cli/cli_test.go:242-389`
- Rate-limit fix: `internal/redmine/client.go:179-218`, `internal/redmine/client.go:268-345`, `internal/redmine/client_test.go:129-206`
- CI and release supply chain: `.github/workflows/go.yml:3-50`, `.github/workflows/release.yml:15-109`, `tools/release/`

## 13. Scan Metadata

- Scan date: 2026-08-27T10:43:38-03:00
- Project path: `/Users/Abigotado/StudioProjects/redmine-cli`
- Project type: Go command-line application
- Tools used: Go 1.27.0, govulncheck v1.1.4 with Go 1.25.14, Gitleaks 8.30.1, go vet, go mod verify, actionlint, ShellCheck, ripgrep, and manual source/sink review.
- Gemini AI: Skipped.
- Total findings: 5 (0 high, 1 medium, 4 low).
- Scan duration: N/A.
- Generated by: Somnio CLI vunknown.
- Skill: security-audit.
- Somnio AI Tools: [somnio-ai-tools](https://github.com/somnio-software/somnio-ai-tools)
