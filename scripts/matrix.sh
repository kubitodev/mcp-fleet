#!/usr/bin/env bash
# Emit the build matrix (build:true entries) as compact JSON for GitHub Actions.
# Requires: yq (mikefarah).
set -euo pipefail
cd "$(dirname "$0")/.."
yq -o=json -I=0 '
  [ .mcps[]
    | select(.build == true)
    | { "name": .name,
        "ref": .ref,
        "context": ("vendor/" + .name),
        "dockerfile": (.dockerfile // ("vendor/" + .name + "/Dockerfile")) }
  ]' fleet.yaml
