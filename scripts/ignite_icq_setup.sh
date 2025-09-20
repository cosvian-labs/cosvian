#!/usr/bin/env bash
# Automates Hermes key setup, funding, and ICQ channel handshake for an Ignite-run Bitora node.
#
# Prerequisites:
#   - `ignite chain serve --reset-once --build.tags "icq_async"` is already running.
#   - `hermes`, `jq`, `curl`, `python3`, `go`, and `./bitorad` are available in PATH.
#   - Osmosis relayer account used below already has sufficient testnet funds.

set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT_DIR"

HERMES_BIN=${HERMES_BIN:-hermes}
BITORA_CHAIN=${BITORA_CHAIN:-bitora}
OSMO_CHAIN=${OSMO_CHAIN:-osmo-test-5}
BITORA_KEY=${BITORA_KEY:-bitora_relayer}
OSMO_KEY=${OSMO_KEY:-osmo-test-relayer}
BITORA_ADDR=${BITORA_ADDR:-bto1thzz35kr96tpdvpjkk0q0mahylckp5akzhc4c9}
OSMO_ADDR=${OSMO_ADDR:-osmo1au6l8aj9my5q4fg0drlq4hspzf6p9patea2w4t}
BITORA_HOME=${BITORA_HOME:-$HOME/.bitora}
NODE=${NODE:-http://localhost:26657}
FUND_SOURCE=${FUND_SOURCE:-public_sale}
FUND_AMOUNT=${FUND_AMOUNT:-5000000ubto}
FUND_MIN_BAL=${FUND_MIN_BAL:-3000000}
FUND_FEE=${FUND_FEE:-1000000ubto}
HERMES_CONFIG=${HERMES_CONFIG:-hermes/config.toml}
HERMES_ONCE_SCRIPT=${HERMES_ONCE_SCRIPT:-scripts/hermes_icq_once.sh}
GO_TEST_PKGS=${GO_TEST_PKGS:-./x/pricefeed/...}

BITORA_MNEMONIC="garbage surround humor wall nurse fitness hand you west ostrich quiz short report exact into hundred fashion minute cable olympic kidney security twenty group"
OSMO_MNEMONIC="festival height poverty much valid shine boy afford base cruel crouch autumn spice real fox trophy supreme unfair exit cancel height post code reduce"

FILES_DIR="$ROOT_DIR/hermes/keys"
BITORA_KEY_FILE="$FILES_DIR/${BITORA_KEY}.mnemonic"
OSMO_KEY_FILE="$FILES_DIR/${OSMO_KEY}.mnemonic"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "[ERROR] Required command '$1' not found in PATH" >&2
    exit 1
  fi
}

for cmd in "$HERMES_BIN" jq curl python3 go; do
  require_cmd "$cmd"
done
require_cmd "$ROOT_DIR/bitorad"

run_hermes() {
  local output
  if ! output=$("$HERMES_BIN" --config "$HERMES_CONFIG" "$@" 2>&1); then
    printf '%s\n' "$output" >&2
    return 1
  fi
  printf '%s\n' "$output"
}

mkdir -p "$FILES_DIR"
printf '%s\n' "$BITORA_MNEMONIC" > "$BITORA_KEY_FILE"
printf '%s\n' "$OSMO_MNEMONIC" > "$OSMO_KEY_FILE"

echo "[+] Checking Bitora RPC availability at $NODE ..."
STATUS_JSON=$(curl -sf "$NODE/status" || true)
if [[ -z "$STATUS_JSON" ]]; then
  echo "[ERROR] Unable to reach Bitora RPC at $NODE. Ensure ignite chain serve is running." >&2
  exit 1
fi
CHAIN_ID=$(echo "$STATUS_JSON" | jq -r '.result.node_info.network')
if [[ -z "$CHAIN_ID" || "$CHAIN_ID" == "null" ]]; then
  echo "[ERROR] Could not determine chain-id from RPC response." >&2
  exit 1
fi
printf '    Detected chain-id: %s\n' "$CHAIN_ID"

ensure_bitorad_key() {
  local name=$1
  local mnemonic=$2
  if ! ./bitorad keys show "$name" --home "$BITORA_HOME" --keyring-backend test >/dev/null 2>&1; then
    echo "[+] Importing $name into local keyring"
    { printf '%s\n' "$mnemonic"; printf '\n'; } | ./bitorad keys add "$name" --home "$BITORA_HOME" --keyring-backend test --recover >/dev/null
  fi
}

ensure_bitorad_key "$BITORA_KEY" "$BITORA_MNEMONIC"

if ! ./bitorad keys show "$FUND_SOURCE" --home "$BITORA_HOME" --keyring-backend test >/dev/null 2>&1; then
  if [[ -f "$FILES_DIR/${FUND_SOURCE}.mnemonic" ]]; then
    echo "[+] Recovering funding key '$FUND_SOURCE' from mnemonic file"
    { cat "$FILES_DIR/${FUND_SOURCE}.mnemonic"; printf '\n'; } | ./bitorad keys add "$FUND_SOURCE" --home "$BITORA_HOME" --keyring-backend test --recover >/dev/null
  else
    echo "[ERROR] Key '$FUND_SOURCE' not found. Provide its mnemonic at hermes/keys/${FUND_SOURCE}.mnemonic" >&2
    exit 1
  fi
