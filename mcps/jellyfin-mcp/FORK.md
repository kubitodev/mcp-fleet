# Fork notice

Adopted from [jaredtrent/jellyfin-mcp](https://github.com/jaredtrent/jellyfin-mcp)
at **v2026.604.2** (`54af5a4fe47fa2c4a0d2df23f5e7dfeaec311176`), MIT licensed.
The upstream `LICENSE` and copyright are retained unchanged.

## Why we adopted it

Jellyfin 12.0 disabled the deprecated sign-in methods. The upstream client
authenticates with the `X-MediaBrowser-Token` header, which the server now
rejects, so every tool call returned 401 against a 12.x server. Upstream's last
code commit was 2026-06-05 and there is no fix in progress.

It was previously an archive-only (`build: false`) entry. Patching in place under
`vendor/` was rejected: `scripts/sync.sh` re-vendors whenever the upstream ref
moves, which would silently revert the fix and bring the 401s back with no
signal. Moving it to `mcps/` makes ownership explicit and rebuilds it on every
source change.

## Local changes

- `internal/jellyfin/client.go` — authenticate with
  `Authorization: MediaBrowser Token="<key>"` instead of the
  `X-MediaBrowser-Token` header, via a new `authHeader` helper used by both
  request paths. This scheme works on 10.x and 12.x alike, so there is no
  version branch.

Everything else is upstream as vendored. The Go module path is still
`github.com/jaredtrent/jellyfin-mcp` — changing it would churn every import for
no benefit, and the `Dockerfile` ldflags reference it.

## Verified

Against a live Jellyfin **12.1** server (`lscr.io/linuxserver/jellyfin:version-12.1ubu2604`):

| scheme | result |
| --- | --- |
| `X-MediaBrowser-Token: <key>` | 401 |
| `X-Emby-Token: <key>` | 401 |
| `?api_key=<key>` | 401 |
| `Authorization: Bearer <key>` | 401 |
| `Authorization: MediaBrowser Token="<key>"` | **200** |

The patched client returns `/System/Info` successfully; `go build`, `go vet` and
the upstream test suite all pass.
