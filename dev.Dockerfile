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
ARG GO_VERSION="1.25.6"
ARG GO_DISTRO="go${GO_VERSION}.linux-amd64"

########################################
# Runtime user configuration
ARG USER="telegram-scout"
ARG UID="1000"
ARG GID="1000"
ARG GOPATH="/home/${USER}/go"

################################################################################
# Go stage
FROM debian:trixie@sha256:2c91e484d93f0830a7e05a2b9d92a7b102be7cab562198b984a84fdbc7806d91 AS go-stage

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
FROM golangci/golangci-lint:v2.8@sha256:bebcfa63db7df53e417845ed61e4540519cf74fcba22793cdd174b3415a9e4e2 AS golangci-lint-stage

################################################################################
# Debian main stage
FROM debian:trixie@sha256:2c91e484d93f0830a7e05a2b9d92a7b102be7cab562198b984a84fdbc7806d91 AS main
ARG USER
ARG UID
ARG GID

# Update apt lists
RUN apt-get update -qq

# Gen locale
RUN apt-get install --no-install-recommends -y -qq locales && \
  echo "en_US.UTF-8 UTF-8" >/etc/locale.gen && \
  locale-gen en_US.UTF-8 && \
  update-locale LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8

# Install generic tools
RUN apt-get install --no-install-recommends -y -qq \
  bash-completion \
  build-essential \
  ca-certificates \
  curl \
  file \
  git \
  gnupg \
  gosu \
  iputils-ping \
  jq \
  less \
  man-db \
  nano \
  net-tools \
  opendoas \
  openssh-client \
  procps \
  shellcheck \
  tini \
  tree \
  wget \
  yq

# Install Docker tools
RUN apt-get install --no-install-recommends -y -qq \
  docker-cli \
  docker-buildx

# Install Go
COPY --from=go-stage /rootfs/ /
# Install GolangCI-Lint
COPY --from=golangci-lint-stage /usr/bin/golangci-lint /usr/local/bin/golangci-lint

########################################
# Create a non-root developing user and configure doas
RUN addgroup --gid "${GID}" "${USER}"
RUN adduser --uid "${UID}" --gid "${GID}" \
  --shell "/bin/bash" --disabled-password "${USER}"

RUN addgroup --gid 110 docker && usermod -aG docker "${USER}"

RUN printf "permit nopass nolog keepenv %s as root\n" "${USER}" >/etc/doas.conf && \
  chmod 400 /etc/doas.conf && \
  printf "%s\nset -e\n%s\n" "#!/bin/sh" "doas \$@" >/usr/local/bin/sudo && \
  chmod a+rx /usr/local/bin/sudo

COPY scripts/switch-user.sh /usr/local/bin/switch-user.sh
COPY scripts/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/switch-user.sh /usr/local/bin/entrypoint.sh

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
