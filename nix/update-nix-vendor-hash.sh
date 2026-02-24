#!/usr/bin/env bash

set -euo pipefail

PACKAGE_FILE="nix/package.nix"

update_hash() {
  local output="$1"
  local attempt="$2"
  
  if echo "$output" | grep -q "hash mismatch in fixed-output derivation"; then
    EXPECTED_HASH=$(echo "$output" | grep "specified:" | sed 's/.*specified: *//')
    GOT_HASH=$(echo "$output" | grep "got:" | sed 's/.*got: *//')
    
    echo "::notice::Attempt $attempt - Expected: $EXPECTED_HASH"
    echo "::notice::Attempt $attempt - Got:      $GOT_HASH"
    
    if echo "$output" | grep -q "go-modules.drv"; then
      echo "::notice::Updating vendorHash"
      sed -i.bak "s|$EXPECTED_HASH|$GOT_HASH|g" "$PACKAGE_FILE"
      rm -f "$PACKAGE_FILE.bak"
      echo "updated"
    elif echo "$output" | grep -q "npm-deps.drv"; then
      echo "::notice::Updating npmDeps hash"
      sed -i.bak "s|$EXPECTED_HASH|$GOT_HASH|g" "$PACKAGE_FILE"
      rm -f "$PACKAGE_FILE.bak"
      echo "updated"
    fi
  fi
}

echo "::notice::Running nix build to check for hash mismatches..."

# nix hash
echo "::group::Build attempt 1"
OUTPUT=$(nix build .#hister 2>&1 || true)
echo "$OUTPUT"
echo "::endgroup::"

RESULT1=$(update_hash "$OUTPUT" "1")

# npm hash
echo "::group::Build attempt 2"
OUTPUT=$(nix build .#hister 2>&1 || true)
echo "$OUTPUT"
echo "::endgroup::"

RESULT2=$(update_hash "$OUTPUT" "2")

if [ -n "$RESULT1" ] || [ -n "$RESULT2" ]; then
  echo "::notice::Updated vendorHash and/or npmDeps"
fi

echo "::group::Verifying final build"
nix build .#hister 2>&1 | tail -5
echo "::endgroup::"
echo "::notice::Build successful!"
