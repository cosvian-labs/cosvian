#!/usr/bin/env bash
# Minimal one-shot ICQ packet relay using Hermes tx commands.
# Defaults target Bitora <-> Osmosis testnet ICQ channel on connection-0.

# NOTE: per user preference, avoid 'set -euo pipefail' to reduce brittleness.

SRC_CHAIN=${SRC_CHAIN:-bitora}
DST_CHAIN=${DST_CHAIN:-osmo-test-5}
SRC_PORT=${SRC_PORT:-icqcontroller}
SRC_CHANNEL=${SRC_CHANNEL:-channel-4}
DST_PORT=${DST_PORT:-icqhost}
DST_CHANNEL=${DST_CHANNEL:-channel-10897}

echo "[hermes-icq-once] Relaying recv packets bitora->osmo (if any)..."
hermes tx packet-recv \
  --dst-chain "$DST_CHAIN" \
  --src-chain "$SRC_CHAIN" \
  --src-port "$SRC_PORT" \
  --src-channel "$SRC_CHANNEL" || true

echo "[hermes-icq-once] Relaying recv packets osmo->bitora (if any)..."
hermes tx packet-recv \
  --dst-chain "$SRC_CHAIN" \
  --src-chain "$DST_CHAIN" \
  --src-port "$DST_PORT" \
  --src-channel "$DST_CHANNEL" || true

echo "[hermes-icq-once] Relaying acknowledgements osmo->bitora (if any)..."
hermes tx packet-ack \
  --src-chain "$DST_CHAIN" \
  --dst-chain "$SRC_CHAIN" \
  --src-port "$DST_PORT" \
  --src-channel "$DST_CHANNEL" || true

echo "[hermes-icq-once] Relaying acknowledgements bitora->osmo (if any)..."
hermes tx packet-ack \
  --src-chain "$SRC_CHAIN" \
  --dst-chain "$DST_CHAIN" \
  --src-port "$SRC_PORT" \
  --src-channel "$SRC_CHANNEL" || true

echo "[hermes-icq-once] Done."
