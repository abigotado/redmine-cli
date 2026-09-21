# Step 03 - Source Secret Patterns

## Detected project type and scan targets

- `PROJECT_DETECTION_RESULTS`: `go@.`
- Primary mandatory target: 55 tracked `*.go` files (39 production files after excluding `*_test.go` and test/vendor paths).
- Supplemental target: tracked textual source, workflow, release, Formula, and documentation files.
- Sensitive credential-file names were checked through tracked paths only; none were tracked. Credential-file contents and environment-variable values were not read.

## SOURCE CODE SECRET PATTERNS results (mandatory)

The scan covered hardcoded password/secret/token/API-key assignments, AWS access-key signatures, payment and VCS token signatures, literal long Bearer credentials, credentials embedded in connection URLs, and private-key headers.

No hardcoded secret patterns detected in source code.

One assignment-shaped match was reviewed and excluded as a test fixture:

- `internal/auth/keychain_integration_darwin_test.go:43` — `password := <redacted test fixture>`; this value is used only by a Darwin integration test to create a temporary local Keychain and is not an application credential.

No production Go file matched the assignment pattern. No tracked text matched the high-confidence cloud, payment, VCS, Bearer, embedded-URL-credential, or private-key patterns.

## Findings by severity

### HIGH

None.

### MEDIUM

None.

### LOW

None.

## Summary

- HIGH: 0
- MEDIUM: 0
- LOW: 0
- Excluded test/sentinel matches: 1
