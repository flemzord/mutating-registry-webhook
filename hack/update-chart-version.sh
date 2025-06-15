#!/bin/bash
set -e

VERSION="${1}"
VERSION="${VERSION#v}"  # Remove 'v' prefix

sed -i.bak "s/^version:.*/version: $VERSION/" dist/chart/Chart.yaml
sed -i.bak "s/^appVersion:.*/appVersion: \"$VERSION\"/" dist/chart/Chart.yaml
rm -f dist/chart/Chart.yaml.bak