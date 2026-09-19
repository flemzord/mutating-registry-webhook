#!/usr/bin/env bash

set -euo pipefail

chart_dir="${1:-dist/chart}"
chart_version="${2:-}"
values_file="${chart_dir}/values.yaml"

test -f "${values_file}" || {
  echo "Helm values file not found: ${values_file}" >&2
  exit 1
}

grep -q '^  replicas: 1$' "${values_file}" || {
  echo "Unexpected generated replicas value in ${values_file}" >&2
  exit 1
}
grep -Eq '^    repository: (controller|ghcr.io/flemzord/mutating-registry-webhook)$' "${values_file}" || {
  echo "Unexpected generated image repository in ${values_file}" >&2
  exit 1
}

sed -i.bak \
  -e 's/^  replicas: 1$/  replicas: 2/' \
  -e 's|^    repository: controller$|    repository: ghcr.io/flemzord/mutating-registry-webhook|' \
  "${values_file}"
rm -f "${values_file}.bak"

if [[ -n "${chart_version}" ]]; then
  "$(dirname "$0")/update-chart-version.sh" "${chart_version}" "${chart_dir}"
fi
