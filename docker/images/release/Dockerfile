#
# Copyright IBM Corp. All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0
#
# Combined image: Go REST API backend + Next.js UI in a single container.
#
# Build context: repo root (.)
#
# The Go binary is built inside Docker (go-builder stage) so the build is
# fully self-contained and does not depend on the Go version installed on
# the CI runner. This matches the pattern used by fabric-x-committer and
# fabric-x-orderer, and avoids failures caused by go.mod toolchain
# directives referencing unreleased Go versions in indirect dependencies.
#
# The Next.js UI is built inside Docker from ui/ source (node builder stage).
#
# Both processes are started by start.sh:
#   - Go backend on :8080
#   - Next.js UI  on :3000 (proxies /api/* to the backend)

###########################################
# Stage 1: Build the Go backend binary
###########################################
FROM --platform=$BUILDPLATFORM golang:1.24 AS go-builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETARCH
ARG TARGETOS=linux
ARG VERSION=dev

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags "-w -s -X github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer/pkg/cli.Version=${VERSION}" \
    -o /out/explorer ./cmd/explorer/

###########################################
# Stage 2: Build the Next.js UI
###########################################
FROM --platform=$BUILDPLATFORM node:22-alpine AS ui-builder

WORKDIR /app

COPY ui/package*.json ./
RUN --mount=type=cache,target=/root/.npm npm ci

COPY ui/ .
RUN npm run build

###########################################
# Stage 3: Production runtime image
###########################################
FROM node:22-alpine AS runner

# Label args
ARG VERSION=dev
ARG CREATED
ARG REVISION=dev

# ── Install wget for healthchecks ──────────────────────────────────────────────
RUN apk add --no-cache wget

# ── Non-root user ──────────────────────────────────────────────────────────────
# uid 10001 matches the fabric-x-committer / existing release image convention.
RUN adduser -u 10001 -D -g "" -s /sbin/nologin explorer && \
    mkdir -p /home/explorer && \
    chown -R 10001:0 /home/explorer && \
    chmod 0755 /home/explorer

# ── Go backend binary (built in go-builder stage) ─────────────────────────────
COPY --from=go-builder /out/explorer /bin/explorer

# ── Next.js standalone output ──────────────────────────────────────────────────
COPY --from=ui-builder --chown=10001:0 /app/.next/standalone /app/
COPY --from=ui-builder --chown=10001:0 /app/.next/static     /app/.next/static

# ── Entrypoint script ──────────────────────────────────────────────────────────
COPY docker/images/release/start.sh /app/start.sh
RUN chmod +x /app/start.sh

# ── OCI metadata labels ────────────────────────────────────────────────────────
LABEL org.opencontainers.image.created="${CREATED}" \
      org.opencontainers.image.description="Hyperledger Fabric-X Block Explorer — backend + UI." \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.revision="${REVISION}" \
      org.opencontainers.image.source="https://github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer" \
      org.opencontainers.image.title="fabric-x-block-explorer" \
      org.opencontainers.image.url="https://github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer" \
      org.opencontainers.image.version="${VERSION}"

EXPOSE 8080 3000

USER 10001
WORKDIR /app

ENV NODE_ENV=production
ENV PORT=3000
ENV HOSTNAME=0.0.0.0
# UI proxy target — both processes share the container network namespace.
ENV BACKEND_URL=http://localhost:8080

ENTRYPOINT ["/app/start.sh"]
