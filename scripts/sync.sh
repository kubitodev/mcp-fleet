#!/usr/bin/env bash
# Vendor each MCP's source into vendor/<name>/ at the latest upstream release
# (or its `pin`), and record ref+commit in fleet.yaml. Idempotent; stages changes.
# Requires: yq (mikefarah), gh, git, rsync.
set -euo pipefail
cd "$(dirname "$0")/.."

count=$(yq '.mcps | length' fleet.yaml)
for i in $(seq 0 $((count - 1))); do
  name=$(yq -r ".mcps[$i].name" fleet.yaml)
  upstream=$(yq -r ".mcps[$i].upstream" fleet.yaml)
  pin=$(yq -r ".mcps[$i].pin // \"\"" fleet.yaml)
  cur=$(yq -r ".mcps[$i].ref" fleet.yaml)
  slug=${upstream#https://github.com/}

  # resolve target ref: pin > latest release > newest semver tag
  if [ -n "$pin" ]; then
    tag="$pin"
  else
    tag=$(gh release view --repo "$slug" --json tagName -q .tagName 2>/dev/null || true)
    [ -z "$tag" ] && tag=$(git -c 'versionsort.suffix=-' ls-remote --tags --refs --sort='v:refname' "$upstream" 2>/dev/null | tail -n1 | sed 's#.*/##')
  fi
  if [ -z "$tag" ]; then echo "!! $name: could not resolve a ref, skipping"; continue; fi

  tmp=$(mktemp -d)
  if ! git clone --quiet --depth 1 --branch "$tag" "$upstream" "$tmp/src" 2>/dev/null; then
    echo "!! $name: clone of $tag failed, skipping"; rm -rf "$tmp"; continue
  fi
  sha=$(git -C "$tmp/src" rev-parse HEAD)
  mkdir -p "vendor/$name"
  rsync -a --delete --exclude='.git' "$tmp/src/" "vendor/$name/"
  rm -rf "$tmp"

  yq -i "(.mcps[$i].ref) = \"$tag\" | (.mcps[$i].commit) = \"$sha\"" fleet.yaml
  [ "$tag" != "$cur" ] && echo ">> $name: $cur -> $tag ($sha)" || echo "== $name: $tag ($sha)"
done

git add -A vendor fleet.yaml
