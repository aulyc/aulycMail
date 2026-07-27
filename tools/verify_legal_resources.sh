#!/bin/bash
# Verify that an aulycMail app contains the required upstream legal materials.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
APP=""
SOURCE_ROOT=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --app)
      APP="$2"
      shift 2
      ;;
    --source-root)
      SOURCE_ROOT="$2"
      shift 2
      ;;
    *)
      echo "Unknown option: $1" >&2
      exit 2
      ;;
  esac
done

if [[ ! -d "$APP" ]]; then
  echo "--app must identify an existing application bundle." >&2
  exit 1
fi
if [[ -n "$SOURCE_ROOT" ]]; then
  SOURCE_ROOT="$(cd "$SOURCE_ROOT" && pwd)"
fi

LEGAL="$APP/Contents/Resources/Legal"
required_files=(
  "LICENSE.txt"
  "THIRD_PARTY_NOTICES.md"
  "Aerion-Apache-2.0.txt"
  "AERION_MODIFICATIONS.md"
  "Nunito-OFL.txt"
)

for required_file in "${required_files[@]}"; do
  if [[ ! -s "$LEGAL/$required_file" ]]; then
    echo "Missing or empty legal resource: $LEGAL/$required_file" >&2
    exit 1
  fi
done

if ! grep -Fq "Copyright 2024-2025 Aerion Contributors" \
  "$LEGAL/Aerion-Apache-2.0.txt"; then
  echo "Aerion license is missing the upstream copyright notice." >&2
  exit 1
fi
if ! grep -Fq "APPENDIX: How to apply the Apache License to your work." \
  "$LEGAL/Aerion-Apache-2.0.txt"; then
  echo "Aerion license is not the complete upstream license text." >&2
  exit 1
fi
if ! grep -Fq "https://github.com/hkdb/aerion" \
  "$LEGAL/THIRD_PARTY_NOTICES.md"; then
  echo "Third-party notices are missing the Aerion source attribution." >&2
  exit 1
fi
if ! grep -Fq "modified by aulyc beginning in 2026" \
  "$LEGAL/AERION_MODIFICATIONS.md"; then
  echo "Aerion modification notice is missing." >&2
  exit 1
fi
if ! grep -Fq "not affiliated with, sponsored by," \
  "$LEGAL/AERION_MODIFICATIONS.md"; then
  echo "Aerion non-endorsement notice is missing." >&2
  exit 1
fi

if [[ -n "$SOURCE_ROOT" ]]; then
  source_files=(
    "$SOURCE_ROOT/LICENSE"
    "$SOURCE_ROOT/THIRD_PARTY_NOTICES.md"
    "$SOURCE_ROOT/LICENSES/Aerion-Apache-2.0.txt"
    "$SOURCE_ROOT/AERION_MODIFICATIONS.md"
    "$SOURCE_ROOT/frontend/src/assets/fonts/OFL.txt"
  )

  for index in "${!required_files[@]}"; do
    source_file="${source_files[$index]}"
    bundled_file="$LEGAL/${required_files[$index]}"
    if [[ ! -f "$source_file" ]]; then
      echo "Missing source legal file: $source_file" >&2
      exit 1
    fi
    if ! cmp -s "$source_file" "$bundled_file"; then
      echo "Bundled legal resource differs from source: $bundled_file" >&2
      exit 1
    fi
  done
fi

echo "Verified required aulycMail legal resources."
