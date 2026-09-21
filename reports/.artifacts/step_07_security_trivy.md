# Step 07 - Security Trivy Scan

- TRIVY STATUS: NOT_INSTALLED
- Detection command: `command -v trivy`
- Filesystem scan: SKIPPED; vulnerability counts and affected-package results are unavailable from Trivy.
- Critical findings summary: not applicable because no Trivy scan ran.
- Installation instruction for macOS: `brew install trivy`
- Recommendation: install Trivy and run `trivy fs .` before publication if a second ecosystem-wide filesystem scanner is required. The repository's pinned Go-native `govulncheck` scan was run separately in Step 05 and reported zero vulnerabilities.
