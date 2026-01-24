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

# Called by CI to update version numbers in various files based on the Git tag.
version=${1#v}

# Update Sonar project version if file exists
if [ -f "sonar-project.properties" ]; then
  sed -i 's/sonar.projectVersion=.*/sonar.projectVersion='"${version}"'/' sonar-project.properties
fi

# Note: Go module versions are typically managed by git tags and don't require
# manual file updates like package.json, unless embedding version string in source.
