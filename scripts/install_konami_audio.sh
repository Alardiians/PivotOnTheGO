#!/usr/bin/env bash
set -euo pipefail

SRC="${HOME}/Downloads/konamisound.mp3"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEST_DIR="${SCRIPT_DIR}/../web/assets/media/.hidden"
DEST="${DEST_DIR}/konamisound.mp3"

if [ ! -f "${SRC}" ]; then
  echo "ERROR: Source file not found at: ${SRC}"
  echo "Please place konamisound.mp3 in your Downloads folder and re-run."
  exit 1
fi

mkdir -p "${DEST_DIR}"

cp "${SRC}" "${DEST}"

echo "Konami audio installed to: ${DEST}"
