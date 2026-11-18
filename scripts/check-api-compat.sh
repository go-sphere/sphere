#!/usr/bin/env bash

set -euo pipefail

export GOWORK=off

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
resolve_baseline() {
	local revision=${1:-HEAD}
	if ! git -C "$ROOT_DIR" describe --tags --abbrev=0 --match 'v[0-9]*' "$revision" 2>/dev/null; then
		echo "Cannot resolve a baseline release tag from git history." >&2
		echo "A shallow clone without tags is the usual cause: actions/checkout needs fetch-depth: 0." >&2
		echo "Set API_BASELINE_VERSION to compare against a specific tag instead." >&2
		exit 1
	fi
}
# API_BASELINE_VERSION overrides the baseline; otherwise use the latest release
# tag, falling back to the previous tag when HEAD itself is the release.
if [[ -n "${API_BASELINE_VERSION:-}" ]]; then
	BASELINE_VERSION="$API_BASELINE_VERSION"
else
	BASELINE_VERSION=$(resolve_baseline)
	if git -C "$ROOT_DIR" tag --points-at HEAD | grep -Fxq "$BASELINE_VERSION"; then
		BASELINE_VERSION=$(resolve_baseline HEAD^)
	fi
fi
APIDIFF_VERSION=${APIDIFF_VERSION:-v0.0.0-20261113113407-e3970619a1a4}
MODULE_PATH=github.com/go-sphere/sphere
ALLOWLIST="$ROOT_DIR/compat/api-incompatibilities.txt"

if [[ ! -f "$ALLOWLIST" ]]; then
	echo "API compatibility allowlist not found: $ALLOWLIST" >&2
	exit 1
fi

TEMP_DIR=$(mktemp -d)
trap 'rm -rf "$TEMP_DIR"' EXIT

BASELINE_DIR="$TEMP_DIR/baseline"
OLD_API="$TEMP_DIR/old.api"
NEW_API="$TEMP_DIR/new.api"
UNEXPECTED="$TEMP_DIR/unexpected.txt"
mkdir -p "$BASELINE_DIR"

(
	cd "$BASELINE_DIR"
	go mod init sphere-api-baseline >/dev/null
	go get "$MODULE_PATH@$BASELINE_VERSION" >/dev/null
	resolved_version=$(go list -m -f '{{.Version}}' "$MODULE_PATH")
	if [[ "$resolved_version" != "$BASELINE_VERSION" ]]; then
		echo "Resolved baseline version $resolved_version does not match requested $BASELINE_VERSION" >&2
		exit 1
	fi
	go mod download all
	go run "golang.org/x/exp/cmd/apidiff@$APIDIFF_VERSION" \
		-m -w "$OLD_API" "$MODULE_PATH"
)

if [[ ! -s "$OLD_API" ]]; then
	echo "Baseline API snapshot is empty; $MODULE_PATH@$BASELINE_VERSION was not inspected" >&2
	exit 1
fi

(
	cd "$ROOT_DIR"
	go run "golang.org/x/exp/cmd/apidiff@$APIDIFF_VERSION" \
		-m -w "$NEW_API" "$MODULE_PATH"
)

if [[ ! -s "$NEW_API" ]]; then
	echo "Current API snapshot is empty; $MODULE_PATH was not inspected" >&2
	exit 1
fi

# The report goes to a file rather than a command substitution: a tool or
# module-resolution failure there would abort under set -e with the same exit
# status as "incompatible changes found", leaving CI unable to tell a broken
# check from a real API break. A non-zero status here is always the tool
# failing — apidiff reports changes on stdout and still exits 0.
CHANGES_FILE="$TEMP_DIR/changes.txt"
if ! go run "golang.org/x/exp/cmd/apidiff@$APIDIFF_VERSION" \
	-m -incompatible "$OLD_API" "$NEW_API" >"$CHANGES_FILE"; then
	echo "apidiff failed to compare the API snapshots (tool or module resolution error)" >&2
	exit 1
fi
CHANGES=$(<"$CHANGES_FILE")

: >"$UNEXPECTED"
while IFS= read -r change; do
	[[ -z "$change" ]] && continue
	if ! grep -Fqx -- "$change" "$ALLOWLIST"; then
		printf '%s\n' "$change" >>"$UNEXPECTED"
	fi
done <<<"$CHANGES"

if [[ -s "$UNEXPECTED" ]]; then
	echo "Unexpected incompatible API changes relative to $BASELINE_VERSION:" >&2
	cat "$UNEXPECTED" >&2
	exit 1
fi

echo "API compatibility check passed against $BASELINE_VERSION"
