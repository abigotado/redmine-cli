#!/usr/bin/env bash
set -euo pipefail

temporary_dir=$(mktemp -d "${TMPDIR:-/tmp}/redmine-draft-lookup-test.XXXXXX")
trap 'rm -rf "$temporary_dir"' EXIT
mkdir "$temporary_dir/bin"
mkdir "$temporary_dir/release-bundle"
ln -s "$PWD/tools/release/testdata/draft-lookup-command.sh" "$temporary_dir/bin/gh"
ln -s "$PWD/tools/release/testdata/draft-lookup-command.sh" "$temporary_dir/bin/sleep"
export PATH="$temporary_dir/bin:$PATH"
export LOOKUP_ATTEMPTS_FILE="$temporary_dir/attempts"
export LOOKUP_SLEEPS_FILE="$temporary_dir/sleeps"
export RUNNER_TEMP=$temporary_dir
export GITHUB_REPOSITORY=example/redmine-cli

bundle=$temporary_dir/release-bundle
printf 'archive\n' >"$bundle/redmine-cli-0.2.2.tar.gz"
printf 'notes\n' >"$bundle/release-notes-v0.2.2.md"
printf 'manifest\n' >"$bundle/release-manifest.json"
printf 'checksums\n' >"$bundle/SHA256SUMS"
LOOKUP_ASSETS_JSON=$(
  for asset in redmine-cli-0.2.2.tar.gz release-notes-v0.2.2.md release-manifest.json SHA256SUMS; do
    size=$(wc -c <"$bundle/$asset" | tr -d ' ')
    digest="sha256:$(sha256sum "$bundle/$asset" | awk '{print $1}')"
    jq -nc --arg name "$asset" --argjson size "$size" --arg digest "$digest" \
      '{name: $name, size: $size, digest: $digest}'
  done | jq -sc .
)
export LOOKUP_ASSETS_JSON

awk '
  BEGIN {
    print "#!/usr/bin/env bash"
    print "set -euo pipefail"
    print "tag=v0.2.2"
    print "bundle=$RUNNER_TEMP/release-bundle"
    print "archive=redmine-cli-0.2.2.tar.gz"
    print "notes=release-notes-v0.2.2.md"
  }
  /# BEGIN DRAFT LOOKUP/ { inside = 1; found++; next }
  /# END DRAFT LOOKUP/ { inside = 0; next }
  inside { print }
  END {
    if (found != 1 || inside) exit 1
    print "printf '\''%s\\n'\'' \"$release_json\""
  }
' .github/workflows/release.yml >"$temporary_dir/lookup.sh"
bash -n "$temporary_dir/lookup.sh"

run_case() {
  local scenario=$1
  local expected_attempts=$2
  local expected_sleeps=$3
  local expected_success=$4
  local expected_error=${5:-}
  export LOOKUP_SCENARIO=$scenario
  printf '0\n' >"$LOOKUP_ATTEMPTS_FILE"
  printf '0\n' >"$LOOKUP_SLEEPS_FILE"
  if bash "$temporary_dir/lookup.sh" \
    >"$temporary_dir/output" 2>"$temporary_dir/error"; then
    [[ $expected_success == true ]] || { echo "unexpected success: $scenario" >&2; exit 1; }
    jq -e '.id == 42 and .tag_name == "v0.2.2" and .draft == true and (.assets | length) == 4' \
      "$temporary_dir/output" >/dev/null
  else
    [[ $expected_success == false ]] || { echo "unexpected failure: $scenario" >&2; exit 1; }
    [[ ! -s $temporary_dir/output ]]
    [[ $(<"$temporary_dir/error") == *"$expected_error"* ]]
  fi
  [[ $(<"$LOOKUP_ATTEMPTS_FILE") == "$expected_attempts" ]]
  [[ $(<"$LOOKUP_SLEEPS_FILE") == "$expected_sleeps" ]]
}

run_case immediate 1 0 true
run_case delayed_release 2 1 true
run_case delayed_assets 2 1 true
run_case delayed_digest 2 1 true
run_case missing 12 11 false 'did not become complete after 12 attempts'
run_case duplicate 1 0 false 'expected exactly one release'
run_case published 1 0 false 'is not a draft'
run_case wrong_digest 1 0 false 'remote asset verification failed'
run_case extra_asset 1 0 false 'unexpected release asset'
run_case duplicate_asset 1 0 false 'duplicate release asset'
run_case malformed 1 0 false 'expected a release list'
run_case api_error 1 0 false 'API unavailable'
echo "draft release lookup tests passed"
