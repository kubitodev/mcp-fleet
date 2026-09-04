# syntax=docker/dockerfile:1
# Nosmoht/talos-mcp-server ships release binaries but no image — build from the
# vendored source. Static binary → distroless nonroot. Context = vendor/talos-mcp-server.
FROM golang:1.26-alpine AS build
WORKDIR /src
# module cache layer
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/talos-mcp-server ./cmd/talos-mcp

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/talos-mcp-server /talos-mcp-server
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/talos-mcp-server"]
