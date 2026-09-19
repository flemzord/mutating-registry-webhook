#!/usr/bin/env bash

set -euo pipefail

VERSION="${1}"
VERSION="${VERSION#v}"  # Remove 'v' prefix
CHART_DIR="${2:-dist/chart}"

sed -i.bak "s/^version:.*/version: $VERSION/" "${CHART_DIR}/Chart.yaml"
sed -i.bak "s/^appVersion:.*/appVersion: \"$VERSION\"/" "${CHART_DIR}/Chart.yaml"
rm -f "${CHART_DIR}/Chart.yaml.bak"
