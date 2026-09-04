# mcp-fleet

A source-mirroring build hub for [Model Context Protocol](https://modelcontextprotocol.io)
servers. One repo that **vendors the source of every MCP it tracks** and **builds container
images for the ones upstream doesn't publish**.

## Why

Small community MCP servers can disappear or stop shipping images. This keeps the set
self-sufficient:

- **Archive** — `vendor/<name>/` holds each MCP's real source at a pinned release. If an
  upstream repo vanishes, the code is still here (auditable, patchable).
- **Build the gaps** — MCPs that ship no image (`build: true` in `fleet.yaml`) are built and
  pushed to `ghcr.io/<owner>/<name>`.
- **Rebuild insurance** — archive-only entries (`build: false`) use upstream's image today; if
  that image ever disappears, flip `build: true` and it rebuilds from the vendored source.

## How it works

One workflow, **`sync`** (weekly + manual):

1. Resolves each MCP's latest upstream release and re-vendors its source into `vendor/<name>/`
   **only when the ref differs** (records `ref` + `commit` in `fleet.yaml`, commits, pushes).
2. In the same run, builds every `build: true` entry **that changed** and publishes
   `ghcr.io/<owner>/<name>:<ref>` + `:latest`, with SBOM and build-provenance attestation.

Run it with **`force_build: true`** to rebuild all `build: true` images regardless of change
(bootstrap the first image, or refresh base images after a CVE).

## Adding or changing an MCP

Edit `fleet.yaml`:

```yaml
- name: something-mcp
  upstream: https://github.com/owner/something-mcp
  ref: v1.2.3          # vendor-sync auto-bumps this
  build: false         # true -> build+publish; add `dockerfile:` if upstream ships none
  image: ghcr.io/owner/something-mcp:v1.2.3   # informational, for build:false
  # pin: v1.2.3        # optional: freeze; vendor-sync won't auto-bump
```

Then run the `sync` workflow (or wait for the weekly run).

## Layout

```
fleet.yaml                     # the manifest (source of truth)
vendor/<name>/                 # mirrored upstream source (committed archive)
dockerfiles/<name>.Dockerfile  # for build:true MCPs whose upstream ships no Dockerfile
scripts/sync.sh                # vendoring logic (skips unchanged; writes .changed)
.github/workflows/             # sync.yml (vendor + build), renovate.yml
```

## Notes

- `vendor-sync` pushes to `main` unreviewed. To add a checkpoint, protect `main` and switch
  the workflow to open a PR instead.
- This repo's own glue (workflows, scripts, Dockerfiles) is MIT. Everything under `vendor/`
  retains its upstream project's license.
