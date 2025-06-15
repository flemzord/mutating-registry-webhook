#!/bin/bash
set -e

echo "$GITHUB_TOKEN" | helm registry login ghcr.io --username "$GITHUB_ACTOR" --password-stdin
helm push dist/mutating-registry-webhook-*.tgz "oci://ghcr.io/flemzord/charts"