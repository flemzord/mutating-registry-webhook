#!/usr/bin/env bash

set -euo pipefail

echo "${GITHUB_TOKEN}" | helm registry login ghcr.io --username "${GITHUB_ACTOR}" --password-stdin

shopt -s nullglob
charts=(dist/mutating-registry-webhook-*.tgz)
if [[ ${#charts[@]} -ne 1 ]]; then
  echo "Expected exactly one packaged Helm chart, found ${#charts[@]}" >&2
  exit 1
fi

helm push "${charts[0]}" "oci://ghcr.io/flemzord/charts"
