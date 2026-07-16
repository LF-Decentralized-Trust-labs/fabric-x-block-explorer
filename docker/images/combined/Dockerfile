#
# Copyright IBM Corp. All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0
#
# Combined image: Go REST API backend + Next.js UI in a single container.
#
# Build context: repo root (.)
#
# The Go binary is pre-built by CI (make build-release) and copied from the
# release/ directory — matching the fabric-x-committer convention of keeping
# the Go toolchain out of the final image.
#
# The Next.js UI is built inside Docker from ui/ source (node builder stage).
#
# Both processes are started by start.sh:
#   - Go backend on :8080
#   - Next.js UI  on :3000 (proxies /api/* to the backend)

###########################################
# Stage 1: Build the Next.js UI
###########################################
FROM --platform=$BUILDPLATFORM node:22-slim AS ui-builder

WORKDIR /app

COPY ui/package*.json ./
RUN --mount=type=cache,target=/root/.npm npm ci

COPY ui/ .
RUN npm run build

###########################################
# Stage 2: Production runtime image
###########################################
FROM node:22-slim AS runner

ARG TARGETARCH

# Label args
ARG VERSION=1.0
ARG CREATED
ARG REVISION=1.0

# ── Install wget for healthchecks (mirrors what the existing backend image uses) ──
RUN apt-get update && apt-get install -y --no-install-recommends wget && rm -rf /var/lib/apt/lists/*

# ── Non-root user ──────────────────────────────────────────────────────────────
# uid 10001 matches the fabric-x-committer / existing release image convention.
RUN useradd -u 10001 -r -g root -s /sbin/nologin \
        -c "Hyperledger Fabric-X explorer user" explorer && \
    mkdir -p /home/explorer && \
    chown -R 10001:0 /home/explorer && \
    chmod 0755 /home/explorer

# ── Go backend binary (pre-built by CI via make build-release) ─────────────────
COPY release/linux-${TARGETARCH}/explorer /bin/explorer

# ── Next.js standalone output ──────────────────────────────────────────────────
COPY --from=ui-builder --chown=10001:0 /app/.next/standalone /app/
COPY --from=ui-builder --chown=10001:0 /app/.next/static     /app/.next/static

# ── Entrypoint script ──────────────────────────────────────────────────────────
COPY docker/images/combined/start.sh /app/start.sh
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
