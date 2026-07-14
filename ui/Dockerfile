# Copyright IBM Corp. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0

FROM --platform=$BUILDPLATFORM node:22-slim AS builder

WORKDIR /app

COPY package*.json ./
RUN --mount=type=cache,target=/root/.npm npm ci

COPY . .
RUN npm run build

# Production stage
FROM node:22-slim AS runner

WORKDIR /app

ENV NODE_ENV=production
ENV PORT=3000
ENV HOSTNAME=0.0.0.0
ENV BACKEND_URL=http://localhost:8080

# OCI image metadata — VERSION is injected by the CI build-push-action.
ARG VERSION
LABEL org.opencontainers.image.title="fabric-x-block-explorer-ui" \
      org.opencontainers.image.description="Hyperledger Fabric block explorer Next.js UI" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.source="https://github.com/LF-Decentralized-Trust-labs/fabric-x-block-explorer" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.vendor="LF Decentralized Trust"

COPY --from=builder --chown=node:node /app/.next/standalone ./
COPY --from=builder --chown=node:node /app/.next/static ./.next/static

USER node
EXPOSE 3000

CMD ["node", "server.js"]
