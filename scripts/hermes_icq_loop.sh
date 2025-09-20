#!/usr/bin/env bash
# Simple looped ICQ packet relay using Hermes tx commands.

SRC_CHAIN=${SRC_CHAIN:-bitora}
DST_CHAIN=${DST_CHAIN:-osmo-test-5}
SRC_PORT=${SRC_PORT:-icqcontroller}
SRC_CHANNEL=${SRC_CHANNEL:-channel-0}
DST_PORT=${DST_PORT:-icqhost}
DST_CHANNEL=${DST_CHANNEL:-channel-10925}
SLEEP_SECS=${SLEEP_SECS:-5}

echo "[hermes-icq-loop] Starting relay loop for $SRC_PORT/$SRC_CHANNEL <-> $DST_PORT/$DST_CHANNEL"

while true; do
  ./scripts/hermes_icq_once.sh || true
  sleep "$SLEEP_SECS"
done