fi

ensure_hermes_key() {
  local chain=$1
  local key=$2
  local file=$3
  if ! "$HERMES_BIN" keys list --chain "$chain" 2>/dev/null | grep -F "- ${key} (" >/dev/null 2>&1; then
    echo "[+] Restoring Hermes key '$key' on $chain"
    local hd_flag=()
    if [[ "$chain" == "$BITORA_CHAIN" ]]; then
      hd_flag=(--hd-path "m/44'/90'/0'/0/0")
    fi
    "$HERMES_BIN" keys add --chain "$chain" --mnemonic-file "$file" --key-name "$key" --overwrite "${hd_flag[@]}" >/dev/null
  else
    echo "[=] Hermes key '$key' already present on $chain"
  fi
}

ensure_hermes_key "$BITORA_CHAIN" "$BITORA_KEY" "$BITORA_KEY_FILE"
ensure_hermes_key "$OSMO_CHAIN" "$OSMO_KEY" "$OSMO_KEY_FILE"

update_channel_refs() {
  local src=$1
  local dst=$2
  if [[ -n "$src" ]]; then
    python3 - "$src" "$HERMES_CONFIG" <<'PY'
import sys, re
channel, path = sys.argv[1:3]
text = open(path).read()
text = re.sub(r"(list\s*=\s*\[\[\s*'icqcontroller',\s*')channel-\d+(')", r"\1"+channel+r"\2", text)
open(path, 'w').write(text)
PY
    for target in "$ROOT_DIR/scripts/hermes_icq_once.sh" "$ROOT_DIR/scripts/hermes_icq_loop.sh"; do
      python3 - "$src" "$target" <<'PY'
import sys, re
channel, path = sys.argv[1:3]
text = open(path).read()
text = re.sub(r"(SRC_CHANNEL=\${SRC_CHANNEL:-)channel-\d+(})", r"\1"+channel+r"\2", text)
open(path, 'w').write(text)
PY
    done
  fi
  if [[ -n "$dst" ]]; then
    for target in "$ROOT_DIR/scripts/hermes_icq_once.sh" "$ROOT_DIR/scripts/hermes_icq_loop.sh"; do
      python3 - "$dst" "$target" <<'PY'
import sys, re
channel, path = sys.argv[1:3]
text = open(path).read()
text = re.sub(r"(DST_CHANNEL=\${DST_CHANNEL:-)channel-\d+(})", r"\1"+channel+r"\2", text)
open(path, 'w').write(text)
PY
    done
  fi
}

printf '[+] Ensuring Bitora relayer account has funds\n'
CURRENT_BAL=$(./bitorad q bank balances "$BITORA_ADDR" --home "$BITORA_HOME" --node "$NODE" -o json 2>/dev/null | jq -r '.balances[]? | select(.denom=="ubto") | .amount' | head -n1)
CURRENT_BAL=${CURRENT_BAL:-0}

python3 - "$CURRENT_BAL" "$FUND_MIN_BAL" "$FUND_AMOUNT" "$FUND_SOURCE" "$BITORA_ADDR" "$BITORA_HOME" "$NODE" "$CHAIN_ID" "$FUND_FEE" <<'PY'
import sys, subprocess
bal = int(sys.argv[1]) if sys.argv[1] else 0
min_bal = int(sys.argv[2])
amount = sys.argv[3]
from_key = sys.argv[4]
to_addr = sys.argv[5]
home = sys.argv[6]
node = sys.argv[7]
chain_id = sys.argv[8]
fees = sys.argv[9]
if bal >= min_bal:
    print(f"    Balance already sufficient: {bal} ubto")
    sys.exit(0)
print(f"    Funding {to_addr} from {from_key} with {amount} (fees {fees})")
cmd = ["./bitorad", "tx", "bank", "send", from_key, to_addr, amount,
       "--home", home, "--keyring-backend", "test",
       "--chain-id", chain_id, "--node", node, "--fees", fees,
       "-y", "--output", "json"]
result = subprocess.run(cmd, check=True, capture_output=True, text=True)
print("    Tx result:", result.stdout.strip())
PY

sleep ${FUND_WAIT_SECONDS:-2}

CURRENT_BAL=$(./bitorad q bank balances "$BITORA_ADDR" --home "$BITORA_HOME" --node "$NODE" -o json 2>/dev/null | jq -r '.balances[]? | select(.denom=="ubto") | .amount' | head -n1)
printf '    Current relayer balance: %s ubto\n' "${CURRENT_BAL:-0}"

