# Copyright (C) 2026  Henrique Almeida
# This file is part of TelegramScout.
#
# TelegramScout is free software: you can redistribute it and/or modify
# it under the terms of the GNU Affero General Public License as published
# by the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# TelegramScout is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU Affero General Public License for more details.
#
# You should have received a copy of the GNU Affero General Public License
# along with TelegramScout.  If not, see <https://www.gnu.org/licenses/>.

################################################################################
# A Dockerfile to build a runtime container for TelegramScout.

################################################################################
# Build stage
ARG DEV_IMAGE="h3nc4/telegram-scout-dev:8@sha256:198794496614fc5b91fb2271334ebaede567b3d11217d19e6802d04a9920e6d5"

FROM ${DEV_IMAGE} AS builder

USER 0:0
WORKDIR /app
COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY cmd/ ./cmd/
COPY internal/ ./internal/

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -o telegram-scout ./cmd/telegram-scout

################################################################################
# Runtime stage
FROM alpine:3.24@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6 AS runtime

# Install CA certificates for HTTPS
RUN apk add --no-cache ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/telegram-scout /rootfs/telegram-scout
# Copy CA certificates
RUN mkdir -p /rootfs/etc/ssl/certs/ && \
  cp /etc/ssl/certs/ca-certificates.crt /rootfs/etc/ssl/certs/ca-certificates.crt

################################################################################
# Final squashed image
FROM scratch AS final
ARG VERSION="dev"
ARG COMMIT_SHA="unknown"
ARG BUILD_DATE="unknown"

COPY --from=runtime /rootfs/ /

USER 65534:65534
CMD ["/telegram-scout"]

LABEL org.opencontainers.image.title="TelegramScout" \
  org.opencontainers.image.description="TelegramScout - Headless Telegram Monitor" \
  org.opencontainers.image.authors="Henrique Almeida <me@h3nc4.com>" \
  org.opencontainers.image.vendor="Henrique Almeida" \
  org.opencontainers.image.licenses="AGPL-3.0-or-later" \
  org.opencontainers.image.url="https://h3nc4.com" \
  org.opencontainers.image.source="https://github.com/h3nc4/TelegramScout" \
  org.opencontainers.image.documentation="https://github.com/h3nc4/TelegramScout/blob/main/README.md" \
  org.opencontainers.image.version="${VERSION}" \
  org.opencontainers.image.revision="${COMMIT_SHA}" \
  org.opencontainers.image.created="${BUILD_DATE}" \
  org.opencontainers.image.ref.name="${VERSION}"
