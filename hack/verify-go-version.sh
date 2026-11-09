#!/usr/bin/env bash
set -euo pipefail

# Check the binary itself: the GoReleaser process can use a different toolchain.
expected_version="go$(awk '$1 == "go" { print $2; exit }' go.mod)"
actual_version=$(go version -m "${1:?Usage: verify-go-version.sh BINARY}" | awk 'NR == 1 { print $NF }')

if [[ "${actual_version}" != "${expected_version}" ]]; then
  echo "${1}: expected ${expected_version}, got ${actual_version}" >&2
  exit 1
fi

echo "${1}: verified ${actual_version}"
