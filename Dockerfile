# Build stage
# Pinned to 1.25.7 for GO-2026-4337 (TLS session resumption fix)
FROM golang:1.25.7-alpine AS builder

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /build

# Cache dependency downloads
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Security: vulnerability check at build time
RUN go install golang.org/x/vuln/cmd/govulncheck@latest && \
    govulncheck ./...

# Security: vet check
RUN go vet ./...

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -trimpath \
    -o /blog \
    ./cmd/blog

# Final stage — distroless for minimal attack surface
FROM gcr.io/distroless/static:nonroot

COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /blog /blog

# Default volumes
VOLUME ["/data/content", "/data/config"]

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/blog"]
CMD ["serve"]
