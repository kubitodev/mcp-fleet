# Changelog

## v2026.604.2

### Security

- Update `github.com/modelcontextprotocol/go-sdk` to 1.6.1 (and `jsonschema-go` to 0.4.3), resolving two high-severity advisories:
  - **GHSA-q382-vc8q-7jhj** — improper handling of null Unicode characters during JSON parsing (pulls patched `segmentio/encoding` v0.5.4).
  - **CVE-2026-33252 / GHSA-89xv-2j6f-qhc8** — cross-site tool execution on the Streamable HTTP transport. 1.6.1 enforces `Content-Type: application/json` on POST and enables DNS-rebinding protection by default.
- Wrap the `/mcp` handler with `net/http` cross-origin protection so Origin / `Sec-Fetch-Site` is verified even in no-token localhost mode. Non-browser clients (MetaMCP, curl) are unaffected; browser cross-site POSTs are rejected with 403.

### Changed

- Bump Docker base images to `golang:1.26.4-alpine` (build) and `alpine:3.23` (runtime).
- Bump CI GitHub Actions to current majors (Node 24 runtime): `checkout` v6, `setup-go` v6, `setup-qemu` v4, `setup-buildx` v4, `login` v4, `build-push` v7, `metadata` v6.

## v2026.604.1

### Added

- **Docker support**: official multi-arch image (`linux/amd64`, `linux/arm64`) published to `ghcr.io/jaredtrent/jellyfin-mcp`, with a hardened `Dockerfile` and `docker-compose.yml`. See the Docker section of the README. Closes #1.

### Fixed

- Corrected `--http` / `--http-token` flag syntax (double dashes) in documentation and startup error messages.

## v2026.603.1

### Fixed

- **`jellyfin_play`**: send playback parameters as query parameters instead of a JSON request body. Jellyfin's `POST /Sessions/{id}/Playing` endpoint requires `playCommand` and `itemIds` as query params, so every play request previously failed with `400 — The playCommand field is required`. Playback now starts correctly. Thanks @perk11 for the fix (#2).

## v2026.318.7 — Initial release

- 31 tools across 8 toolsets: discovery, media, user, playback, admin, content, livetv, analytics
- 13 live MCP resources (server info, sessions, libraries, favorites, recently played, etc.)
- 18 guided prompt workflows (movie-night, troubleshoot, library-health, etc.)
- 10 built-in reference guides (transcoding, Docker, file naming, migration, etc.)
- Two transports: stdio and Streamable HTTP with bearer token auth
- Safety controls: read-only mode, disable-destructive, toolset scoping, confirmation gates
- Resource subscriptions with change-detection polling
- Auto-completion for prompt arguments and resource template URIs
- Structured MCP log notifications with timing data
- Platforms: Linux (x64, arm64), macOS (Apple Silicon, Intel), Windows (x64)
- npm package for MetaMCP and Docker-based gateways
