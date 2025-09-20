# Bitora Pricefeed ICQ Test (Hermes + ibc-go v10)

Tujuan:

1. Nyalakan chain via Ignite (tag `icq_async`)
2. Pastikan key relayer Hermes (bitora & osmo testnet) ada
3. Fund relayer Bitora
4. Buat connection & channel ICQ (UNORDERED)
5. Set params modul `osmosisicq` via governance
6. Relay paket ICQ hingga harga masuk (non‑fallback)
7. Verifikasi fee memakai harga oracle

## 0. Variabel Lingkungan (opsional)

```bash
export BITORA_HOME=~/.bitora
export BITORA_CHAIN=bitora
export OSMO_CHAIN=osmo-test-5
export BITORA_RELAYER_KEY=bitora_relayer
export OSMO_RELAYER_KEY=osmo-test-relayer
export FUND_SOURCE=public_sale
export RELAYER_FUND_AMOUNT=2000000000ubto
# Gunakan fee tinggi agar lolos guard rails (hybrid fee validation)
export RELAYER_FEE=1000000ubto
```

## 1. Start Chain (Ignite)

```bash
cd /home/munra/project/bitora/bitora-blockchain
ignite chain serve --reset-once -v --build.tags "icq_async"
```

Simpan semua mnemonic yang muncul (copy ke file aman).

## 2. Cek Chain ID & Akun Funding

```bash
bitorad status 2>/dev/null | jq -r '.NodeInfo.network'
bitorad keys list --home $BITORA_HOME --keyring-backend test | jq '.[].name'
bitorad keys show $FUND_SOURCE -a --home $BITORA_HOME --keyring-backend test
```

## 3. Cek Hermes Keys

```bash
hermes keys list --chain $BITORA_CHAIN || true
hermes keys list --chain $OSMO_CHAIN || true
```

## 4. Tambah Key Bitora Relayer (jika belum ada)

Kalau output sebelumnya belum ada `- $BITORA_RELAYER_KEY (`:

```bash
# Recover dari mnemonic yang kamu pilih untuk relayer bitora:
read -p "Paste mnemonic relayer bitora: " MNEMONIC
printf "%s" "$MNEMONIC" > /tmp/bitora_relayer.mn
hermes keys add --chain $BITORA_CHAIN --key-name $BITORA_RELAYER_KEY --mnemonic-file /tmp/bitora_relayer.mn --overwrite
shred -u /tmp/bitora_relayer.mn
hermes keys list --chain $BITORA_CHAIN | grep $BITORA_RELAYER_KEY
```

## 5. Tambah Key Osmosis Relayer (jika belum)

```bash
read -p "Paste mnemonic osmosis relayer: " OM
printf "%s" "$OM" > /tmp/osmo_relayer.mn
hermes keys add --chain $OSMO_CHAIN --key-name $OSMO_RELAYER_KEY --mnemonic-file /tmp/osmo_relayer.mn --overwrite
shred -u /tmp/osmo_relayer.mn
hermes keys list --chain $OSMO_CHAIN | grep $OSMO_RELAYER_KEY
```

## 6. Fund Relayer Bitora

```bash
RELAYER_ADDR=$(hermes keys list --chain $BITORA_CHAIN | awk -v K=$BITORA_RELAYER_KEY '$2==K {gsub(/[()]/,"",$3); print $3}')
echo "Relayer address: $RELAYER_ADDR"
bitorad q bank balances $RELAYER_ADDR --home $BITORA_HOME --keyring-backend test
bitorad tx bank send $FUND_SOURCE $RELAYER_ADDR $RELAYER_FUND_AMOUNT \
  --home $BITORA_HOME --keyring-backend test --chain-id $BITORA_CHAIN --fees $RELAYER_FEE -y
bitorad q bank balances $RELAYER_ADDR
```

## 7. Buat Connection (connection-0)

Cek dulu:

```bash
hermes query connections --chain $BITORA_CHAIN | grep connection-0 || hermes create connection --a-chain $BITORA_CHAIN --b-chain $OSMO_CHAIN
hermes query connections --chain $BITORA_CHAIN
```

## 8. Buat Channel ICQ (UNORDERED)

```bash
hermes query channels --chain $BITORA_CHAIN | grep icqcontroller || hermes create channel \
  --a-chain $BITORA_CHAIN \
  --a-connection connection-0 \
  --a-port icqcontroller \
  --b-port icqhost \
  --order unordered \
  --channel-version icq-1
hermes query channels --chain $BITORA_CHAIN | grep icqcontroller
```

