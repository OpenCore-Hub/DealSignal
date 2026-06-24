# syntax=docker/dockerfile:1
# Pinned builder image for reproducible builds.
FROM golang:1.25-alpine@sha256:523c3effe300580ed375e43f43b1c9b091b68e935a7c3a92bfcc4e7ed55b18c2 AS builder

WORKDIR /build

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o server ./cmd/server

# Runtime image with poppler-utils for PDF page rendering (pdftoppm).
# Alpine is used instead of scratch to provide the pdftoppm binary needed by
# the ingestion pipeline to render real PDF page images.
FROM alpine:3.21

RUN apk add --no-cache ca-certificates poppler-utils poppler-data font-noto-emoji ttf-dejavu tzdata \
    && addgroup -g 65532 -S appgroup \
    && adduser -u 65532 -S appuser -G appgroup

WORKDIR /app

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/server /app/server

ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
ENV TZ=UTC

USER 65532:65532

EXPOSE 8080

ENTRYPOINT ["/app/server"]
