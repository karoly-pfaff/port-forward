#!/usr/bin/env bash
# Build a portable macOS release archive.
# Output: build/releases/macos/portier-portable-macos-<version>.tar.gz
#
# Usage:
#   bash scripts/macos/release/build-release.sh [--no-package] [--version VERSION]
#
# Options:
#   --no-package     Skip npm run package:portier; use existing build/portier/.
#   --version V      Override version string (default: reads from package.json).
#
# Note: .pkg creation (pkgbuild/productbuild) requires macOS with Xcode Command
# Line Tools. This script produces a portable tar.gz on any platform. If
# pkgbuild is available, a .pkg can be added in a future release.
# See docs/installer-strategy.md for the full macOS distribution plan.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
PACKAGE_DIR="$REPO_ROOT/build/portier"
OUTPUT_DIR="$REPO_ROOT/build/releases/macos"

VERSION=""
NO_PACKAGE=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --no-package) NO_PACKAGE=1;  shift ;;
    --version)    VERSION="$2";  shift 2 ;;
    *) echo "Unknown argument: $1" >&2; exit 1 ;;
  esac
done

# Read version from package.json if not provided
if [ -z "$VERSION" ]; then
  PKG_JSON="$REPO_ROOT/package.json"
  if [ ! -f "$PKG_JSON" ]; then
    echo "Error: package.json not found at $PKG_JSON. Pass --version manually." >&2
    exit 1
  fi
  if command -v node >/dev/null 2>&1; then
    VERSION="$(node -e "process.stdout.write(require('$PKG_JSON').version)")"
  elif command -v python3 >/dev/null 2>&1; then
    VERSION="$(python3 -c "import json; f=open('$PKG_JSON'); d=json.load(f); print(d['version'], end='')")"
  else
    echo "Error: node or python3 required to read version. Pass --version manually." >&2
    exit 1
  fi
fi

echo "Portier macOS release build"
echo "  Version    : $VERSION"
echo "  Source     : $PACKAGE_DIR"
echo "  Output dir : $OUTPUT_DIR"
echo ""

if [ -z "$NO_PACKAGE" ]; then
  echo "Running npm run package:portier..."
  npm --prefix "$REPO_ROOT" run package:portier
  echo ""
else
  echo "Skipping package build (--no-package)."
fi

# Verify required files in build/portier/
for req in service server.js "web/index.html" readme.txt; do
  if [ ! -e "$PACKAGE_DIR/$req" ]; then
    echo "Error: Required file not found in build/portier/: $req" >&2
    echo "  Run: npm run package:portier" >&2
    exit 1
  fi
done

mkdir -p "$OUTPUT_DIR"

ARCHIVE_NAME="portier-portable-macos-${VERSION}.tar.gz"
ARCHIVE_PATH="$OUTPUT_DIR/$ARCHIVE_NAME"
TMP_STAGE="$OUTPUT_DIR/_stage-$$"

# Stage clean layout
mkdir -p "$TMP_STAGE"
cp "$PACKAGE_DIR/service"    "$TMP_STAGE/service"
cp "$PACKAGE_DIR/server.js"  "$TMP_STAGE/server.js"
cp "$PACKAGE_DIR/readme.txt" "$TMP_STAGE/readme.txt"
cp -r "$PACKAGE_DIR/web"     "$TMP_STAGE/web"
chmod +x "$TMP_STAGE/service"

# Create archive
(cd "$OUTPUT_DIR" && tar -czf "$ARCHIVE_NAME" -C "$TMP_STAGE" .)
rm -rf "$TMP_STAGE"

echo ""
echo "Created: $ARCHIVE_PATH"
echo ""

if command -v pkgbuild >/dev/null 2>&1; then
  echo "Note: pkgbuild is available on this system."
  echo "  A .pkg installer can be added in a future release. See docs/installer-strategy.md."
else
  echo "Note: pkgbuild not available on this platform."
  echo "  Portable tar.gz produced. .pkg requires macOS with Xcode Command Line Tools."
  echo "  See docs/installer-strategy.md for the macOS .pkg follow-up."
fi
echo ""
echo "Archive contents:"
tar -tzf "$ARCHIVE_PATH"
