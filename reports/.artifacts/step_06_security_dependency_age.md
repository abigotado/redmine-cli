# Step 06 - Security Dependency Age

## Counts

- OUTDATED COUNT: 3 direct dependencies; 3 additional indirect dependencies (6 total).
- DEPRECATED COUNT: 0.
- RETRACTED RESOLVED VERSION COUNT: 0.
- Resolved external dependency count: 10 (5 direct, 5 indirect).

The counts come from `go list -m -u all` and structured `go list -m -u -json all` output on 2026-08-27. A dependency is classified as direct when the resolved module metadata reports `Indirect: false`, including `github.com/spf13/pflag`, which is explicitly required in `go.mod`.

## Outdated direct dependencies

| Package | Current | Latest | Delta |
| --- | --- | --- | --- |
| `github.com/spf13/pflag` | v1.0.9 | v1.0.10 | patch |
| `golang.org/x/sys` | v0.44.0 | v0.47.0 | minor |
| `golang.org/x/term` | v0.32.0 | v0.45.0 | minor |

## Outdated indirect dependencies

| Package | Current | Latest | Delta |
| --- | --- | --- | --- |
| `github.com/cpuguy83/go-md2man/v2` | v2.0.6 | v2.0.7 | patch |
| `go.yaml.in/yaml/v3` | v3.0.4 | v3.0.5 | patch |
| `gopkg.in/check.v1` | v0.0.0-20161208181325-20d25e280405 | v1.0.0-20201130134442-10cb98267c6c | major by version prefix; pseudo-version update within the v1 module path |

## Deprecated dependencies

DEPRECATED LIST: none. No resolved module emitted Go module deprecation metadata.

## Additional age observations

- `github.com/inconshreveable/mousetrap` v1.1.0 (2022-11-27) and `github.com/russross/blackfriday/v2` v2.1.0 (2020-10-27) are old but `go list -m -u` found no newer version, so they are not counted as outdated.
- The pinned release baseline `go1.25.14` completed `govulncheck@v1.1.4` with zero findings. No listed update is required to remediate a currently reachable vulnerability.

## Summary

Prioritize the direct patch update to `pflag`, then assess the `x/sys` and `x/term` minor updates together because they are related low-level platform modules. Run the full release gate and regenerate the checksum-pinned Homebrew resources after any module change. Evaluate transitive updates through their direct parent dependencies where possible. No deprecated module replacement is required.
