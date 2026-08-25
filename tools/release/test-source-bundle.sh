#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
bundle_script="$script_dir/create-source-bundle.sh"
temporary_dir=$(mktemp -d "${TMPDIR:-/tmp}/redmine-release-test.XXXXXX")
cleanup() {
  rm -rf "$temporary_dir"
}
trap cleanup EXIT

repository="$temporary_dir/repository"
mkdir -p "$repository/docs/releases" "$repository/cmd/redmine-cli"
cd "$repository"
git init -q -b main
git config user.name "Release Test"
git config user.email "release-test@example.invalid"
printf 'module example.invalid/redmine-cli\n\ngo 1.25.0\n' >go.mod
printf 'fixture checksums\n' >go.sum
printf 'MIT\n' >LICENSE
printf 'package main\n\nfunc main() {}\n' >cmd/redmine-cli/main.go
printf '# Changelog\n\n## [0.1.0] - 2026-08-25\n' >CHANGELOG.md
printf '# v0.1.0\n' >docs/releases/v0.1.0.md
git add .
GIT_AUTHOR_DATE=2026-08-25T00:00:00Z \
  GIT_COMMITTER_DATE=2026-08-25T00:00:00Z \
  git commit -q -m "fixture"
commit=$(git rev-parse HEAD)
GIT_COMMITTER_DATE=2026-08-25T00:00:01Z \
  git tag -a v0.1.0 -m "v0.1.0" "$commit"
printf 'untracked local data\n' >local-only.txt

first="$temporary_dir/first"
second="$temporary_dir/second"
"$bundle_script" v0.1.0 "$commit" "$first" >/dev/null
"$bundle_script" v0.1.0 "$commit" "$second" >/dev/null

for asset in redmine-cli-0.1.0.tar.gz release-notes-v0.1.0.md release-manifest.json SHA256SUMS; do
  cmp "$first/$asset" "$second/$asset"
done

tar -tzf "$first/redmine-cli-0.1.0.tar.gz" >"$temporary_dir/entries"
grep -Fxq 'redmine-cli-0.1.0/go.mod' "$temporary_dir/entries"
if grep -Eq '(^|/)\.git(/|$)|local-only\.txt$' "$temporary_dir/entries"; then
  echo "archive included repository metadata or an untracked file" >&2
  exit 1
fi

jq -e --arg commit "$commit" '
  .schema == 1 and
  .tag == "v0.1.0" and
  .version == "0.1.0" and
  .commit_sha == $commit and
  (.tag_object_sha | test("^[0-9a-f]{40}$")) and
  .source.name == "redmine-cli-0.1.0.tar.gz" and
  .release_notes.name == "release-notes-v0.1.0.md"
' "$first/release-manifest.json" >/dev/null

if command -v sha256sum >/dev/null 2>&1; then
  (cd "$first" && sha256sum -c SHA256SUMS >/dev/null)
else
  (cd "$first" && shasum -a 256 -c SHA256SUMS >/dev/null)
fi

if "$bundle_script" invalid "$commit" "$temporary_dir/invalid" >/dev/null 2>&1; then
  echo "invalid tag was accepted" >&2
  exit 1
fi
git tag v0.1.1 "$commit"
if "$bundle_script" v0.1.1 "$commit" "$temporary_dir/lightweight" >/dev/null 2>&1; then
  echo "lightweight tag was accepted" >&2
  exit 1
fi

echo "source release bundle tests passed"
