# ── Stage 1: Build ──────────────────────────────────────────────────────────
# Multi-stage builds keep production images small — only the binary ships.
FROM golang:1.22-alpine AS builder

WORKDIR /src
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /kubelab-api .

# ── Stage 2: Runtime ────────────────────────────────────────────────────────
# Distroless/scratch images reduce attack surface. Alpine is a good middle ground.
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app && adduser -S app -G app

WORKDIR /app
COPY --from=builder /kubelab-api /app/kubelab-api

USER app
EXPOSE 8080

# HEALTHCHECK is used by Docker; in K8s you use liveness/readiness probes instead.
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/kubelab-api"]
