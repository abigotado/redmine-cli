# Step 08 - SAST Pattern Scan

- Language and scope: Go source, release shell scripts, workflows, and Formula tooling in the repository.
- SQL injection: 0 findings.
- XSS: 0 findings; no browser-rendering surface exists.
- Path traversal: 0 findings from request-derived file paths.
- Eval/code injection: 0 production findings.
- `os/exec` appears only in architecture and disposable-Keychain tests, where it invokes fixed Go-built helper binaries or the Go toolchain with argument arrays.
- Release shell injection: 0 findings; ref contexts are quoted environment values and stable SemVer/full SHAs are validated before use.
- Release path traversal: 0 findings; asset names derive from validated SemVer, output must be absolute and empty, and publication enforces an exact regular-file allowlist.
- Windows uninstall race: independently reported and remediated. Cleanup atomically moves an owned path to a random quarantine name, opens it without delete sharing, then hashes and marks that same handle for deletion. Concurrent replacement of the original path is preserved; a changed quarantine is retained with a conflict.
- Prompt injection boundary: independently reported and remediated. The Agent Skill explicitly treats all server-controlled Redmine values as untrusted data and requires separate user authorization for suggested actions.
- Homebrew network boundary: independently reported and remediated. Formula module resources are checksum-pinned and the build disables the module proxy and checksum service while enforcing vendoring.
- LOW findings: 0
- MEDIUM findings: 0
