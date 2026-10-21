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
# A Dockerfile to build a development container for TelegramScout.

########################################
# Go version
ARG GO_VERSION="1.27.3"
ARG GO_DISTRO="go${GO_VERSION}.linux-amd64"

########################################
# Runtime user configuration
# dev, because dev-base bakes the user it creates and every repository
# shares that image.
ARG USER="dev"
ARG UID="1000"
ARG GID="1000"
ARG GOPATH="/home/${USER}/go"

# The package mirror to build through. It answers on one network only, so
# resolving the name is the test for reaching it.
ARG APT_MIRROR="http://debian.lan.h3nc4.com"

################################################################################
# Go stage
FROM debian:13-slim@sha256:62bd423aeee912639ca8ee681c2b762cb3e25137e0a92b20be455898a9f034ad AS go-stage

ARG APT_MIRROR
RUN host="${APT_MIRROR#http://}"; \
  if [ -n "${host}" ] && getent hosts "${host}" >/dev/null 2>&1; then \
    sed -i "s|^URIs: http://deb.debian.org/|URIs: ${APT_MIRROR}/|" \
      /etc/apt/sources.list.d/debian.sources; \
  fi

RUN apt-get update && apt-get install -y --no-install-recommends \
  gnupg

ARG GO_VERSION
ARG GO_DISTRO

########################################
# Download and install Go
ADD "https://go.dev/dl/${GO_DISTRO}.tar.gz" /tmp/go.tar.gz
ADD "https://go.dev/dl/${GO_DISTRO}.tar.gz.asc" /tmp/go.tar.gz.asc
ADD "https://keyserver.ubuntu.com/pks/lookup?op=get&search=0x0F06FF86BEEAF4E71866EE5232EE5355A6BC6E42" "/google.asc"

RUN gpg --import /google.asc && \
  gpg --verify /tmp/go.tar.gz.asc /tmp/go.tar.gz

RUN mkdir -p /rootfs/usr/local && \
  tar -xzf /tmp/go.tar.gz -C /rootfs/usr/local

################################################################################
# GolangCI-Lint stage
FROM golangci/golangci-lint:v2.15@sha256:fbfcfd12a6f8d7f1b97b3362ca80fc2a244d46863148c73cb88e7793af96c481 AS golangci-lint-stage

################################################################################
# Debian main stage
FROM h3nc4/dev-base:debian-13@sha256:7e41f028629060d3c141d122ac49f0a169d350fecb8efb752d0aae2d33d57715 AS main

# dev-base ends as the dev user, and the steps below need root.
USER root

# Not inherited: dev-base sets it while building, and its squashed image does not
# carry it into the runtime environment.
ENV DEBIAN_FRONTEND=noninteractive

########################################
# What cgo needs to link, which dev-base does not carry
ARG APT_MIRROR
RUN host="${APT_MIRROR#http://}"; \
  if [ -n "${host}" ] && getent hosts "${host}" >/dev/null 2>&1; then \
    sed -i "s|^URIs: http://deb.debian.org/|URIs: ${APT_MIRROR}/|" \
      /etc/apt/sources.list.d/debian.sources; \
  fi

RUN apt-get update -qq && apt-get install --no-install-recommends -y -qq \
  build-essential

# Install Go
COPY --from=go-stage /rootfs/ /
# Install GolangCI-Lint
COPY --from=golangci-lint-stage /usr/bin/golangci-lint /usr/local/bin/golangci-lint

########################################
# Clean cache
RUN apt-get clean && rm -rf /var/lib/apt/lists/* && \
  if [ -n "${APT_MIRROR}" ]; then \
    sed -i "s|^URIs: ${APT_MIRROR}/|URIs: http://deb.debian.org/|" \
      /etc/apt/sources.list.d/debian.sources; \
  fi
RUN rm -rf /var/cache/* /var/log/* /tmp/*

################################################################################
# Final squash image.
FROM scratch AS final
ARG USER
ENV USER="${USER}" \
  LANG="en_US.UTF-8" \
  LC_ALL="en_US.UTF-8" \
  PATH="/usr/local/go/bin:${PATH}"

COPY --from=main / /

USER "${USER}"

ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/entrypoint.sh"]
CMD ["/usr/bin/sleep", "infinity"]
