#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
BINARY="$ROOT_DIR/build/cosviand"
CHAIN_ID="${CHAIN_ID:-cosvian-1}"
HOME_DIR="${HOME_DIR:-$HOME/.cosvian-stub}"
MONIKER="${MONIKER:-infra-validator}"
KEYRING_BACKEND="${KEYRING_BACKEND:-test}"
RELAYER_KEY_NAME="${RELAYER_KEY_NAME:-relayer}"
RELAYER_FUNDS="${RELAYER_FUNDS:-200000000ucsv}"
RELAYER_FEE="${RELAYER_FEE:-1000000ucsv}"
RELAYER_FUNDING_SOURCE="${RELAYER_FUNDING_SOURCE:-public_sale}"
HERMES_CONFIG="${HERMES_CONFIG:-$HOME/.hermes/config.toml}"
HERMES_CHAIN_ID="$CHAIN_ID"
HERMES_BINARY="${HERMES_BINARY:-hermes}"
NODE_LOG="${NODE_LOG:-$HOME_DIR/cosviand.log}"
HERMES_LOG="${HERMES_LOG:-$ROOT_DIR/hermes/hermes.log}"
SUCCESS=0
KEYS_DIR="$ROOT_DIR/hermes/keys"

ACCOUNTS=(
  "pool_ecosystem:5000000000000ucsv"
  "conversion_pool:25000000000000ucsv"
  "pos_infra:10000000000000ucsv"
  "exchange_fund:5000000000000ucsv"
  "norix_ecosystem:5000000000000ucsv"
  "infra_validators:6000000000000ucsv"
  "education_ops:4000000000000ucsv"
  "compliance_vault:5000000000000ucsv"
  "emergency_reserve:5000000000000ucsv"
  "strategic_growth:5000000000000ucsv"
  "team:8000000000000ucsv"
  "dao_reserve:4000000000000ucsv"
  "public_sale:18000000000000ucsv"
  "tester:100000000ucsv"
)

FEE_SPLIT_KEYS=(
  "treasury_wallet:bto1team00000000000000000000000000000000000"
  "retail_wallet:bto1retail000000000000000000000000000000000"
  "token_dev_wallet:bto1dev000000000000000000000000000000000000"
  "token_creator_wallet:bto1creator00000000000000000000000000000000"
)

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "\n[ERROR] Required command '$1' not found in PATH" >&2
    exit 1
  fi
}

require_cmd jq
require_cmd python3
require_cmd curl
require_cmd "$HERMES_BINARY"

