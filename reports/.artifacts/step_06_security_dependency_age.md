# Step 06 - Dependency Age

- OUTDATED COUNT: 2 direct dependencies; 4 additional transitive modules have newer releases.
- DEPRECATED COUNT: 0
- OUTDATED LIST:
  - `golang.org/x/sys`: v0.44.0 -> v0.47.0, minor delta; v0.44.0 is the first version fixing the advisory relevant to the previous module version.
  - `golang.org/x/term`: v0.32.0 -> v0.45.0, minor delta.
- TRANSITIVE UPDATE LIST:
  - `github.com/cpuguy83/go-md2man/v2`: v2.0.6 -> v2.0.7.
  - `github.com/spf13/pflag`: v1.0.9 -> v1.0.10.
  - `go.yaml.in/yaml/v3`: v3.0.4 -> v3.0.5.
  - `gopkg.in/check.v1`: older pseudo-version -> newer pseudo-version.
- DEPRECATED LIST: none detected.
- SUMMARY: no update is required for a known reachable vulnerability; review compatible minor updates before a release.
