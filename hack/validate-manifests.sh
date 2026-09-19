#!/usr/bin/env bash

set -euo pipefail

chart_dir="${1:-dist/chart}"
rendered_file="$(mktemp)"
override_file="$(mktemp)"
trap 'rm -f "${rendered_file}" "${override_file}"' EXIT

helm lint "${chart_dir}"
helm template validation "${chart_dir}" > "${rendered_file}"

assert_rendered() {
  local pattern="$1"
  local description="$2"
  if ! grep -Eq -- "${pattern}" "${rendered_file}"; then
    echo "Rendered chart is missing ${description}" >&2
    exit 1
  fi
}

assert_rendered '^  replicas: 2$' 'the two-replica manager deployment'
assert_rendered 'image: "ghcr.io/flemzord/mutating-registry-webhook:[^"]+"' 'the published controller image'
assert_rendered 'path: /healthz$' 'the liveness probe'
assert_rendered 'path: /readyz$' 'the readiness probe'
assert_rendered 'pods/ephemeralcontainers$' 'ephemeral container admission'
assert_rendered '--leader-elect$' 'leader election for the two-replica controller'
assert_rendered 'coordination.k8s.io$' 'leader-election API permissions'
assert_rendered '  - leases$' 'leader-election lease permissions'

if grep -Eq 'image: "?controller:' "${rendered_file}"; then
  echo 'Rendered chart still references the local controller image' >&2
  exit 1
fi

helm template validation "${chart_dir}" \
  --set-string manager.image.repository=registry.example.com:5000/team/webhook \
  --set-string manager.image.tag=test \
  > "${override_file}"
grep -q 'image: "registry.example.com:5000/team/webhook:test"' "${override_file}"

grep -Eq 'image: ghcr.io/flemzord/mutating-registry-webhook:[^[:space:]]+' dist/install.yaml
grep -q 'pods/ephemeralcontainers' dist/install.yaml

echo 'Kustomize installer and Helm chart render checks passed.'
