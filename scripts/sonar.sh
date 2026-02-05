#!/bin/sh
#
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

set -e

cd "$(dirname "$0")/../"

if [ "$1" = "-i" ]; then
  skip_scan="1"
fi

sonar_container_name="sonarqube"
sonar_image="sonarqube:26.2.0.119303-community"
sonar_scan_image="sonarsource/sonar-scanner-cli:12"
sonar_url="http://localhost:9000"

cfg_sonar() {
  curl --fail -su admin:admin -X POST "${sonar_url}/api/$1" >/dev/null
}

# Pull scanner image in background
if [ -z "${skip_scan}" ]; then
  docker pull "${sonar_scan_image}" >/dev/null 2>&1 &
  pull_pid=$!
fi

is_running="$(docker ps -q -f "name=${sonar_container_name}")"
if [ -z "${is_running}" ]; then
  is_exited="$(docker ps -aq -f status=exited -f "name=${sonar_container_name}")"
  if [ -n "${is_exited}" ]; then
    docker start "${sonar_container_name}" >/dev/null
  else
    docker run -d \
      --name "${sonar_container_name}" \
      -p 0.0.0.0:9000:9000 \
      -v sonarqube_data:/opt/sonarqube/data \
      -v sonarqube_extensions:/opt/sonarqube/extensions \
      -v sonarqube_logs:/opt/sonarqube/logs \
      -e SONAR_ES_BOOTSTRAP_CHECKS_DISABLE=true \
      "${sonar_image}" >/dev/null
    unconfigured="1"
  fi

  echo "Waiting for SonarQube to start..."
  while ! wget -qO- "${sonar_url}/api/system/status" 2>/dev/null | grep -q 'UP'; do
    sleep 1
  done

  if [ -n "${unconfigured}" ]; then
    echo "Configuring SonarQube..."

    # Configure SonarQube to allow anonymous access
    cfg_sonar "settings/set?key=sonar.forceAuthentication&value=false"
    cfg_sonar "permissions/add_group?permission=provisioning&groupName=anyone"
    cfg_sonar "permissions/add_group?permission=scan&groupName=anyone"

    # Create quality gate
    cfg_sonar "qualitygates/create?name=TelegramScoutGate"
    cfg_sonar "qualitygates/create_condition?gateName=TelegramScoutGate&metric=violations&op=GT&error=0"
    cfg_sonar "qualitygates/create_condition?gateName=TelegramScoutGate&metric=security_hotspots_reviewed&op=LT&error=100"
    cfg_sonar "qualitygates/create_condition?gateName=TelegramScoutGate&metric=coverage&op=LT&error=0"
    cfg_sonar "qualitygates/create_condition?gateName=TelegramScoutGate&metric=duplicated_lines_density&op=GT&error=0"
    cfg_sonar "qualitygates/set_as_default?name=TelegramScoutGate"
  fi
fi

echo "SonarQube is running at ${sonar_url}"
if [ -n "${skip_scan}" ]; then
  exit 0
fi

echo "Running SonarQube analysis..."

wait "${pull_pid}"
docker run \
  --rm \
  --network="host" \
  -v "${TELEGRAMSCOUT_HOST_ROOT:-${PWD}}/:/usr/src" \
  "${sonar_scan_image}" \
  -Dsonar.host.url="${sonar_url}" \
  -Dsonar.go.coverage.reportPaths=coverage.out \
  -Dsonar.qualitygate.wait=true
