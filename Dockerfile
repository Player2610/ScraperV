# syntax=docker/dockerfile:1

# ─────────────────────────────────────────────────────────────────────────────
# Base builder stage: shared by all targets
# ─────────────────────────────────────────────────────────────────────────────
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Cache dependency downloads separately from source
COPY go.mod go.sum ./
RUN go mod download

# Copy all source code
COPY . .

# ─────────────────────────────────────────────────────────────────────────────
# api target
# ─────────────────────────────────────────────────────────────────────────────
FROM builder AS api-builder
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM alpine:3.19 AS api
RUN apk add --no-cache ca-certificates tzdata
COPY --from=api-builder /out/api /usr/local/bin/api
USER nobody
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/api"]

# ─────────────────────────────────────────────────────────────────────────────
# scraper target
# ─────────────────────────────────────────────────────────────────────────────
FROM builder AS scraper-builder
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/scraper ./cmd/scraper

FROM alpine:3.19 AS scraper
RUN apk add --no-cache ca-certificates tzdata
COPY --from=scraper-builder /out/scraper /usr/local/bin/scraper
USER nobody
ENTRYPOINT ["/usr/local/bin/scraper"]

# ─────────────────────────────────────────────────────────────────────────────
# migrate target
# ─────────────────────────────────────────────────────────────────────────────
FROM builder AS migrate-builder
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

FROM alpine:3.19 AS migrate
RUN apk add --no-cache ca-certificates
COPY --from=migrate-builder /out/migrate /usr/local/bin/migrate
USER nobody
ENTRYPOINT ["/usr/local/bin/migrate"]
