#!/usr/bin/env bash
# Vendor each MCP's source into vendor/<name>/ — but ONLY when the resolved upstream
# ref differs from what's already vendored (or the dir is missing). Records ref+commit
# in fleet.yaml, and writes changed names to .changed (consumed by the build plan).
# Requires: yq (mikefarah), gh, git, rsync.
set -euo pipefail
cd "$(dirname "$0")/.."
: > .changed

count=$(yq '.mcps | length' fleet.yaml)
for i in $(seq 0 $((count - 1))); do
  name=$(yq -r ".mcps[$i].name" fleet.yaml)
  # first-party MCPs (source in-repo under mcps/) have no upstream to vendor
  if [ "$(yq -r ".mcps[$i].local // false" fleet.yaml)" = "true" ]; then
    echo "== $name: first-party (in-repo under mcps/), not vendored"; continue
  fi
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

  # skip if already at this ref and vendored — the "only if sha differs" bit
  if [ "$tag" = "$cur" ] && [ -d "vendor/$name" ]; then
    echo "== $name: up to date ($tag)"; continue
  fi

  tmp=$(mktemp -d)
  if ! git clone --quiet --depth 1 --branch "$tag" "$upstream" "$tmp/src" 2>/dev/null; then
    echo "!! $name: clone of $tag failed, skipping"; rm -rf "$tmp"; continue
  fi
  sha=$(git -C "$tmp/src" rev-parse HEAD)
  mkdir -p "vendor/$name"
  rsync -a --delete --exclude='.git' "$tmp/src/" "vendor/$name/"
  rm -rf "$tmp"

  yq -i "(.mcps[$i].ref) = \"$tag\" | (.mcps[$i].commit) = \"$sha\"" fleet.yaml
  echo ">> $name: $cur -> $tag ($sha)"
  echo "$name" >> .changed
done

git add -A vendor fleet.yaml
