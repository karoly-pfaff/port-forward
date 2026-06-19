#!/usr/bin/env bash
# Build the native Linux RPM package (.rpm) for Portier from the packaged runtime layout.
# Mirrors the .deb (scripts/linux/release/build-release.sh) — same payload, same file-install,
# disabled-unit, no-silent-service behavior. The portable tar.gz is built separately by
# scripts/build-portable.js.
#
# Output:
#   build/releases/linux/portier-<version>-1.x86_64.rpm
#
# Usage:
#   bash scripts/linux/release/build-rpm.sh [--version V] [--source-dir DIR] [--output-dir DIR]
#
# FILE-INSTALL package: lays the runtime under /opt/portier and installs a systemd unit
# (portier.service) left DISABLED. Scriptlets only `systemctl daemon-reload` and print opt-in
# guidance; on erasure they stop/disable the unit. It never enables/starts the service or
# creates/overwrites/migrates user config (/etc/portier/rules.json). The Go service binary is
# static (CGO disabled) — AutoReqProv is off so the package declares no dependencies.
#
# Requires rpmbuild (the `rpm` package on Debian/Ubuntu, or a Fedora/RHEL host). Without it the
# .rpm step is skipped (exit 0) so the rest of the release flow still completes.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
SOURCE_DIR="$REPO_ROOT/build/portier"
OUTPUT_DIR="$REPO_ROOT/build/releases/linux"
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

echo "Portier Linux .rpm build"
echo "  Version    : $VERSION"
echo "  Source     : $SOURCE_DIR"
echo "  Output dir : $OUTPUT_DIR"
echo ""

if ! command -v rpmbuild >/dev/null 2>&1; then
  echo "Note: rpmbuild not available — skipping .rpm (install the 'rpm' package or use a"
  echo "  Fedora/RHEL host). The .deb + portable tar.gz remain the baseline Linux artifacts."
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
TOPDIR="$(mktemp -d "${TMPDIR:-/tmp}/portier-rpm.XXXXXX")"
cleanup() { rm -rf "$TOPDIR"; }
trap cleanup EXIT

mkdir -p "$TOPDIR/BUILD" "$TOPDIR/RPMS" "$TOPDIR/SPECS"

# ── Payload staged exactly like the .deb: /opt/portier + /lib/systemd/system unit ──
PAYLOAD="$TOPDIR/payload"
mkdir -p "$PAYLOAD/opt/portier" "$PAYLOAD/lib/systemd/system"
cp -R "$SOURCE_DIR/." "$PAYLOAD/opt/portier/"
chmod 0755 "$PAYLOAD/opt/portier/portier" "$PAYLOAD/opt/portier/service"
cp "$REPO_ROOT/scripts/linux/service/portier.service.example" \
   "$PAYLOAD/lib/systemd/system/portier.service"

SPEC="$TOPDIR/SPECS/portier.spec"
cat > "$SPEC" <<EOF
# Prebuilt static Go binaries — disable rpm's strip/debuginfo/build-id post-processing.
%global __os_install_post %{nil}
%global debug_package %{nil}
%define _build_id_links none

Name:           portier
Version:        $VERSION
Release:        1
Summary:        Local TCP/UDP port-forwarding manager
License:        MIT
BuildArch:      x86_64
AutoReqProv:    no

%description
Portier is a local TCP/UDP port-forwarding manager for development and LAN testing.
This package installs the runtime under /opt/portier and a systemd unit (portier.service)
that is left disabled. It does not start or enable the service, and never creates,
overwrites, or migrates user config/data. Run 'sudo systemctl enable --now portier' to opt in.

%install
mkdir -p %{buildroot}
cp -a %{payload}/. %{buildroot}/

%files
/opt/portier
/lib/systemd/system/portier.service

# %post: make the unit visible + print opt-in guidance. NEVER enables/starts. (\$1>=1 = install/upgrade)
%post
mkdir -p /etc/portier
if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload || true
fi
echo "Portier installed to /opt/portier (systemd unit 'portier.service', disabled)."
echo "Config is external at /etc/portier/rules.json (created on first start)."
echo "Enable and start it with: sudo systemctl enable --now portier"

# %preun: on final erasure only (\$1==0), stop+disable so nothing runs against deleted files.
%preun
if [ "\$1" = "0" ]; then
  if command -v systemctl >/dev/null 2>&1; then
    systemctl stop portier.service 2>/dev/null || true
    systemctl disable portier.service 2>/dev/null || true
  fi
fi

# %postun: reload units after erasure. User config under /etc/portier is preserved.
%postun
if [ "\$1" = "0" ]; then
  if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload 2>/dev/null || true
  fi
fi
EOF

rpmbuild --define "_topdir $TOPDIR" --define "payload $PAYLOAD" -bb "$SPEC"

RPM_NAME="portier-${VERSION}-1.x86_64.rpm"
BUILT="$TOPDIR/RPMS/x86_64/$RPM_NAME"
if [ ! -f "$BUILT" ]; then
  echo "Error: expected RPM not found at $BUILT" >&2
  exit 1
fi
cp "$BUILT" "$OUTPUT_DIR/$RPM_NAME"

echo ""
echo "Created: $OUTPUT_DIR/$RPM_NAME"
echo "  File-install package. Installs /opt/portier + disabled systemd unit."
echo "  Does NOT enable/start the service and does NOT touch user config/data."
