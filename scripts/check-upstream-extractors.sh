#!/usr/bin/env bash
# Usage: ./scripts/check-upstream-extractors.sh  — reports drift between upstream kepano/defuddle TS extractors and local Go ports
set -euo pipefail

UPSTREAM_INFRA="^(_base|_conversation|bbcode-data|types|base|index|registry)$"

# Fetch upstream extractor names: .ts only, strip extension, exclude infra
upstream_list=$(
  gh api repos/kepano/defuddle/contents/src/extractors --jq '.[].name' \
    | grep '\.ts$' \
    | sed 's/\.ts$//' \
    | grep -Ev "$UPSTREAM_INFRA" \
    | sed 's/-/_/g' \
    | sort
)

# Enumerate canonical local ports by the BaseExtractor Name method. The owning
# filename is the port name; helper and split files have no Name method.
local_list=$(
	find "$(cd "$(dirname "$0")/.." && pwd)/extractors" -maxdepth 1 -type f -name '*.go' ! -name '*_test.go' \
		-exec awk '/^func \([^)]*\) Name\(\) string/ { print FILENAME; nextfile }' {} + \
	| sed 's#.*/##; s/\.go$//' \
	| sort
)

missing=$(comm -23 <(echo "$upstream_list") <(echo "$local_list"))
local_only=$(comm -13 <(echo "$upstream_list") <(echo "$local_list"))
in_sync=$(comm -12 <(echo "$upstream_list") <(echo "$local_list"))

echo "=== Upstream Extractor Drift Report ==="
echo ""

indent() { while IFS= read -r line; do echo "  $line"; done; }

if [[ -n "$missing" ]]; then
  echo "--- Missing ports (upstream has, we don't) ---"
  echo "$missing" | indent
  echo ""
else
  echo "--- Missing ports: none ---"
  echo ""
fi

if [[ -n "$local_only" ]]; then
  echo "--- Local-only (fork-specific, OK) ---"
  echo "$local_only" | indent
  echo ""
fi

echo "--- In sync ($(echo "$in_sync" | grep -c .) ported) ---"
echo "$in_sync" | indent
echo ""

total_upstream=$(echo "$upstream_list" | grep -c .)
total_local=$(echo "$local_list" | grep -c .)
echo "Upstream: $total_upstream site extractors  |  Local ports: $total_local"

exit 0
