#!/usr/bin/env bash
# Build a native Linux package for Portier from the packaged runtime layout — either a
# Debian package (.deb, default) or an RPM (.rpm). The two share the same payload and the
# same file-install, disabled-unit, no-silent-service behavior; only the packaging tool and
# metadata differ. (The portable tar.gz is built separately by scripts/build-portable.js;
# this is the Linux counterpart to scripts/macos/release/build-release.sh's .pkg step.)
#
# Output:
#   build/releases/linux/portier_<version>_amd64.deb      (--format deb, default)
#   build/releases/linux/portier-<version>-1.x86_64.rpm   (--format rpm)
#
# Note: the two filenames follow each ecosystem's own (different) convention — this is
# intentional, not an inconsistency:
#   - .deb uses  name_version_arch.deb            and Debian's arch name "amd64".
#   - .rpm uses  name-version-release.arch.rpm    and RPM's arch name "x86_64" (same arch).
#     The trailing "-1" is the RPM Release (packaging revision; from `Release: 1` in the
#     spec) — bump it when re-packaging the same upstream version. It is mandatory in RPM
#     naming, so it is kept as-is.
#
# Usage:
#   bash scripts/linux/release/build-release.sh [--format deb|rpm] [--version V]
#                                               [--source-dir DIR] [--output-dir DIR]
#
# FILE-INSTALL package: lays the runtime under /opt/portier and installs a systemd unit
# (portier.service) left DISABLED. The maintainer scripts/scriptlets only `systemctl
# daemon-reload` (so the unit is visible) and print opt-in guidance; on package removal they
# stop/disable the unit so nothing runs against deleted files. It never enables/starts the
# service or creates/overwrites/migrates user config (/etc/portier/rules.json). The Go
# service binary is static (CGO disabled), so the package declares no dependencies.
#
# Requires the tool for the chosen format: dpkg-deb (the `dpkg`/Debian/Ubuntu) for .deb, or
# rpmbuild (the `rpm` package, native on Fedora/RHEL) for .rpm. Without it that format is
# skipped (exit 0) so the rest of the release flow still completes.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
SOURCE_DIR="$REPO_ROOT/build/portier"
OUTPUT_DIR="$REPO_ROOT/build/releases/linux"
FORMAT="deb"
VERSION=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --format)     FORMAT="$2";     shift 2 ;;
    --version)    VERSION="$2";    shift 2 ;;
    --source-dir) SOURCE_DIR="$2"; shift 2 ;;
    --output-dir) OUTPUT_DIR="$2"; shift 2 ;;
    *) echo "Unknown argument: $1" >&2; exit 1 ;;
  esac
done

case "$FORMAT" in
  deb|rpm) ;;
  *) echo "Error: --format must be 'deb' or 'rpm' (got '$FORMAT')." >&2; exit 1 ;;
esac

if [ -z "$VERSION" ]; then
  PKG_JSON="$REPO_ROOT/package.json"
  if command -v node >/dev/null 2>&1; then
    VERSION="$(node -e "process.stdout.write(require('$PKG_JSON').version)")"
  else
    echo "Error: node required to read version, or pass --version." >&2
    exit 1
  fi
fi

echo "Portier Linux .$FORMAT build"
echo "  Version    : $VERSION"
echo "  Source     : $SOURCE_DIR"
echo "  Output dir : $OUTPUT_DIR"
echo ""

# Per-format build tool. Skip gracefully (exit 0) when it is unavailable.
if [ "$FORMAT" = "deb" ]; then
  if ! command -v dpkg-deb >/dev/null 2>&1; then
    echo "Note: dpkg-deb not available — skipping .deb (requires Debian/Ubuntu)."
    echo "  The portable tar.gz remains the baseline Linux artifact."
    exit 0
  fi
else
  if ! command -v rpmbuild >/dev/null 2>&1; then
    echo "Note: rpmbuild not available — skipping .rpm (install the 'rpm' package or use a"
    echo "  Fedora/RHEL host). The .deb + portable tar.gz remain the baseline Linux artifacts."
    exit 0
  fi
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
STAGE="$(mktemp -d "${TMPDIR:-/tmp}/portier-$FORMAT.XXXXXX")"
RPM_TOPDIR=""
cleanup() { rm -rf "$STAGE" "${RPM_TOPDIR:-}"; }
trap cleanup EXIT

# ── Shared payload: runtime under /opt/portier + the disabled systemd unit ──
mkdir -p "$STAGE/opt/portier" "$STAGE/lib/systemd/system"
cp -R "$SOURCE_DIR/." "$STAGE/opt/portier/"
chmod 0755 "$STAGE/opt/portier/portier" "$STAGE/opt/portier/service"
cp "$REPO_ROOT/scripts/linux/service/portier.service.example" \
   "$STAGE/lib/systemd/system/portier.service"

if [ "$FORMAT" = "deb" ]; then
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
else
  # ── RPM: a spec whose %install copies the staged payload into the buildroot ──
  RPM_TOPDIR="$(mktemp -d "${TMPDIR:-/tmp}/portier-rpm-top.XXXXXX")"
  mkdir -p "$RPM_TOPDIR/BUILD" "$RPM_TOPDIR/RPMS" "$RPM_TOPDIR/SPECS"
  SPEC="$RPM_TOPDIR/SPECS/portier.spec"

  cat > "$SPEC" <<EOF
# Prebuilt static Go binaries — disable rpm's strip/debuginfo/build-id post-processing.
%global __os_install_post %{nil}
%global debug_package %{nil}
%define _build_id_links none

Name:           portier
Version:        $VERSION
# Release: RPM packaging revision (the "-1" in the filename); RPM convention, not the
# software version. BuildArch: RPM's name for the same 64-bit x86 arch the .deb calls amd64.
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

  rpmbuild --define "_topdir $RPM_TOPDIR" --define "payload $STAGE" -bb "$SPEC"

  # RPM filename = name-version-release.arch.rpm (rpmbuild's own output name). The "-1" is
  # the Release and "x86_64" is RPM's arch label (the .deb's amd64) — convention, kept as-is.
  RPM_NAME="portier-${VERSION}-1.x86_64.rpm"
  BUILT="$RPM_TOPDIR/RPMS/x86_64/$RPM_NAME"
  if [ ! -f "$BUILT" ]; then
    echo "Error: expected RPM not found at $BUILT" >&2
    exit 1
  fi
  cp "$BUILT" "$OUTPUT_DIR/$RPM_NAME"

  echo ""
  echo "Created: $OUTPUT_DIR/$RPM_NAME"
  echo "  File-install package. Installs /opt/portier + disabled systemd unit."
  echo "  Does NOT enable/start the service and does NOT touch user config/data."
fi