cleanup() {
  if [[ $SUCCESS -ne 1 ]]; then
    [[ -n "${HERMES_PID:-}" ]] && kill "$HERMES_PID" >/dev/null 2>&1 || true
    [[ -n "${NODE_PID:-}" ]] && kill "$NODE_PID" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

if [[ ! -x "$BINARY" ]]; then
  echo "\n>>> Building cosviand binary"
  (cd "$ROOT_DIR" && go build -o "$BINARY" ./cmd/cosviand)
fi

echo "\n>>> Stopping existing processes"
pkill -f "$BINARY start" >/dev/null 2>&1 || true
pkill -f "$HERMES_BINARY start" >/dev/null 2>&1 || true
sleep 1

initialize_genesis() {
  rm -rf "$HOME_DIR"
  "$BINARY" init "$MONIKER" --chain-id "$CHAIN_ID" --home "$HOME_DIR"
  mkdir -p "$KEYS_DIR"

  for entry in "${ACCOUNTS[@]}"; do
    local name=${entry%%:*}
    local amount=${entry##*:}
    local key_json=""
    if ! "$BINARY" keys show "$name" --home "$HOME_DIR" --keyring-backend "$KEYRING_BACKEND" >/dev/null 2>&1; then
      key_json=$("$BINARY" keys add "$name" --home "$HOME_DIR" --keyring-backend "$KEYRING_BACKEND" --output json)
      printf '%s' "$key_json" > "$KEYS_DIR/${name}.json"
      local mnemonic
      mnemonic=$(echo "$key_json" | jq -r '.mnemonic // empty')
      if [[ -n "$mnemonic" ]]; then
        printf '%s' "$mnemonic" > "$KEYS_DIR/${name}.mnemonic"
      fi
    fi
    local address
    address=$("$BINARY" keys show "$name" --home "$HOME_DIR" --keyring-backend "$KEYRING_BACKEND" --address)
    "$BINARY" genesis add-genesis-account "$address" "$amount" --home "$HOME_DIR"
  done

  local validator_amount=6000000000000ucsv
  "$BINARY" genesis gentx infra_validators "$validator_amount" \
    --chain-id "$CHAIN_ID" \
    --moniker "$MONIKER" \
    --home "$HOME_DIR" \
    --keyring-backend "$KEYRING_BACKEND"
  "$BINARY" genesis collect-gentxs --home "$HOME_DIR"

  local GEN_FILE="$HOME_DIR/config/genesis.json"
  local tmp
  tmp=$(mktemp)

  jq \
    --arg denom "ucsv" \
    '.app_state.staking.params.bond_denom = $denom |
     .app_state.mint.minter.inflation = "0.000000000000000000" |
     .app_state.mint.params.inflation_rate_change = "0.000000000000000000" |
     .app_state.mint.params.inflation_max = "0.000000000000000000" |
     .app_state.mint.params.inflation_min = "0.000000000000000000" |
     .app_state.mint.params.mint_denom = $denom |
     .app_state.distribution.params.base_proposer_reward = "0.000000000000000000" |
     .app_state.distribution.params.bonus_proposer_reward = "0.000000000000000000" |
     .app_state.distribution.params.community_tax = "0.000000000000000000" |
     .app_state.bank.denom_metadata = [
       {
         "base": "ucsv",
         "display": "CSV",
         "description": "Cosvian native token",
         "denom_units": [
           {"denom": "ucsv", "exponent": 0},
           {"denom": "CSV", "exponent": 6}
         ],
         "name": "Cosvian Token",
         "symbol": "CSV"
       }
     ] |
     .app_state.wasm.params.code_upload_access.permission = "Everybody" |
     .app_state.wasm.params.instantiate_default_permission = "Everybody" |
     .app_state.fees.params.fee_mode = "hybrid" |
     .app_state.fees.params.fee_table_usd.deploy.usd_amount = "0.000000000000000000" |
     .app_state.fees.params.fee_table_usd.wizard.usd_amount = "0.000000000000000000" |
     .app_state.fees.params.fee_table_usd.pos_payment.usd_amount = "0.150000000000000000" |
     .app_state.fees.params.fee_table_usd.token_interaction.usd_amount = "3.000000000000000000" |
     .app_state.fees.params.fee_table_usd.native_transfer.usd_amount = "1.000000000000000000" |
     .app_state.fees.params.fee_table_usd.dex_native.usd_amount = "1.000000000000000000" |
     .app_state.fees.params.fee_table_usd.dex_user.usd_amount = "3.000000000000000000" |
     .app_state.fees.params.oracle_params.band_request_id = "1" |
     .app_state.fees.params.oracle_params.twap_window = "300s" |
     .app_state.fees.params.oracle_params.deviation_limit = "0.050000000000000000" |
     .app_state.fees.params.oracle_params.fallback_ttl = "1800s" |
     .app_state.fees.params.oracle_params.max_price_age = "600s" |
     .app_state.fees.params.guard_rails.min_gas_price_bto = "0.000001000000000000" |
     .app_state.fees.params.guard_rails.max_gas_price_bto = "1.000000000000000000" |
     .app_state.fees.params.guard_rails.max_gas_wizard = "100000" |
     .app_state.fees.params.guard_rails.max_gas_deploy = "500000" |
     .app_state.fees.params.min_gas_policy.enabled = true |
     .app_state.fees.params.min_gas_policy.global_min_gas_price = "0.000001000000000000" |
     .app_state.fees.params.min_gas_policy.allow_per_tx_override = true |
     .app_state.osmosisicq.params = {
       "connection_id": "connection-0",
       "update_interval_seconds": "30",
       "pool_id": "1464",
       "base_denom": "uosmo",
       "quote_denom": "ibc/PLACEHOLDER_HASH",
       "use_twap": true,
       "twap_window_seconds": "300",
       "min_liquidity": "0",
       "max_deviation": "0.5"
     }
    ' "$GEN_FILE" > "$tmp"

  mv "$tmp" "$GEN_FILE"

  for kv in "${FEE_SPLIT_KEYS[@]}"; do
    local key=${kv%%:*}
    local value=${kv##*:}
    jq ".app_state.fees.params.$key = \"$value\"" "$GEN_FILE" > "$tmp"
    mv "$tmp" "$GEN_FILE"
  done

  "$BINARY" genesis validate --home "$HOME_DIR"
}

echo "\n>>> Preparing fresh node state (CHAIN_ID=$CHAIN_ID)"
initialize_genesis

NODE_ARGS=(
  start
  --home "$HOME_DIR"
  --chain-id "$CHAIN_ID"
  --minimum-gas-prices 0.1ucsv
  --grpc.address 127.0.0.1:9090
  --grpc.enable true
  --api.enable false
)

echo "\n>>> Launching node (logs: $NODE_LOG)"
rm -f "$NODE_LOG"
"$BINARY" "${NODE_ARGS[@]}" >"$NODE_LOG" 2>&1 &
NODE_PID=$!
trap 'kill $NODE_PID >/dev/null 2>&1 || true' EXIT

wait_for_rpc() {
  local rpc="${1:-http://127.0.0.1:26657/status}"
  echo "Waiting for node RPC ($rpc)"
  until curl -sf "$rpc" >/dev/null; do
    sleep 1
  done
}

wait_for_height() {
  local target_height=${1:-2}
  while true; do
    local height
    height=$(curl -sf http://127.0.0.1:26657/status | jq -r '.result.sync_info.latest_block_height // "0"')
    if [[ "${height:-0}" -ge "$target_height" ]]; then
      break
    fi
    sleep 1
  done
}

wait_for_rpc
wait_for_height 3
echo "Node is ready"

KEY_EXISTS=$("$BINARY" keys show "$RELAYER_KEY_NAME" --home "$HOME_DIR" --keyring-backend "$KEYRING_BACKEND" >/dev/null 2>&1 && echo yes || echo no)
if [[ "$KEY_EXISTS" == yes ]]; then
  yes | "$BINARY" keys delete "$RELAYER_KEY_NAME" --home "$HOME_DIR" --keyring-backend "$KEYRING_BACKEND" >/dev/null
fi

RELAYER_JSON=$("$BINARY" keys add "$RELAYER_KEY_NAME" --home "$HOME_DIR" --keyring-backend "$KEYRING_BACKEND" --output json)
RELAYER_MNEMONIC=$(echo "$RELAYER_JSON" | jq -r '.mnemonic')
RELAYER_ADDRESS=$(echo "$RELAYER_JSON" | jq -r '.address')
RELAYER_MNEMONIC_FILE="$ROOT_DIR/hermes/${RELAYER_KEY_NAME}_mnemonic.txt"
mkdir -p "$ROOT_DIR/hermes"
printf '%s' "$RELAYER_MNEMONIC" > "$RELAYER_MNEMONIC_FILE"
printf '\nRelayer address: %s\n' "$RELAYER_ADDRESS"

echo "\n>>> Funding relayer account"
"$BINARY" tx bank send public_sale "$RELAYER_ADDRESS" "$RELAYER_FUNDS" \
  --home "$HOME_DIR" \
  --keyring-backend "$KEYRING_BACKEND" \
  --chain-id "$CHAIN_ID" \
  --gas auto \
  --gas-adjustment 1.4 \
  --fees "$RELAYER_FEE" \
  --broadcast-mode sync \
  --yes >/dev/null

sleep 2

python3 - <<PY
from pathlib import Path
config_path = Path("$HERMES_CONFIG").expanduser()
text = config_path.read_text()
text = text.replace("id = 'cosvian'", "id = '$CHAIN_ID'")
text = text.replace("id = 'stubchain'", "id = '$CHAIN_ID'")
text = text.replace("key_name = 'pool_ecosystem'", "key_name = '$RELAYER_KEY_NAME'")
config_path.write_text(text)
PY

echo "\n>>> Syncing Hermes keyring"
$HERMES_BINARY keys delete --chain "$CHAIN_ID" --all >/dev/null 2>&1 || true
$HERMES_BINARY keys add --chain "$CHAIN_ID" --mnemonic-file "$RELAYER_MNEMONIC_FILE" --overwrite

wait_for_height 5

echo "\n>>> Creating IBC connection"
CONNECTION_OUTPUT=$($HERMES_BINARY create connection --a-chain "$CHAIN_ID" --b-chain osmo-test-5)
echo "$CONNECTION_OUTPUT"
A_CONNECTION_ID=$(echo "$CONNECTION_OUTPUT" | grep -m1 "a_side" -A4 | grep 'connection_id' | head -n1 | awk '{print $2}' | tr -d '"')
A_CONNECTION_ID=${A_CONNECTION_ID:-connection-0}

echo "\n>>> Creating ICQ channel (unordered)"
CHANNEL_OUTPUT=$($HERMES_BINARY create channel \
  --a-chain "$CHAIN_ID" \
  --a-connection "$A_CONNECTION_ID" \
  --a-port icqcontroller \
  --b-port icqhost \
  --order unordered \
  --channel-version icq-1)
echo "$CHANNEL_OUTPUT"
CHANNEL_ID=$(echo "$CHANNEL_OUTPUT" | grep -m1 "a_side" -A6 | grep 'channel_id' | head -n1 | awk '{print $2}' | tr -d '"')

mkdir -p "$(dirname "$HERMES_LOG")"
rm -f "$HERMES_LOG"
nohup $HERMES_BINARY start > "$HERMES_LOG" 2>&1 &
HERMES_PID=$!
echo "$HERMES_PID" > "$ROOT_DIR/hermes/hermes.pid"

SUCCESS=1
cat <<SUMMARY

============================================================
Setup complete
------------------------------------------------------------
Chain ID      : $CHAIN_ID
Node home     : $HOME_DIR
Node log      : $NODE_LOG (PID $NODE_PID)
Relayer addr  : $RELAYER_ADDRESS
Hermes config : $HERMES_CONFIG
Hermes log    : $HERMES_LOG (PID $HERMES_PID)
Connection    : $A_CONNECTION_ID
Channel       : ${CHANNEL_ID:-unknown}
============================================================
SUMMARY

trap - EXIT
exit 0
