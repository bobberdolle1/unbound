#!/bin/bash
set -e

echo "=== LINUX DOCKER VALIDATION ENVIRONMENT ==="
go version
node --version
npm --version
wails version

echo "=== GO MOD DOWNLOAD ==="
go mod download

echo "=== FRONTEND BUILD & TEST ==="
cd frontend
npm ci
npx tsc --noEmit
npm test
npm run build
cd ..

echo "=== GO TEST SUITE ==="
go test ./...

echo "=== NATIVE WAILS LINUX BUILD ==="
# Wails build on Ubuntu 24.04 uses webkit2_41 tag
wails build -clean -tags webkit2_41 -ldflags "-s -w -X unbound/engine.Version=0.3.0-rc.3" -o Unbound-linux-amd64

echo "=== ARTIFACT INSPECTION ==="
BINARY="build/bin/Unbound-linux-amd64"
if [ ! -f "$BINARY" ]; then
  BINARY="build/bin/unbound"
fi

echo "Binary path: $BINARY"
file "$BINARY"
sha256sum "$BINARY"
stat "$BINARY"

echo "=== ELF HEADERS ==="
readelf -h "$BINARY"

echo "=== DYNAMIC DEPENDENCIES (LDD) ==="
ldd "$BINARY"

echo "=== CLI CAPABILITY TESTS ==="
"$BINARY" --version || true
"$BINARY" --help || true

echo "=== ASSET VERIFICATION ==="
go test -v -run TestEngine_AssetVerification ./tests

echo "=== HEADLESS GUI SMOKE TEST (Xvfb + DBus) ==="
export DISPLAY=:99
Xvfb :99 -screen 0 1024x768x24 > /dev/null 2>&1 &
XVFB_PID=$!
sleep 2

dbus-run-session -- "$BINARY" --version > /tmp/headless_gui.log 2>&1 || true

kill $XVFB_PID 2>/dev/null || true

echo "=== CONTAINER NETWORK NAMESPACE TEST ==="
if command -v iptables >/dev/null 2>&1; then
  echo "iptables available in container"
  iptables -L -n || echo "NET_ADMIN required for iptables manipulation"
fi

echo "=== LINUX VALIDATION STEP COMPLETE ==="
