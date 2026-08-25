# Step 01 - Security Tool Detection

- PROJECT_DETECTION_RESULTS: go@.
- Project type: Go command-line application
- Source extensions: `.go`, `.sh`, `.md`, `.yaml`, `.rb`
- Package manager: Go modules (`go.mod`, `go.sum`)
- Go: `/opt/homebrew/bin/go`, go1.27.0 used for this local scan; module minimum is go1.25.0
- Gitleaks: installed
- Trivy: not installed
- govulncheck: executed through a pinned `go run` tool version
- actionlint and ShellCheck: installed
- Gemini CLI: not installed; external AI analysis was also outside the authorized review boundary
- Gemini authentication and extension were not inspected or changed
- Gemini AI analysis available: no
