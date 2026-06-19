#!/usr/bin/env bash
# Build the native Linux release installer: a Debian package (.deb) for Portier from
# the packaged runtime layout. (The portable tar.gz is built by scripts/build-portable.js;
# this is the Linux counterpart to scripts/macos/release/build-release.sh's .pkg step.)
#
# Output:
#   build/releases/linux/portier_<version>_amd64.deb
#
# Usage:
#   bash scripts/linux/release/build-release.sh [--version V] [--source-dir DIR] [--output-dir DIR]
#
# This is a FILE-INSTALL package (dpkg-deb). It lays the runtime under /opt/portier and
# installs a systemd unit (portier.service) that is left DISABLED. It never enables,
# starts, creates, overwrites, or migrates user config/data (rules.json). The maintainer
# scripts only run `systemctl daemon-reload` (so the unit is visible) and print how to
# opt in; on package removal they stop/disable the unit so nothing is left running
# against deleted binaries. The Go service binary is static (CGO disabled), so the
# package has no shared-library dependency.
#
# Requires dpkg-deb (Debian/Ubuntu). On a host without it the .deb step is skipped
# (exit 0) so the rest of the release flow still completes; the portable tar.gz is the
# baseline Linux artifact. .rpm is a planned later track.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
SOURCE_DIR="$REPO_ROOT/build/portier"
OUTPUT_DIR="$REPO_ROOT/build/releases/linux"
INSTALL_PREFIX="opt/portier"
VERSION=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version)    VERSION="$2";    shift 2 ;;
    --source-dir) SOURCE_DIR="$2"; shift 2 ;;
    --output-dir) OUTPUT_DIR="$2"; shift 2 ;;
    *) echo "Unknown argument: $1" >&2; exit 1 ;;
  esac
done

if [ -z "$VERSION" ]; then
  PKG_JSON="$REPO_ROOT/package.json"
  if command -v node >/dev/null 2>&1; then
    VERSION="$(node -e "process.stdout.write(require('$PKG_JSON').version)")"
  else
    echo "Error: node required to read version, or pass --version." >&2
    exit 1
  fi
fi

echo "Portier Linux .deb build"
echo "  Version    : $VERSION"
echo "  Source     : $SOURCE_DIR"
echo "  Output dir : $OUTPUT_DIR"
echo ""

if ! command -v dpkg-deb >/dev/null 2>&1; then
  echo "Note: dpkg-deb not available — skipping .deb (requires Debian/Ubuntu)."
  echo "  The portable tar.gz remains the baseline Linux artifact."
  exit 0
fi

# Verify the full release layout in the source dir (matches the Slice-2 contract).
for req in portier service server.js "web/index.html" "api/openapi.json" readme.txt; do
  if [ ! -e "$SOURCE_DIR/$req" ]; then
    echo "Error: Required file not found in $SOURCE_DIR: $req" >&2
    echo "  Run: npm run build:runtime" >&2
    exit 1
  fi
done

mkdir -p "$OUTPUT_DIR"
STAGE="$(mktemp -d "${TMPDIR:-/tmp}/portier-deb.XXXXXX")"
cleanup() { rm -rf "$STAGE"; }
trap cleanup EXIT

# ── Payload: runtime under /opt/portier ──
mkdir -p "$STAGE/$INSTALL_PREFIX"
cp -R "$SOURCE_DIR/." "$STAGE/$INSTALL_PREFIX/"
chmod 0755 "$STAGE/$INSTALL_PREFIX/portier" "$STAGE/$INSTALL_PREFIX/service"

# ── systemd unit (disabled; canonical template) ──
mkdir -p "$STAGE/lib/systemd/system"
cp "$REPO_ROOT/scripts/linux/service/portier.service.example" \
   "$STAGE/lib/systemd/system/portier.service"

# ── Debian control + maintainer scripts ──
mkdir -p "$STAGE/DEBIAN"

cat > "$STAGE/DEBIAN/control" <<EOF
Package: portier
Version: $VERSION
Section: net
Priority: optional
Architecture: amd64
Maintainer: Portier <noreply@portier.invalid>
Description: Local TCP/UDP port-forwarding manager
 Portier is a local TCP/UDP port-forwarding manager for development and LAN
 testing. This package installs the runtime under /opt/portier and a systemd
 unit (portier.service) that is left disabled. It does not start or enable the
 service, and never creates, overwrites, or migrates user config/data. Run
 'sudo systemctl enable --now portier' to opt in.
EOF

# postinst: make the unit visible and print opt-in guidance. NEVER enables/starts.
cat > "$STAGE/DEBIAN/postinst" <<'EOF'
#!/bin/sh
set -e
if [ "$1" = "configure" ]; then
  mkdir -p /etc/portier
  if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload || true
  fi
  echo "Portier installed to /opt/portier (systemd unit 'portier.service', disabled)."
  echo "Config is external at /etc/portier/rules.json (created on first start)."
  echo "Enable and start it with: sudo systemctl enable --now portier"
fi
exit 0
EOF

# prerm: on package removal only, stop+disable so nothing runs against deleted files.
cat > "$STAGE/DEBIAN/prerm" <<'EOF'
#!/bin/sh
set -e
if [ "$1" = "remove" ]; then
  if command -v systemctl >/dev/null 2>&1; then
    systemctl stop portier.service 2>/dev/null || true
    systemctl disable portier.service 2>/dev/null || true
  fi
fi
exit 0
EOF

# postrm: reload units after removal. User config under /etc/portier is preserved.
cat > "$STAGE/DEBIAN/postrm" <<'EOF'
#!/bin/sh
set -e
if [ "$1" = "remove" ] || [ "$1" = "purge" ]; then
  if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload 2>/dev/null || true
  fi
fi
exit 0
EOF

chmod 0755 "$STAGE/DEBIAN/postinst" "$STAGE/DEBIAN/prerm" "$STAGE/DEBIAN/postrm"

DEB_NAME="portier_${VERSION}_amd64.deb"
# --root-owner-group makes payload root:root without needing fakeroot/root.
dpkg-deb --root-owner-group --build "$STAGE" "$OUTPUT_DIR/$DEB_NAME"

echo ""
echo "Created: $OUTPUT_DIR/$DEB_NAME"
echo "  File-install package. Installs /opt/portier + disabled systemd unit."
echo "  Does NOT enable/start the service and does NOT touch user config/data."
