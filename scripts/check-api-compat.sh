#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
BASELINE_VERSION=${API_BASELINE_VERSION:-v0.0.3}
APIDIFF_VERSION=${APIDIFF_VERSION:-v0.0.0-20260218203240-3dfff04db8fa}
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
	# v0.0.3 selected an older confstore release that no longer contains every
	# package imported by the tag. Override only the dependency version so the
	# released Sphere packages can be loaded for API inspection.
	go get github.com/go-sphere/confstore@v0.0.4 >/dev/null
	go mod download all
	go run "golang.org/x/exp/cmd/apidiff@$APIDIFF_VERSION" \
		-m -w "$OLD_API" "$MODULE_PATH"
)

(
	cd "$ROOT_DIR"
	go run "golang.org/x/exp/cmd/apidiff@$APIDIFF_VERSION" \
		-m -w "$NEW_API" "$MODULE_PATH"
)

CHANGES=$(
	go run "golang.org/x/exp/cmd/apidiff@$APIDIFF_VERSION" \
		-m -incompatible "$OLD_API" "$NEW_API"
)

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
