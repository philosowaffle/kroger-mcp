# ── Build stage ───────────────────────────────────────────────────────────────
FROM golang:1.21-alpine AS builder
WORKDIR /build
COPY go.mod .
COPY cmd/ cmd/
RUN go build -o kroger-cart ./cmd/kroger-cart/

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /data
COPY --from=builder /build/kroger-cart /usr/local/bin/kroger-cart
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["kroger-cart"]
