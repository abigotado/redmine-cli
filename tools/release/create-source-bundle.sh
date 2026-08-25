#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 3 ]]; then
  echo "usage: create-source-bundle.sh TAG COMMIT OUTPUT_DIR" >&2
  exit 2
fi

tag=$1
commit=$2
output_dir=$3

if [[ ! $tag =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "release tag must be stable SemVer with a v prefix" >&2
  exit 2
fi
if [[ ! $commit =~ ^[0-9a-f]{40}$ ]]; then
  echo "release commit must be a full lowercase SHA-1" >&2
  exit 2
fi
if [[ $output_dir != /* ]]; then
  echo "output directory must be absolute" >&2
  exit 2
fi

repository_root=$(git rev-parse --show-toplevel)
cd "$repository_root"

if [[ $(git cat-file -t "refs/tags/$tag" 2>/dev/null) != tag ]]; then
  echo "release tag must be an annotated tag" >&2
  exit 1
fi
tag_object_sha=$(git rev-parse "refs/tags/$tag")
peeled_commit_sha=$(git rev-parse "refs/tags/$tag^{commit}")
if [[ $peeled_commit_sha != "$commit" ]]; then
  echo "release tag does not peel to the requested commit" >&2
  exit 1
fi

version=${tag#v}
notes_path="docs/releases/$tag.md"
if ! git cat-file -e "$commit:$notes_path" 2>/dev/null; then
  echo "release notes are missing: $notes_path" >&2
  exit 1
fi
if ! git show "$commit:CHANGELOG.md" | grep -Eq "^## \[$version\] - [0-9]{4}-[0-9]{2}-[0-9]{2}$"; then
  echo "CHANGELOG.md has no exact entry for $version" >&2
  exit 1
fi

mkdir -p "$output_dir"
if find "$output_dir" -mindepth 1 -maxdepth 1 -print -quit | grep -q .; then
  echo "output directory must be empty" >&2
  exit 1
fi

archive_name="redmine-cli-$version.tar.gz"
notes_name="release-notes-$tag.md"
manifest_name=release-manifest.json
checksum_name=SHA256SUMS
temporary_dir=$(mktemp -d "${TMPDIR:-/tmp}/redmine-release.XXXXXX")
cleanup() {
  rm -rf "$temporary_dir"
}
trap cleanup EXIT

for attempt in one two; do
  git archive --format=tar --prefix="redmine-cli-$version/" "$commit" |
    gzip -n -9 >"$temporary_dir/$archive_name.$attempt"
done

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

first_digest=$(sha256_file "$temporary_dir/$archive_name.one")
second_digest=$(sha256_file "$temporary_dir/$archive_name.two")
if [[ $first_digest != "$second_digest" ]]; then
  echo "source archive generation is not deterministic" >&2
  exit 1
fi
mv "$temporary_dir/$archive_name.one" "$output_dir/$archive_name"
git show "$commit:$notes_path" >"$output_dir/$notes_name"

archive_entries="$temporary_dir/archive-entries.txt"
tar -tzf "$output_dir/$archive_name" >"$archive_entries"
if [[ ! -s $archive_entries ]] ||
  grep -Ev "^redmine-cli-$version/([^/].*)?$" "$archive_entries" | grep -q .; then
  echo "source archive contains an invalid root layout" >&2
  exit 1
fi
for required in go.mod go.sum LICENSE cmd/redmine-cli/main.go packaging/homebrew/modules.txt packaging/homebrew/resources.tsv; do
  if ! grep -Fxq "redmine-cli-$version/$required" "$archive_entries"; then
    echo "source archive is missing $required" >&2
    exit 1
  fi
done
if grep -E '(^|/)\.git(/|$)|(^|/)\.env($|\.)|(^|/)\.DS_Store$' "$archive_entries" | grep -q .; then
  echo "source archive contains forbidden local content" >&2
  exit 1
fi

archive_size=$(wc -c <"$output_dir/$archive_name" | tr -d ' ')
notes_digest=$(sha256_file "$output_dir/$notes_name")
notes_size=$(wc -c <"$output_dir/$notes_name" | tr -d ' ')
cat >"$temporary_dir/manifest.json" <<EOF
{
  "schema": 1,
  "tag": "$tag",
  "version": "$version",
  "tag_object_sha": "$tag_object_sha",
  "commit_sha": "$commit",
  "source": {
    "name": "$archive_name",
    "sha256": "$first_digest",
    "size": $archive_size
  },
  "release_notes": {
    "name": "$notes_name",
    "sha256": "$notes_digest",
    "size": $notes_size
  }
}
EOF
jq -S . "$temporary_dir/manifest.json" >"$output_dir/$manifest_name"

(
  cd "$output_dir"
  for asset in "$archive_name" "$notes_name" "$manifest_name"; do
    printf '%s  %s\n' "$(sha256_file "$asset")" "$asset"
  done | LC_ALL=C sort -k2 >"$checksum_name"
)

actual_assets=$(find "$output_dir" -mindepth 1 -maxdepth 1 -type f -print | wc -l | tr -d ' ')
if [[ $actual_assets != 4 ]]; then
  echo "release bundle has an unexpected asset count" >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  (cd "$output_dir" && sha256sum -c "$checksum_name")
else
  (cd "$output_dir" && shasum -a 256 -c "$checksum_name")
fi
