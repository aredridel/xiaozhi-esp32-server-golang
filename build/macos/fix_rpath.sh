#!/usr/bin/env bash

set -euo pipefail

# For macOS release packages: Change the rpath from the development machine's absolute source path to a path relative to the executable.
# Applicable directory structure:
#   ./xiaozhi_server
#   ./ten-vad/lib/macOS/ten_vad.framework

if [[ $# -ne 1 ]]; then
  echo "Usage: $0 <xiaozhi_server binary path>" >&2
  exit 1
fi

BIN_PATH="$1"
TARGET_RPATH="@executable_path/ten-vad/lib/macOS"

if [[ ! -f "$BIN_PATH" ]]; then
  echo "Binary does not exist: $BIN_PATH" >&2
  exit 1
fi

if ! command -v otool >/dev/null 2>&1; then
  echo "Missing otool, please install Xcode Command Line Tools" >&2
  exit 1
fi

if ! command -v install_name_tool >/dev/null 2>&1; then
  echo "Missing install_name_tool, please install Xcode Command Line Tools" >&2
  exit 1
fi

CURRENT_RPATHS=()
while IFS= read -r line; do
  CURRENT_RPATHS+=("$line")
done < <(
  otool -l "$BIN_PATH" | awk '
    $1 == "cmd" && $2 == "LC_RPATH" { in_rpath = 1; next }
    in_rpath && $1 == "path" { print $2; in_rpath = 0 }
  '
)

if [[ ${#CURRENT_RPATHS[@]} -eq 0 ]]; then
  echo "No LC_RPATH detected, preparing to write target rpath directly"
fi

for rpath in "${CURRENT_RPATHS[@]}"; do
  if [[ "$rpath" == "$TARGET_RPATH" ]]; then
    continue
  fi
  install_name_tool -delete_rpath "$rpath" "$BIN_PATH" 2>/dev/null || true
done

if ! otool -l "$BIN_PATH" | grep -Fq "path $TARGET_RPATH "; then
  install_name_tool -add_rpath "$TARGET_RPATH" "$BIN_PATH"
fi

echo "Written rpath: $TARGET_RPATH"
