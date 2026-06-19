#!/usr/bin/env bash
# Build macOS release artifacts: a portable tar.gz and (on macOS) a native .pkg.
#
# Output:
#   build/releases/macos/portier-portable-macos-<version>.tar.gz
#   build/releases/macos/Portier-<version>.pkg          (when pkgbuild is available)
#
# Usage:
#   bash scripts/macos/release/build-release.sh [--no-package] [--portable-only] [--version V]
#
# Options:
#   --no-package     Skip `npm run build:runtime`; use the existing build/portier/.
#   --portable-only  Build only the portable tar.gz; skip the .pkg.
#   --version V      Override version string (default: reads from package.json).
#
# The .pkg is a FILE-INSTALL package (pkgbuild) that lays the runtime layout under
# /usr/local/portier and bundles the canonical macOS LaunchAgent scripts under
# /usr/local/portier/service-scripts. It does NOT auto-install the LaunchAgent and
# never creates, overwrites, or migrates user config/data (rules.json, logs). It is
# unsigned. pkgbuild requires macOS with the Xcode Command Line Tools; on other
# platforms the .pkg step is skipped (the portable tar.gz is still produced).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
PACKAGE_DIR="$REPO_ROOT/build/portier"
OUTPUT_DIR="$REPO_ROOT/build/releases/macos"
PKG_INSTALL_LOCATION="/usr/local/portier"
PKG_IDENTIFIER="com.portier.portier"

VERSION=""
NO_PACKAGE=""
PORTABLE_ONLY=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --no-package)    NO_PACKAGE=1;     shift ;;
    --portable-only) PORTABLE_ONLY=1;  shift ;;
    --version)       VERSION="$2";     shift 2 ;;
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
  echo "Running npm run build:runtime..."
  npm --prefix "$REPO_ROOT" run build:runtime
  echo ""
else
  echo "Skipping runtime build (--no-package)."
fi

# Verify the full release layout in build/portier/ (matches the Slice-2 contract).
for req in portier service server.js "web/index.html" "api/openapi.json" readme.txt; do
  if [ ! -e "$PACKAGE_DIR/$req" ]; then
    echo "Error: Required file not found in build/portier/: $req" >&2
    echo "  Run: npm run build:runtime" >&2
    exit 1
  fi
done

mkdir -p "$OUTPUT_DIR"

# ── Stage the full runtime layout (portier, service, server.js, web/, api/, readme) ──
RUNTIME_STAGE="$(mktemp -d "$OUTPUT_DIR/_stage-runtime.XXXXXX")"
cleanup() { rm -rf "$RUNTIME_STAGE" "${PKG_STAGE:-}"; }
trap cleanup EXIT

cp -R "$PACKAGE_DIR/." "$RUNTIME_STAGE/"
chmod +x "$RUNTIME_STAGE/portier" "$RUNTIME_STAGE/service"

# ── Portable tar.gz ──
ARCHIVE_NAME="portier-portable-macos-${VERSION}.tar.gz"
(cd "$RUNTIME_STAGE" && tar -czf "$OUTPUT_DIR/$ARCHIVE_NAME" .)
echo ""
echo "Created: $OUTPUT_DIR/$ARCHIVE_NAME"

# ── Native .pkg (macOS only) ──
PKG_NAME="Portier-${VERSION}.pkg"
if [ -n "$PORTABLE_ONLY" ]; then
  echo "Skipping .pkg (--portable-only)."
elif command -v pkgbuild >/dev/null 2>&1; then
  PKG_STAGE="$(mktemp -d "$OUTPUT_DIR/_stage-pkg.XXXXXX")"
  cp -R "$RUNTIME_STAGE/." "$PKG_STAGE/"
  # Bundle the canonical macOS LaunchAgent scripts. The runtime binary is named
  # 'service', so the scripts go under 'service-scripts/' to avoid a name clash.
  mkdir -p "$PKG_STAGE/service-scripts"
  cp "$REPO_ROOT/scripts/macos/service/"*.sh "$PKG_STAGE/service-scripts/"
  cp "$REPO_ROOT/scripts/macos/service/com.portier.plist.example" "$PKG_STAGE/service-scripts/"

  pkgbuild \
    --root "$PKG_STAGE" \
    --identifier "$PKG_IDENTIFIER" \
    --version "$VERSION" \
    --install-location "$PKG_INSTALL_LOCATION" \
    "$OUTPUT_DIR/$PKG_NAME"

  echo ""
  echo "Created: $OUTPUT_DIR/$PKG_NAME"
  echo "  File-install package (unsigned). Install location: $PKG_INSTALL_LOCATION"
  echo "  Bundles canonical LaunchAgent scripts under service-scripts/."
  echo "  Does NOT auto-install the LaunchAgent and does NOT touch user config/data."
else
  echo ""
  echo "Note: pkgbuild not available — skipping .pkg (requires macOS with Xcode Command Line Tools)."
  echo "  Portable tar.gz produced. The .pkg is built on macOS by 'npm run build:release:current'."
fi

echo ""
echo "macOS release build complete."
