# Step 05 - Security Dependency Audit

## Scope and tooling

- PROJECT_DETECTION_RESULTS: `go@.`
- Project type: single Go command-line application
- Package manager: Go modules (`go.mod`, `go.sum`)
- Release toolchain target: `go1.25.14` from the `go` directive
- Local audit host: `go1.27.0 darwin/arm64`
- Vulnerability command: `GOTOOLCHAIN=go1.25.14 go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...`
- Vulnerability result: `No vulnerabilities found.` The scanner completed successfully at source/symbol level using the release toolchain target.

## Vulnerability counts

| Severity | Reachable findings |
| --- | ---: |
| Critical | 0 |
| High | 0 |
| Medium | 0 |
| Low | 0 |
| **Total** | **0** |

Because no vulnerability was reported, there are no affected packages or remediation versions to enumerate. `govulncheck` is the repository's native Go vulnerability gate and is pinned to v1.1.4 in the Makefile and both CI workflows.

## Dependency and integrity status

- External modules in the resolved build list: 10 (5 direct and 5 indirect).
- Modules with a newer available version: 6 (3 direct and 3 indirect); see Step 06.
- `go mod verify`: PASS (`all modules verified`).
- Lock/checksum file: `go.sum` exists (16 lines) and contains module and `go.mod` hashes.
- Replacement directives: 0.
- Retracted resolved versions: 0.
- Deprecated resolved modules: 0.
- Git/path/unversioned dependencies: 0; all external dependencies resolve as versioned Go modules.

## Automated security tooling

- Dependabot: NOT CONFIGURED.
- Renovate: NOT CONFIGURED.
- Snyk: NOT CONFIGURED.
- Pre-commit security hooks: NOT CONFIGURED.
- CI vulnerability scanning: CONFIGURED. `.github/workflows/go.yml` scans pull requests and pushes to `main`; `.github/workflows/release.yml` repeats the scan before packaging. Both run pinned `govulncheck@v1.1.4` on Linux.
- CI checksum verification: CONFIGURED. Both workflows run `go mod verify` before build/test steps.
- Third-party GitHub Actions: all workflow uses are pinned to full commit SHAs (`checkout`, `setup-go`, and `upload-artifact`).

## Supply-chain and release evidence

- `make release-check` includes `go mod verify`, pinned `govulncheck`, workflow linting, deterministic source-bundle tests, and an offline Homebrew dependency build.
- The Homebrew Formula stages every resolved Go module as an individually SHA-256-pinned resource, then builds with `GOPROXY=off`, `GOSUMDB=off`, `GOTOOLCHAIN=local`, and `-mod=vendor`. Disabling the checksum database at that offline boundary does not remove integrity checking because the downloaded module archives are independently pinned and verified before staging.
- The release workflow verifies an annotated stable SemVer tag and destination-`main` ancestry, builds a deterministic source bundle, validates an exact asset allowlist, verifies `SHA256SUMS`, re-peels the live tag through the GitHub API, and compares uploaded asset digests.
- Release publication grants `contents: write` only to the final no-checkout publish job; earlier verification and packaging jobs are read-only.
- Direct GitHub API checks on 2026-08-27 returned no repository rulesets, a
  404 for `main` branch protection, and no enabled immutable-release setting.
  The required destination protection gate is therefore not configured.

## Recommendations

1. Review and update the 3 outdated direct modules before release, regenerating and re-verifying Homebrew resource pins as required; these updates are maintenance items, not current vulnerability remediations.
2. Add Dependabot or Renovate for ongoing Go module and pinned GitHub Action update coverage.
3. Optionally add Trivy filesystem scanning and/or SBOM generation as defense in depth; absence of Trivy does not invalidate the clean Go-native vulnerability result.