verifikasi cek channel

```bash
bitorad q ibc channel connections connection-0 -o json | jq '.channels'
```

Catat:

```bash
# Ambil ID channel (kolom sebelum ':', bukan port)
BITORA_CH=$(hermes query channels --chain $BITORA_CHAIN | awk '/icqcontroller/ {sub(/:.*/,"",$1); print $1; exit}')
OSMO_CH=$(hermes query channels --chain $OSMO_CHAIN | awk '/icqhost/ {sub(/:.*/,"",$1); print $1; exit}')
echo "Bitora channel: $BITORA_CH  Osmosis channel: $OSMO_CH"
if [ -z "$BITORA_CH" ] || [ -z "$OSMO_CH" ]; then
  echo "[WARN] Channel IDs belum terdeteksi. Pastikan create channel sukses." >&2
fi
```

## 9. Governance: Set Params osmosisicq

Ambil authority gov:

```bash
GOV=$(bitorad q auth module-accounts -o json --home $BITORA_HOME | jq -r '.accounts[] | select(.value.name=="gov") | .value.address')
echo "$GOV"
if [ -z "$GOV" ] || [ "$GOV" = "null" ]; then
  echo "[ERROR] GOV module account tidak ditemukan via module-accounts (cek struktur JSON)." >&2
  echo "[INFO] Coba fallback genesis (format bisa beda)" >&2
  GOV=$(jq -r '.app_state.auth.accounts[] | select(.name=="gov") | .base_account.address // empty' $BITORA_HOME/config/genesis.json)
  if [ -z "$GOV" ]; then
    GOV=$(jq -r '.app_state.auth.module_accounts[]? | select(.name=="gov") | .base_account.address // .base_vesting_account.base_account.address // empty' $BITORA_HOME/config/genesis.json)
  fi
  echo "Genesis GOV address: $GOV" >&2
  if [ -z "$GOV" ] || [ "$GOV" = "null" ]; then
    echo "[FATAL] Tidak menemukan address gov. Pastikan govtypes.ModuleName ada di moduleAccPerms & regen chain." >&2
  fi
fi
```

Pastikan node sudah produce blok (hindari error "bitora is not ready"):

```bash
while true; do HEIGHT=$(bitorad status 2>/dev/null | jq -r '.SyncInfo.latest_block_height'); [ "$HEIGHT" != "0" -a "$HEIGHT" != "null" ] && echo "Height=$HEIGHT" && break; sleep 2; done
```

Buat proposal:

```bash
cat > /tmp/osmo_params.json <<'EOF'
{
  "messages": [
    {
      "@type": "/bitora.osmosisicq.v1.MsgUpdateParams",
      "authority": "__GOV_PLACEHOLDER__",
      "params": {
        "connection_id": "connection-0",
        "update_interval_seconds": "30",
        "pool_id": "666",
        "base_denom": "uosmo",
        "quote_denom": "ibc/C5B7196709BDFC3A312B06D7292892FA53F379CD3D556B65DB00E1531D471BBA",
        "use_twap": false,
        "twap_window_seconds": "300",
        "min_liquidity": "0",
        "max_deviation": "0.8"
      }
    }
  ],
  "deposit": "1000000ubto",
  "title": "ICQ Params",
  "summary": "Set pricefeed params"
}
EOF

sed -i "s/__GOV_PLACEHOLDER__/$GOV/" /tmp/osmo_params.json
jq '.' /tmp/osmo_params.json
```

Sanity test (generate-only sebelum broadcast penuh):

Catatan: Sejak gov v1, proposal JSON dibungkus otomatis menjadi `MsgSubmitProposal` (`/cosmos.gov.v1.MsgSubmitProposal`) yang punya array `messages` internal. Jadi field pertama yang nampak adalah wrapper gov, dan pesan kustom kita ada di `messages[0]` di dalamnya.

