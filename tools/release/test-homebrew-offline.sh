#!/usr/bin/env bash
set -euo pipefail

repository_root=$(git rev-parse --show-toplevel)
commit=$(git rev-parse HEAD)
temporary_dir=$(mktemp -d "${TMPDIR:-/tmp}/redmine-homebrew-offline.XXXXXX")
cleanup() {
  rm -rf "$temporary_dir"
}
trap cleanup EXIT

source_dir="$temporary_dir/source"
mkdir -p "$source_dir"
git -C "$repository_root" archive "$commit" | tar -x -C "$source_dir"
mkdir -p "$source_dir/vendor"

while IFS=$'\t' read -r module version expected_digest; do
  if [[ -z $module || -z $version || ! $expected_digest =~ ^[0-9a-f]{64}$ ]]; then
    echo "invalid Homebrew resource manifest entry" >&2
    exit 1
  fi
  module_json=$(go mod download -json "$module@$version")
  module_zip=$(jq -er '.Zip' <<<"$module_json")
  actual_digest=$(shasum -a 256 "$module_zip" | awk '{print $1}')
  if [[ $actual_digest != "$expected_digest" ]]; then
    echo "Homebrew resource digest mismatch for $module@$version" >&2
    exit 1
  fi
  extract_dir="$temporary_dir/extract"
  rm -rf "$extract_dir"
  mkdir -p "$extract_dir" "$(dirname "$source_dir/vendor/$module")"
  unzip -q "$module_zip" -d "$extract_dir"
  module_root="$extract_dir/$module@$version"
  if [[ ! -d $module_root ]]; then
    echo "unexpected module archive root for $module@$version" >&2
    exit 1
  fi
  cp -R "$module_root" "$source_dir/vendor/$module"
done <"$repository_root/packaging/homebrew/resources.tsv"

cp "$source_dir/packaging/homebrew/modules.txt" "$source_dir/vendor/modules.txt"
empty_module_cache="$temporary_dir/empty-module-cache"
empty_build_cache="$temporary_dir/empty-build-cache"
mkdir -p "$empty_module_cache" "$empty_build_cache"
(
  cd "$source_dir"
  GOPROXY=off \
    GOSUMDB=off \
    GOMODCACHE="$empty_module_cache" \
    GOCACHE="$empty_build_cache" \
    GOFLAGS='-mod=vendor -trimpath' \
    go build -o "$temporary_dir/redmine-cli" ./cmd/redmine-cli
)

echo "offline Homebrew dependency build passed"
