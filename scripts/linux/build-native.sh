#!/usr/bin/env bash
# Build a Linux runtime package under build/linux/.
# Output layout:
#   build/linux/
#     service          (Go binary, linux/amd64)
#     server.js        (Node/TypeScript server bundle — fallback, requires Node.js)
#     web/             (built React client)
#     readme.txt

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
OUTPUT_DIR="${1:-$REPO_ROOT/build/linux}"

ESBUILD="$REPO_ROOT/node_modules/.bin/esbuild"
BUNDLE_ENTRY="$REPO_ROOT/server/sources/index.ts"
BUNDLE_WORK="$OUTPUT_DIR/_bundle"
BUNDLE_CJS="$BUNDLE_WORK/server.cjs"

echo "Building Linux package → $OUTPUT_DIR"
echo ""

mkdir -p "$OUTPUT_DIR"
mkdir -p "$BUNDLE_WORK"

echo "Building shared package..."
npm --prefix "$REPO_ROOT" run build:shared

echo "Building server package..."
npm --prefix "$REPO_ROOT" run build:server

echo "Building client package..."
npm --prefix "$REPO_ROOT" run build:client

echo "Bundling server as server.js..."
"$ESBUILD" "$BUNDLE_ENTRY" \
  --bundle \
  --minify \
  --platform=node \
  --format=cjs \
  --target=node22 \
  --outfile="$BUNDLE_CJS"
cp "$BUNDLE_CJS" "$OUTPUT_DIR/server.js"

echo "Building Go service for Linux (linux/amd64)..."
pushd "$REPO_ROOT/service" > /dev/null
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o "$OUTPUT_DIR/service" ./sources
popd > /dev/null

echo "Copying web UI..."
rm -rf "$OUTPUT_DIR/web"
cp -r "$REPO_ROOT/client/build" "$OUTPUT_DIR/web"

cat > "$OUTPUT_DIR/readme.txt" << 'EOF'
Portier Linux Portable Package
================================

This portable archive contains the Portier runtime files only. It does not
install OS services. Use the install scripts from the Portier repository to
set up a systemd service.

Native service (preferred):
  ./service --service --config /etc/portier/rules.json --host 127.0.0.1 --port 47831 --static-dir ./web

Node server (fallback, requires Node.js):
  node ./server.js --service --config /etc/portier/rules.json --host 127.0.0.1 --port 47831 --static-dir ./web

Options:
  --config <path>     Path to rules.json (required; not bundled in this archive)
  --host <addr>       Management host (default: 127.0.0.1)
  --port <port>       Management port (default: 47831)
  --static-dir <dir>  Path to web UI directory (use ./web from this directory)

Default management URL:
  http://127.0.0.1:47831

Config (rules.json) is external and must be provided. A new empty rules.json is
created automatically if the path does not exist when the service starts.

systemd service install (requires root):
  sudo bash scripts/linux/service/install-service.sh    (from the Portier repository)
  Installs to /opt/portier/ with config at /etc/portier/rules.json.
  See deploy/systemd/readme.md for full options.

Forwarded listen ports may need firewall rules (ufw, iptables, firewalld).
EOF

rm -rf "$BUNDLE_WORK"

echo ""
echo "Linux package created at $OUTPUT_DIR"
echo "  Go service : $OUTPUT_DIR/service"
echo "  Node server: $OUTPUT_DIR/server.js"
echo "  Web UI     : $OUTPUT_DIR/web"
