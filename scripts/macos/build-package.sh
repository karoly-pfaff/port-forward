#!/usr/bin/env bash
# Build a macOS runtime package under build/macos/.
# Output layout:
#   build/macos/
#     service          (Go binary, darwin/amd64)
#     server.js        (Node/TypeScript server bundle — fallback, requires Node.js)
#     web/             (built React client)
#     readme.txt

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
OUTPUT_DIR="${1:-$REPO_ROOT/build/macos}"

ESBUILD="$REPO_ROOT/node_modules/.bin/esbuild"
BUNDLE_ENTRY="$REPO_ROOT/server/sources/index.ts"
BUNDLE_WORK="$OUTPUT_DIR/_bundle"
BUNDLE_CJS="$BUNDLE_WORK/server.cjs"

echo "Building macOS package → $OUTPUT_DIR"
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

echo "Building Go service for macOS (darwin/amd64)..."
pushd "$REPO_ROOT/service" > /dev/null
GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -o "$OUTPUT_DIR/service" ./sources
popd > /dev/null

echo "Copying web UI..."
rm -rf "$OUTPUT_DIR/web"
cp -r "$REPO_ROOT/client/build" "$OUTPUT_DIR/web"

cat > "$OUTPUT_DIR/readme.txt" << 'EOF'
Portier macOS Package
=====================

Go service (preferred):
  ./service --service --config "~/Library/Application Support/Portier/rules.json" --host 127.0.0.1 --port 47831 --static-dir ./web

Node server (fallback, requires Node.js):
  node ./server.js --service --config "~/Library/Application Support/Portier/rules.json" --host 127.0.0.1 --port 47831 --static-dir ./web

Install as a LaunchAgent from the repository:
  bash scripts/macos/install-launch-agent.sh --install-dir ~/Applications/Portier

Config file:
  Keep rules.json external. The recommended config path is:
  ~/Library/Application Support/Portier/rules.json

Default management URL:
  http://127.0.0.1:47831

Forwarded listen ports may need macOS Firewall rules.
EOF

rm -rf "$BUNDLE_WORK"

echo ""
echo "macOS package created at $OUTPUT_DIR"
echo "  Go service : $OUTPUT_DIR/service"
echo "  Node server: $OUTPUT_DIR/server.js"
echo "  Web UI     : $OUTPUT_DIR/web"
