# syntax=docker/dockerfile:1
# Pinned builder image for reproducible builds.
FROM golang:1.25-alpine@sha256:523c3effe300580ed375e43f43b1c9b091b68e935a7c3a92bfcc4e7ed55b18c2 AS builder

WORKDIR /build

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o server ./cmd/server

# Create a writable /tmp directory for the scratch runtime.
FROM alpine:3.21 AS tmp
RUN mkdir -p /tmp && chmod 1777 /tmp

# Empty runtime image — only the static binary and CA bundle are included.
# This removes the distroless Debian base layer and eliminates Trivy OS-package
# findings while keeping the container non-root.
FROM scratch

WORKDIR /app

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/server /app/server
COPY --chown=65532:65532 --from=tmp /tmp /tmp

ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt

USER 65532:65532

EXPOSE 8080

ENTRYPOINT ["/app/server"]
