# syntax=docker/dockerfile:1

# ── Build stage ───────────────────────────────────────────────────────────────
# Build on the runner's native platform and cross-compile for the target, so
# multi-arch images don't need QEMU emulation.
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build
ARG TARGETOS TARGETARCH
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Reported by -version and to MCP clients; the release workflow sets them.
# Declared after the module download so changing them keeps that layer cached.
ARG VERSION=dev
ARG COMMIT=unknown
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
      -ldflags="-s -w -X github.com/philosowaffle/kroger-cart/internal/server.Version=${VERSION} -X github.com/philosowaffle/kroger-cart/internal/server.Commit=${COMMIT}" \
      -o /out/kroger-mcp . \
    && mkdir -p /out/data

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot
# The release workflow sets this too; repeated here so local builds carry it.
LABEL org.opencontainers.image.licenses="GPL-3.0-only"
WORKDIR /
COPY --from=build /out/kroger-mcp /kroger-mcp
# /data holds state.json (linked Kroger account tokens, preferred store). Owned
# by nonroot so a fresh named volume inherits writable permissions.
COPY --from=build --chown=nonroot:nonroot /out/data /data

ENV KROGER_MCP_ADDR=:8080 \
    KROGER_MCP_DATA_DIR=/data
EXPOSE 8080
VOLUME ["/data"]
USER nonroot:nonroot

# The binary probes itself: distroless has no shell or curl.
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD ["/kroger-mcp", "-healthcheck"]

ENTRYPOINT ["/kroger-mcp"]
