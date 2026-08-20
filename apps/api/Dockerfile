# syntax=docker/dockerfile:1
# Cloudflare Containers require linux/amd64. Runtime config is injected via
# Container envVars (Deploy/api) or compose — never COPY a .env into the image.
FROM --platform=linux/amd64 golang:1.25.12-alpine@sha256:56961d79ea8129efddcc0b8643fd8a5416b4e6228cfd477e3fd61deb2672c587 AS builder

WORKDIR /build

# Allow go.mod toolchain directive if image patch lags briefly.
ENV GOTOOLCHAIN=auto

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-w -s" -o server ./cmd/server

# Runtime image with poppler-utils for PDF page rendering (pdftoppm).
# Alpine is used instead of scratch to provide the pdftoppm binary needed by
# the ingestion pipeline to render real PDF page images.
FROM --platform=linux/amd64 alpine:3.21

RUN apk add --no-cache ca-certificates poppler-utils poppler-data font-noto-emoji ttf-dejavu tzdata \
    && addgroup -g 65532 -S appgroup \
    && adduser -u 65532 -S appuser -G appgroup

WORKDIR /app

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder --chown=65532:65532 /build/server /app/server

ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
ENV TZ=UTC
ENV PORT=8080

USER 65532:65532

EXPOSE 8080

STOPSIGNAL SIGTERM

HEALTHCHECK --interval=30s --timeout=5s --start-period=60s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null || exit 1

ENTRYPOINT ["/app/server"]
