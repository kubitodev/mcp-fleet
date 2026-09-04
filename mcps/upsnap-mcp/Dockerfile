# syntax=docker/dockerfile:1
# First-party MCP (source in this repo, not vendored). Static binary → distroless
# nonroot. Build context = mcps/upsnap-mcp.
FROM golang:1.27-alpine AS build
WORKDIR /src
# module cache layer
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/upsnap-mcp .

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/upsnap-mcp /upsnap-mcp
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/upsnap-mcp"]
