#!/usr/bin/env bash

# Copyright 2018 Google Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

# remember to set GITHUB_CLIENT_ID and GITHUB_CLIENT_SECRET in the environment now
# Or use your preferred way to set them as environment variables.

TARGET_FILE="./main.go"

BACKUP_FILE="$(mktemp)"
cp "$TARGET_FILE" "$BACKUP_FILE"
function restore {
	cp "$BACKUP_FILE" "$TARGET_FILE"
	rm -f "$BACKUP_FILE"
}
trap restore EXIT

# Build the sample bar with all the keys set. Pass all arguments to the
# `go build` command, allowing e.g. `./build.sh -o ~/bin/mybar`, or even
# `./build.sh -race -tags debuglog`.
go build "$@"

