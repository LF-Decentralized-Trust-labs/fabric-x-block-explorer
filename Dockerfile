# Copyright IBM Corp. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0

FROM golang:1.26-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /explorer ./cmd/explorer

FROM alpine:3.21
WORKDIR /app
COPY --from=builder /explorer /usr/local/bin/explorer
COPY config.yaml ./
ENTRYPOINT ["explorer"]
CMD ["start"]
