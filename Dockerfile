# syntax=docker/dockerfile:1

# ── Build stage ───────────────────────────────────────────────────────────────
FROM golang:1.26-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/kroger-mcp . \
    && mkdir -p /out/data

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot
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
