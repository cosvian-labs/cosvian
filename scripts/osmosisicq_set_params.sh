#!/usr/bin/env bash
# Submit a governance proposal to update osmosisicq params.
# Requires a local node with gov enabled and a key that can deposit and vote.

set -euo pipefail

KEY=${KEY:-public_sale}
FROM=${FROM:-$KEY}
CHAIN_ID=${CHAIN_ID:-cosvian-1}
NODE=${NODE:-http://localhost:26657}
FEES=${FEES:-10000ubto}
DEPOSIT=${DEPOSIT:-1000000ubto}
PROPOSAL_FILE=${PROPOSAL_FILE:-proposals/osmosisicq_update_params.json}

# Replace authority placeholder with gov module address dynamically (robust resolution)
# Try: direct module-account query -> list filter -> debug addr (legacy)
GOV_ADDR=$(cosviand q auth module-account gov -o json 2>/dev/null | jq -r '.account.base_account.address // .base_account.address // empty')
if [[ -z "$GOV_ADDR" || "$GOV_ADDR" == "null" ]]; then
  GOV_ADDR=$(cosviand q auth module-accounts -o json 2>/dev/null | jq -r '.accounts[] | select(.name=="gov" or .base_vesting_account.base_account.address!=null) | .base_account.address // .base_vesting_account.base_account.address' | head -n1)
fi
if [[ -z "$GOV_ADDR" || "$GOV_ADDR" == "null" ]]; then
  GOV_ADDR=$(cosviand debug addr "gov" 2>/dev/null || true)
fi
if [[ -z "$GOV_ADDR" || "$GOV_ADDR" == "null" ]]; then
  echo "[warn] Could not auto-resolve gov module address; proposal will keep placeholder."
fi

TMP=$(mktemp)
jq --arg auth "${GOV_ADDR}" '.messages[0].authority = ($auth // .messages[0].authority)' "$PROPOSAL_FILE" > "$TMP"

echo "Submitting osmosisicq params proposal..."
cosviand tx gov submit-proposal "$TMP" \
  --from "$FROM" \
  --chain-id "$CHAIN_ID" \
  --node "$NODE" \
  --fees "$FEES" \
  -y

echo "Depositing..."
cosviand tx gov deposit 1 "$DEPOSIT" \
  --from "$FROM" --chain-id "$CHAIN_ID" --node "$NODE" --fees "$FEES" -y

echo "Voting YES..."
cosviand tx gov vote 1 yes \
  --from "$FROM" --chain-id "$CHAIN_ID" --node "$NODE" --fees "$FEES" -y

echo "Done. Monitor proposal status with:\n  cosviand q gov proposal 1\n"