```bash
# Tampilkan tipe wrapper (harus /cosmos.gov.v1.MsgSubmitProposal)
bitorad tx gov submit-proposal /tmp/osmo_params.json \
  --from $FUND_SOURCE --chain-id $BITORA_CHAIN --fees $RELAYER_FEE \
  --keyring-backend test --generate-only -o json | jq -r '.body.messages[0]."@type"'

# Validasi tipe pesan kustom di dalam wrapper
bitorad tx gov submit-proposal /tmp/osmo_params.json \
  --from $FUND_SOURCE --chain-id $BITORA_CHAIN --fees $RELAYER_FEE \
  --keyring-backend test --generate-only -o json | jq -r '.body.messages[0].messages[0]."@type"'
```

Harus output kedua: `/bitora.osmosisicq.v1.MsgUpdateParams`.

Submit + deposit + vote:

```bash
# Gunakan fee besar agar tidak ditolak guard rails fee hybrid
PROPID=$(bitorad tx gov submit-proposal /tmp/osmo_params.json \
  --from $FUND_SOURCE --chain-id $BITORA_CHAIN --fees $RELAYER_FEE \
  --keyring-backend test -y -o json | jq -r '.logs[0].events[] | select(.type=="submit_proposal").attributes[] | select(.key=="proposal_id").value')
echo "Proposal ID: $PROPID"

# Fallback jika parsing gagal (ambil id terbaru)
if [ -z "$PROPID" ] || [ "$PROPID" = "null" ]; then
  PROPID=$(bitorad q gov proposals -o json | jq -r '.proposals| last | .proposal_id')
  echo "(fallback) Proposal ID: $PROPID"
fi

bitorad tx gov deposit $PROPID 1000000ubto --from $FUND_SOURCE --chain-id $BITORA_CHAIN --fees $RELAYER_FEE --keyring-backend test -y
bitorad tx gov vote $PROPID yes --from $FUND_SOURCE --chain-id $BITORA_CHAIN --fees $RELAYER_FEE --keyring-backend test -y
watch -n 4 "bitorad q gov proposal $PROPID -o json | jq -r '.status'"
```

Setelah PASSED:

```bash
bitorad q osmosisicq params -o json | jq .
```

## 10. Start Hermes Relayer (Background)

Terminal baru:

```bash
hermes start --chain $BITORA_CHAIN --chain $OSMO_CHAIN
```

(Atau tanpa filter: `hermes start`)

## 11. Manual Kick (Jika butuh cepat)

Di terminal lain (opsional):

```bash
hermes tx packet-recv --dst-chain $OSMO_CHAIN --src-chain $BITORA_CHAIN --src-port icqcontroller --src-channel $BITORA_CH
hermes tx packet-recv --dst-chain $BITORA_CHAIN --src-chain $OSMO_CHAIN --src-port icqhost --src-channel $OSMO_CH
hermes tx packet-ack  --src-chain $OSMO_CHAIN --dst-chain $BITORA_CHAIN --src-port icqhost --src-channel $OSMO_CH
hermes tx packet-ack  --src-chain $BITORA_CHAIN --dst-chain $OSMO_CHAIN --src-port icqcontroller --src-channel $BITORA_CH
```

## 12. Pantau Event Harga

```bash
tail -f ignite_run.log | grep -E 'icq_|osmosisicq_|price'
```

## 13. Uji Fee Menggunakan Harga (Non-fallback)

Kirim tx kecil:

```bash
RELAYER_ADDR=$(hermes keys list --chain $BITORA_CHAIN | awk -v K=$BITORA_RELAYER_KEY '$2==K {gsub(/[()]/,"",$3); print $3}')
bitorad tx bank send $FUND_SOURCE $RELAYER_ADDR 1ubto \
  --chain-id $BITORA_CHAIN --fees 50000ubto -y --broadcast-mode=block -o json \
  | jq '.events[] | select(.type|test("oracle|fees|price"))'
```

Periksa event: tidak ada indikator fallback (misal `fallback_used=true`).

## 14. Troubleshooting Cepat

| Gejala                | Solusi                                           |
| --------------------- | ------------------------------------------------ |
| Proposal tidak PASSED | Cek deposit & vote, tunggu voting period         |
| Channel tidak OPEN    | Pastikan UNORDERED & connection-0 OK             |
| Harga tidak muncul    | Relay packet (packet-recv/ack) ulang; cek params |
| Fallback terus        | Tunggu interval berikut & pastikan ack sukses    |

## 15. Reset Ulang (Jika Perlu Mulai Bersih)

```bash
pkill bitorad || true
ignite chain serve --reset-once --build.tags icq_async
```

Selesai. Ikuti urutan tanpa lompat untuk hasil konsisten.
