# Changelog

All notable changes to this project are documented in this file. The format is
based on Keep a Changelog, and this project follows Semantic Versioning.

## [Unreleased]

## [0.2.1] - 2026-09-21

### Fixed

- Publish v0.2 source artifacts from an annotated release tag through the
  checksum-verified release pipeline.

## [0.2.0] - 2026-09-18

### Added

- Explicitly confirmed issue and project create/update commands.
- Project-file listing and upload plus bounded attachment downloads.

### Fixed

- Send Redmine's `is_public` attribute for project visibility and reject
  immutable project identifiers on update.
- Derive attachment downloads from their numeric IDs instead of trusting an
  upstream content URL, and preserve timeout/cancellation semantics for reads.
- Preserve comma-containing attachment filenames, return local input failures
  as `USAGE`, and read requested write associations before rendering them.
- Keep upload tokens opaque through the CLI, send a verified Content-Length,
  and harden selected-file handling against special-file and reparse races.
- Validate issue-list filters and malformed pagination cursors before profile
  and Keychain access so invalid local input returns `USAGE` without credential
  operations.
- Avoid creating per-profile lock artifacts for profile names that do not
  exist while retaining an under-lock profile refresh before token use.
- Preserve `RATE_LIMITED` and the server's bounded `retry_after` when the
  advertised delay cannot fit the command deadline.
- Publish draft GitHub releases by numeric release ID instead of ambiguous tag
  lookup during final verification.
- Correct Homebrew source-version detection and checksum-pinned module resource
  staging for offline builds.

## [0.1.0] - 2026-08-25

### Added

- Read-only Redmine commands for the current user, projects, and issues.
- Explicit multi-account profiles backed by native macOS Keychain storage.
- Versioned JSON envelopes, stable exit codes, bounded output, and opaque
  query-bound pagination cursors.
- One provider-neutral Agent Skill for Codex and Claude Code with a guarded
  compare-and-swap installer.
- Source-building Homebrew Formula tooling and a source-only release pipeline.
- Race, architecture, HTTP contract, credential, skill installer, and
  disposable cross-binary Keychain tests.

[Unreleased]: https://github.com/abigotado/redmine-cli/compare/v0.2.1...HEAD
[0.2.1]: https://github.com/abigotado/redmine-cli/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/abigotado/redmine-cli/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/abigotado/redmine-cli/releases/tag/v0.1.0
