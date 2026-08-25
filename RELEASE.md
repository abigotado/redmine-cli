# Release process

`redmine-cli` v0.1.x releases are source-only. A release contains exactly:

- `redmine-cli-X.Y.Z.tar.gz`;
- `release-notes-vX.Y.Z.md`;
- `release-manifest.json`;
- `SHA256SUMS`.

The workflow does not publish unsigned macOS binaries. Linux and Windows
builds cannot authenticate until native credential backends exist.

## Destination gate

Before any push, confirm that the intended repository is exactly
`github.com/abigotado/redmine-cli`, add or inspect its remote, and verify that
the remote default branch is `main`. Never rely on saved GitHub CLI context.

Configure the destination before the first release:

1. Protect `main` and require the normal pull-request checks.
2. Protect `v*` tags so only the release operator can create or update them.
3. Enable GitHub immutable releases when the repository plan supports them.
4. Keep the default Actions token read-only; the release workflow grants
   `contents: write` only to its final publication job.

If immutable releases are unavailable, treat release assets as checksum-pinned,
not intrinsically immutable.

## Prepare and merge

1. Work on a human project-convention branch such as `feat/redmine_cli`.
2. Update `CHANGELOG.md` and add `docs/releases/vX.Y.Z.md`.
3. Run `make release-check` and the publication security review.
4. Commit and push the feature branch only after explicitly verifying its
   destination remote.
5. Merge through a pull request. Do not tag the feature-branch commit.
6. Fetch the destination and require the candidate commit to be the merged
   `origin/main` tip. Rebase or rebuild the release candidate if destination
   `main` already has history.

The local repository began with an unborn `main`; therefore its initial
feature-branch root commit must never be treated as proof of destination-main
ancestry. The remote `main` check in the release workflow is authoritative.

## Tag and publish through CI

After the release commit is merged into destination `main`, create an annotated
stable SemVer tag on that exact commit and push only the tag:

```text
git fetch origin main --tags
git merge-base --is-ancestor <release-commit> origin/main
git tag -a v0.1.0 <release-commit> -m "redmine-cli v0.1.0"
git push origin refs/tags/v0.1.0
```

The tag workflow independently validates the annotated tag object, recursively
peels it to a commit, checks destination-main ancestry and changelog/release
notes, rebuilds the source archive twice, and verifies identical SHA-256
digests. Its final no-checkout job resolves the live tag through the GitHub API
again before creating a draft, verifies all four uploaded asset names, sizes,
and digests, and only then publishes the release.

The workflow fails closed if any release with the same tag already exists. If
an upload interruption leaves a partial draft, inspect it, delete only that
draft explicitly, verify that the tag has not moved, and rerun the same workflow
run. Never overwrite or clobber an existing asset.

## Homebrew handoff

After publication, copy the source-archive digest from `SHA256SUMS` and render
the Formula locally:

```text
go run ./tools/renderformula \
  --version 0.1.0 \
  --source-url https://github.com/abigotado/redmine-cli/releases/download/v0.1.0/redmine-cli-0.1.0.tar.gz \
  --sha256 <source-archive-sha256> \
  --output /absolute/local/path/redmine-agent-cli.rb
```

Run the Formula tests before proposing it to a tap. Publishing or updating a
Homebrew tap is a separate explicitly authorized action.
