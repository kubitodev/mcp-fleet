# Contributing

Thanks for your interest in contributing to jellyfin-mcp.

## Prerequisites

- [Go 1.25+](https://go.dev/dl/)
- A running Jellyfin server (10.8+) with an API key for testing
- [golangci-lint](https://golangci-lint.run/welcome/install/) (optional, for linting)

## Getting started

```sh
git clone https://github.com/jaredtrent/jellyfin-mcp.git
cd jellyfin-mcp

# Build for your platform
make build-local

# Run tests
make test

# Run vet + lint + tests
make check
```

## Project layout

```
main.go                          CLI entry point (cobra)
internal/jellyfin/               Jellyfin API client, types, helpers
internal/server/                 MCP server init, middleware, subscriptions
internal/server/tools/           31 tools organized into 8 toolsets
internal/server/resources/       13 live data resources + reference guides
internal/server/prompts/         18 guided workflows
npm/                             npm package for linux/x64 distribution
```

## Running locally

```sh
cp .env.example .env
# Edit .env with your Jellyfin URL and API key

source .env
./build/jellyfin-mcp
```

Or in HTTP mode:

```sh
source .env
./build/jellyfin-mcp --http
# Server at http://127.0.0.1:8080/mcp
```

## Testing

```sh
make test          # Run all tests
make vet           # Static analysis
make lint          # golangci-lint (if installed)
make check         # All of the above
```

Tests use a mock Jellyfin client (`internal/server/tools/mock_client_test.go`) so no real server is needed.

## Submitting changes

1. Fork the repo and create a branch from `main`
2. Make your changes
3. Run `make check` and fix any issues
4. Open a pull request with a clear description of what changed and why

Keep PRs focused — one feature or fix per PR. If you're planning a large change, open an issue first to discuss the approach.

## Adding a new tool

1. Pick the right file in `internal/server/tools/` based on the toolset it belongs to
2. Write the handler function following the existing pattern (input struct, client call, formatted output)
3. Register it in the corresponding `Register*Tools` function with the correct annotation (`AnnotReadOnly`, `AnnotWriteOp`, `AnnotWriteCreate`, or `AnnotDestructive`)
4. Add the tool name to `ToolsetMap` in `tools.go`
5. Update `internal/server/tools/README.md` with the new tool
6. Add tests

## Code style

- Follow standard Go conventions (`gofmt`, `go vet`)
- Keep tool handlers self-contained — each tool should be understandable on its own
- Use the shared annotation presets in `tools.go` rather than inline annotations
- Prefer returning structured output via the `Format*` helpers in `internal/jellyfin/`
