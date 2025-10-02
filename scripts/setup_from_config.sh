#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
BINARY=${BINARY:-"$ROOT_DIR/build/cosviand"}
CHAIN_ID=${CHAIN_ID:-cosvian-local-1}
HOME_DIR=${HOME_DIR:-"$HOME/.cosvian-stub"}
MONIKER=${MONIKER:-infra-validator}
KEYRING_BACKEND=${KEYRING_BACKEND:-test}

if [[ ! -x "$BINARY" ]]; then
  echo "Binary '$BINARY' not found or not executable. Build it first (e.g. go build -o build/cosviand ./cmd/cosviand)." >&2
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required but not installed." >&2
  exit 1
fi

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
  "treasury_wallet:csv1team00000000000000000000000000000000000"
  "retail_wallet:csv1retail000000000000000000000000000000000"
  "token_dev_wallet:csv1dev000000000000000000000000000000000000"
  "token_creator_wallet:csv1creator00000000000000000000000000000000"
)

# Fresh init
rm -rf "$HOME_DIR"
"$BINARY" init "$MONIKER" --chain-id "$CHAIN_ID" --home "$HOME_DIR"

echo "\n>>> Creating keys & genesis accounts"
for entry in "${ACCOUNTS[@]}"; do
  name=${entry%%:*}
  amount=${entry##*:}
  if ! "$BINARY" keys show "$name" --home "$HOME_DIR" --keyring-backend "$KEYRING_BACKEND" >/dev/null 2>&1; then
    "$BINARY" keys add "$name" --home "$HOME_DIR" --keyring-backend "$KEYRING_BACKEND" --output json >/dev/null
  fi
  "$BINARY" genesis add-genesis-account "$("$BINARY" keys show "$name" --home "$HOME_DIR" --keyring-backend "$KEYRING_BACKEND" --address)" "$amount" --home "$HOME_DIR"
done

echo "\n>>> Creating validator gentx"
VALIDATOR_NAME=infra_validators
VALIDATOR_AMOUNT=6000000000000ucsv
"$BINARY" genesis gentx "$VALIDATOR_NAME" "$VALIDATOR_AMOUNT" \
  --chain-id "$CHAIN_ID" \
  --moniker "$MONIKER" \
  --home "$HOME_DIR" \
  --keyring-backend "$KEYRING_BACKEND"
"$BINARY" genesis collect-gentxs --home "$HOME_DIR"

echo "\n>>> Updating genesis params"
GEN_FILE="$HOME_DIR/config/genesis.json"
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
   .app_state.fees.params.guard_rails.min_gas_price_csv = "0.000001000000000000" |
   .app_state.fees.params.guard_rails.max_gas_price_csv = "1.000000000000000000" |
   .app_state.fees.params.guard_rails.max_gas_wizard = "100000" |
   .app_state.fees.params.guard_rails.max_gas_deploy = "500000" |
   .app_state.fees.params.min_gas_policy.enabled = true |
   .app_state.fees.params.min_gas_policy.global_min_gas_price = "0.000001000000000000" |
   .app_state.fees.params.min_gas_policy.allow_per_tx_override = true
  ' "$GEN_FILE" > "$tmp"

mv "$tmp" "$GEN_FILE"

for kv in "${FEE_SPLIT_KEYS[@]}"; do
  key=${kv%%:*}
  value=${kv##*:}
  jq ".app_state.fees.params.$key = \"$value\"" "$GEN_FILE" > "$tmp"
  mv "$tmp" "$GEN_FILE"
done

"$BINARY" genesis validate --home "$HOME_DIR"

echo "\nSetup complete. Start the node with:\n  $BINARY start --home $HOME_DIR --minimum-gas-prices 0.1ucsv"
