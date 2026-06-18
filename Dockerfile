# Copyright IBM Corp. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0

# ── Build stage ──────────────────────────────────────────────────────────────
FROM golang:1.26-alpine AS builder

# VERSION is injected into the binary via -ldflags so `explorer version`
# reports the real release tag. CI passes the git tag here.
ARG VERSION=dev

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-X github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/cli.Version=${VERSION}" \
    -o /explorer ./cmd/explorer

# ── Runtime stage ────────────────────────────────────────────────────────────
FROM alpine:3.21

ARG VERSION=dev

# OCI image labels — https://github.com/opencontainers/image-spec/blob/main/annotations.md
LABEL org.opencontainers.image.title="Fabric-X Block Explorer" \
      org.opencontainers.image.description="Block explorer for Hyperledger Fabric-X: ingests blocks from a sidecar into PostgreSQL and serves a REST API." \
      org.opencontainers.image.source="https://github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.version="${VERSION}"

WORKDIR /app
COPY --from=builder /explorer /usr/local/bin/explorer
# Copy only the deployment config. config.local.yaml is for local developer use
# only and is intentionally excluded from the image.
# docker-compose.yaml passes --config config.docker.yaml at runtime.
COPY config.docker.yaml ./

# Liveness probe: the REST server exposes GET /healthz (no DB call, instant 200).
# config.docker.yaml binds REST to 0.0.0.0:8080, so localhost works in-container.
# Uses busybox wget, which ships with the alpine base image.
HEALTHCHECK --interval=10s --timeout=5s --start-period=10s --retries=6 \
    CMD wget --quiet --tries=1 --spider http://localhost:8080/healthz || exit 1

ENTRYPOINT ["explorer"]
CMD ["start"]
