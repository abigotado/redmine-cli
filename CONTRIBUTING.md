# Contributing

Keep the external Redmine surface read-only unless a separately reviewed
contract deliberately adds mutations. Machine-envelope fields, exit codes,
flag names, defaults, and profile-selection behavior are public API.

Before changing auth, profile, request construction, logging, or output:

- prove a credential sentinel cannot appear in stdout, stderr, verbose logs,
  wrapped errors, raw mode, or generated docs;
- keep redirects disabled and response decoding bounded;
- preserve the direct Security.framework boundary and explicit profile;
- test HTTP with `httptest` or injected transports, never a live Redmine;
- use fake stores in ordinary tests and the build-tagged disposable Keychain
  test only for cross-binary ACL behavior.

Run `make verify`, the Keychain integration test on macOS, and
`go generate ./...`. Generated command and contract references must have no
diff afterward.

macOS Homebrew packaging is a source-building Formula pinned to the canonical
release source asset and its SHA-256. Do not add a prebuilt Darwin binary, cask,
mutable URL, placeholder release, or quarantine-removal workaround.
Every Go dependency must also remain represented by a checksum-pinned Formula
resource and pass the empty-module-cache offline build test.
