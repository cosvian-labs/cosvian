#!/usr/bin/env bash
# Build and run Cosvian with ICQ async tag, start Hermes ICQ loop, and tail logs for osmosisicq.

set -euo pipefail

export GOFLAGS=${GOFLAGS:-"-tags icq_async"}

pushd "$(dirname "$0")/.." >/dev/null

echo "Building cosviand with icq_async tag..."
go build -tags icq_async -mod=readonly -o build/cosviand ./cmd/cosviand

echo "Starting hermes ICQ loop (background)..."
nohup ./scripts/hermes_icq_loop.sh > /tmp/hermes_icq_loop.log 2>&1 &
echo $! > /tmp/hermes_icq_loop.pid

echo "Run node separately if not already running, then to observe events:\n  tail -f ~/.cosvian/logs/app.log | grep -E 'osmosisicq_|icq_'\n  cosviand q osmosisicq status\n"

popd >/dev/null

echo "[ERROR] The Osmosis-based pricefeed test harness has been removed." >&2
echo "Use the updated oracle/pricefeed test flows instead." >&2
exit 1
