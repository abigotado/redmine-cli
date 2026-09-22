#!/usr/bin/env bash
set -euo pipefail

case ${0##*/} in
  sleep)
    [[ $# == 1 && $1 == 5 ]]
    count=$(<"$LOOKUP_SLEEPS_FILE")
    printf '%s\n' "$((count + 1))" >"$LOOKUP_SLEEPS_FILE"
    ;;
  gh)
    [[ $# == 2 && $1 == api && $2 == 'repos/example/redmine-cli/releases?per_page=100' ]]
    count=$(<"$LOOKUP_ATTEMPTS_FILE")
    printf '%s\n' "$((count + 1))" >"$LOOKUP_ATTEMPTS_FILE"
    release=$(jq -nc --argjson assets "$LOOKUP_ASSETS_JSON" \
      '{id: 42, tag_name: "v0.2.2", draft: true, assets: $assets}')
    case $LOOKUP_SCENARIO in
      immediate) jq -cn --argjson release "$release" '[$release]' ;;
      delayed_release)
        if [[ $count == 0 ]]; then
          printf '[]\n'
        else
          jq -cn --argjson release "$release" '[$release]'
        fi
        ;;
      delayed_assets)
        if [[ $count == 0 ]]; then
          jq -cn --argjson release "$release" '[$release | .assets = []]'
        else
          jq -cn --argjson release "$release" '[$release]'
        fi
        ;;
      delayed_digest)
        if [[ $count == 0 ]]; then
          jq -cn --argjson release "$release" '[$release | .assets[0].digest = null]'
        else
          jq -cn --argjson release "$release" '[$release]'
        fi
        ;;
      missing) printf '[]\n' ;;
      duplicate) jq -cn --argjson release "$release" '[$release, $release]' ;;
      published) jq -cn --argjson release "$release" '[$release | .draft = false]' ;;
      wrong_digest) jq -cn --argjson release "$release" '[$release | .assets[0].digest = "sha256:bad"]' ;;
      extra_asset) jq -cn --argjson release "$release" '[$release | .assets += [{name: "extra", size: 1, digest: "sha256:bad"}]]' ;;
      duplicate_asset) jq -cn --argjson release "$release" '[$release | .assets += [.assets[0]]]' ;;
      malformed) printf '{}\n' ;;
      api_error) echo 'API unavailable' >&2; exit 1 ;;
      *) exit 2 ;;
    esac
    ;;
  *) exit 2 ;;
esac