CHANNEL_OUTPUT=$(run_hermes query channels --chain "$BITORA_CHAIN")
readarray -t EXISTING_IDS < <(python3 - "$CHANNEL_OUTPUT" <<'PY'
import sys, re
text = sys.argv[1]
print('\n'.join(re.findall(r'channel_id:\s*ChannelId\("([^"\s]+)"\)', text)))
PY
)
BITORA_CHANNEL=${EXISTING_IDS[0]:-}
OSMO_CHANNEL=${EXISTING_IDS[1]:-}

if [[ -z "$BITORA_CHANNEL" ]]; then
  echo "[+] No existing ICQ channel found. Creating clients, connection, and channel via Hermes."
  CONNECTION_ID=
  CONNECTION_LIST=$(run_hermes query connections --chain "$BITORA_CHAIN")
  readarray -t CONNECTION_IDS < <(python3 - "$CONNECTION_LIST" <<'PY'
import sys, re
text = sys.argv[1]
print('\n'.join(re.findall(r'ConnectionId\("([^"\s]+)"\)', text)))
PY
  )
  for cid in "${CONNECTION_IDS[@]}"; do
    if [[ $cid != connection-localhost ]]; then
      CONNECTION_ID=$cid
      break
    fi
  done

  if [[ -z "$CONNECTION_ID" ]]; then
    BITORA_CLIENT_OUT=$(run_hermes create client --host-chain "$BITORA_CHAIN" --reference-chain "$OSMO_CHAIN")
    BITORA_CLIENT=$(python3 - "$BITORA_CLIENT_OUT" <<'PY'
import sys, re
text = sys.argv[1]
m = re.search(r'client_id:\s*ClientId\("([^"\s]+)"\)', text)
print(m.group(1) if m else "")
PY
    )
    OSMO_CLIENT_OUT=$(run_hermes create client --host-chain "$OSMO_CHAIN" --reference-chain "$BITORA_CHAIN")
    OSMO_CLIENT=$(python3 - "$OSMO_CLIENT_OUT" <<'PY'
import sys, re
text = sys.argv[1]
m = re.search(r'client_id:\s*ClientId\("([^"\s]+)"\)', text)
print(m.group(1) if m else "")
PY
    )
    printf '    Created clients: %s (bitora), %s (osmo)\n' "${BITORA_CLIENT:-?}" "${OSMO_CLIENT:-?}"

    CONNECTION_OUTPUT=$(run_hermes create connection --a-chain "$BITORA_CHAIN" --b-chain "$OSMO_CHAIN")
    CONNECTION_ID=$(python3 - "$CONNECTION_OUTPUT" <<'PY'
import sys, re
text = sys.argv[1]
m = re.search(r'connection_id:\s*ConnectionId\("([^"\s]+)"\)', text)
print(m.group(1) if m else "")
PY
    )
    printf '    Connection established: %s\n' "${CONNECTION_ID:-unknown}"
  else
    printf '    Reusing existing connection: %s\n' "$CONNECTION_ID"
  fi

  if [[ -z "$CONNECTION_ID" ]]; then
    echo "[ERROR] Unable to determine a connection ID for channel creation" >&2
    exit 1
  fi

  CHANNEL_JSON=$(run_hermes create channel --a-chain "$BITORA_CHAIN" --a-connection "$CONNECTION_ID" --a-port icqcontroller --b-port icqhost --order unordered --channel-version icq-1)
  readarray -t CHANNEL_IDS < <(python3 - "$CHANNEL_JSON" <<'PY'
import sys, re
text = sys.argv[1]
print('\n'.join(re.findall(r'channel_id:\s*ChannelId\("([^"\s]+)"\)', text)))
PY
  )
  BITORA_CHANNEL=${CHANNEL_IDS[0]:-}
  OSMO_CHANNEL=${CHANNEL_IDS[1]:-}
  printf '    Channels: bitora/%s <-> osmo/%s\n' "$BITORA_CHANNEL" "${OSMO_CHANNEL:-?}"
else
  echo "[=] Existing channel detected: $BITORA_CHANNEL (skipping handshake)"
  CH_END=$(run_hermes query channel end --chain "$BITORA_CHAIN" --port icqcontroller --channel "$BITORA_CHANNEL")
  readarray -t CHANNEL_IDS < <(python3 - "$CH_END" <<'PY'
import sys, re
text = sys.argv[1]
print('\n'.join(re.findall(r'channel_id:\s*ChannelId\("([^"\s]+)"\)', text)))
PY
  )
  # Query returns [local, counterparty] order; prefer counterparty if present.
  if [[ ${#CHANNEL_IDS[@]} -ge 2 ]]; then
    OSMO_CHANNEL=${CHANNEL_IDS[1]}
  fi
fi

update_channel_refs "$BITORA_CHANNEL" "$OSMO_CHANNEL"

printf '[+] Triggering immediate ICQ packet relay sweep\n'
"$HERMES_ONCE_SCRIPT"

printf '[+] Running Go pricefeed tests (%s)\n' "$GO_TEST_PKGS"
GOFLAGS=${GOFLAGS:-""} go test $GO_TEST_PKGS

printf '[✓] Hermes relayer environment prepared. You can now run: %s --config %s start\n' "$HERMES_BIN" "$HERMES_CONFIG"
