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
ARG GO_VERSION="1.27.1"
ARG GO_DISTRO="go${GO_VERSION}.linux-amd64"

########################################
# Runtime user configuration
# dev, because dev-base bakes the user it creates and every repository
# shares that image.
ARG USER="dev"
ARG UID="1000"
ARG GID="1000"
ARG GOPATH="/home/${USER}/go"

################################################################################
# Go stage
FROM debian:13@sha256:f324c7ff54321e8d9c588493a20244965938ce0aa50bbd1022d38010e9ffc4b1 AS go-stage

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
FROM golangci/golangci-lint:v2.13@sha256:ba07dffad130794ae79ebaa0056809d18c0168f3f846480ffd3eb6c04578b83d AS golangci-lint-stage

################################################################################
# Debian main stage
FROM h3nc4/dev-base:debian-13@sha256:7e16158a6bc18e5dc393f373a00521a0109d0b1ce0150ad6f949e416ce1a051f AS main

# dev-base ends as the dev user, and the steps below need root.
USER root

# Not inherited: dev-base sets it while building, and its squashed image does not
# carry it into the runtime environment.
ENV DEBIAN_FRONTEND=noninteractive

########################################
# What cgo needs to link, which dev-base does not carry
RUN apt-get update -qq && apt-get install --no-install-recommends -y -qq \
  build-essential

# Install Go
COPY --from=go-stage /rootfs/ /
# Install GolangCI-Lint
COPY --from=golangci-lint-stage /usr/bin/golangci-lint /usr/local/bin/golangci-lint

########################################
# Clean cache
RUN apt-get clean && rm -rf /var/lib/apt/lists/*
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
