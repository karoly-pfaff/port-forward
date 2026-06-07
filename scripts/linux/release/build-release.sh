#!/usr/bin/env bash
# Build a portable Linux release archive.
# Usage: bash scripts/linux/release/build-release.sh [--no-package] [--version VERSION]
#
# Output: build/releases/linux/portier-<version>-linux.tar.gz
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

NO_PACKAGE=""
VERSION_OVERRIDE=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --no-package) NO_PACKAGE=1;         shift   ;;
    --version)    VERSION_OVERRIDE="$2"; shift 2 ;;
    *) echo "Unknown argument: $1" >&2; exit 1 ;;
  esac
done

# Resolve version from package.json
if [ -n "$VERSION_OVERRIDE" ]; then
  VERSION="$VERSION_OVERRIDE"
elif command -v node >/dev/null 2>&1; then
  VERSION="$(node -e "process.stdout.write(require('$REPO_ROOT/package.json').version)")"
elif command -v python3 >/dev/null 2>&1; then
  VERSION="$(python3 -c "import json,sys; print(json.load(open('$REPO_ROOT/package.json'))['version'], end='')")"
else
  echo "Error: node or python3 required to read version from package.json." >&2
  echo "  Pass --version VERSION to override." >&2
  exit 1
fi

PACKAGE_DIR="$REPO_ROOT/build/portier"
OUTPUT_DIR="$REPO_ROOT/build/releases/linux"
ARCHIVE_NAME="portier-${VERSION}-linux.tar.gz"

echo "Portier Linux release archive"
echo "  Version  : $VERSION"
echo "  Output   : $OUTPUT_DIR/$ARCHIVE_NAME"
echo ""

# Build package unless --no-package
if [ -z "$NO_PACKAGE" ]; then
  echo "Running npm run build:runtime..."
  npm --prefix "$REPO_ROOT" run build:runtime
  echo ""
else
  echo "Skipping runtime build (--no-package)."
  echo ""
fi

# Verify required files
for f in "service" "server.js" "web/index.html" "readme.txt"; do
  if [ ! -e "$PACKAGE_DIR/$f" ]; then
    echo "Error: Required file missing from $PACKAGE_DIR: $f" >&2
    echo "  Run: npm run build:runtime" >&2
    exit 1
  fi
done

# Stage the archive contents
TMP_STAGE="$(mktemp -d)"
trap 'rm -rf "$TMP_STAGE"' EXIT

cp -r "$PACKAGE_DIR/." "$TMP_STAGE/"
chmod +x "$TMP_STAGE/service"

# Create output directory and archive
mkdir -p "$OUTPUT_DIR"
(cd "$TMP_STAGE" && tar -czf "$OUTPUT_DIR/$ARCHIVE_NAME" .)

echo "Archive created: $OUTPUT_DIR/$ARCHIVE_NAME"
echo ""
echo "Contents:"
tar -tzf "$OUTPUT_DIR/$ARCHIVE_NAME"
echo ""
echo "Install on a target Linux machine:"
echo "  tar -xzf $ARCHIVE_NAME -C /opt/portier/"
echo "  sudo bash scripts/linux/service/install-service.sh --source-dir /opt/portier"
