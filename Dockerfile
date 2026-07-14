# Copyright IBM Corp. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0

# Build on the native build platform and cross-compile to the target,
# so multi-arch builds (buildx) don't pay the cost of QEMU emulation.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -o /explorer ./cmd/explorer

FROM alpine:3.21

# OCI image metadata — values injected by the CI build-push-action.
ARG VERSION
ARG CREATED
ARG REVISION
LABEL org.opencontainers.image.title="fabric-x-block-explorer" \
      org.opencontainers.image.description="Lightweight Hyperledger Fabric block explorer backend (Go binary)" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.created="${CREATED}" \
      org.opencontainers.image.revision="${REVISION}" \
      org.opencontainers.image.source="https://github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.vendor="LF Decentralized Trust"

WORKDIR /app
COPY --from=builder /explorer /usr/local/bin/explorer
# Copy only the deployment config. config.local.yaml is for local developer use
# only and is intentionally excluded from the image.
# docker-compose.yaml passes --config config.docker.yaml at runtime.
COPY config.docker.yaml ./
ENTRYPOINT ["explorer"]
CMD ["start"]
