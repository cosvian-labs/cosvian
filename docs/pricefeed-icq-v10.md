# Cosvian Pricefeed ICQ Test (ibc-go v10 + Hermes)

Tujuan singkat:

1. Jalankan chain (tag icq_async)
2. Siapkan keys & fund relayer
3. Buat connection & channel ICQ
4. Set params modul osmosisicq via governance (script)
5. Jalankan Hermes, pastikan harga non‑fallback
6. Uji fee event pakai harga oracle

---

## 0. Export Variabel

```bash
export COSVIAN_HOME=~/.cosvian
export COSVIAN_CHAIN=cosvian
export OSMO_CHAIN=osmo-test-5

export COSVIAN_RELAYER_KEY=cosvian_relayer
export OSMO_RELAYER_KEY=osmo-test-relayer

export FUND_SOURCE=public_sale
export RELAYER_FUND_AMOUNT=2000000000ucsv
export RELAYER_FEE=1000000ucsv
```

Cek binary:

```bash
which cosviand
cosviand version
```

## 1. Start Chain

```bash
cd /mnt/c/Project/Cosvian/cosvian-blockchain
ignite chain serve --reset-once -v --build.tags "icq_async"
```

Tunggu blok jalan:

```bash
watch -n2 'cosviand status 2>/dev/null | jq -r ".SyncInfo.latest_block_height"'
```

## 2. Keys Hermes

```bash
hermes keys list --chain $COSVIAN_CHAIN || true
hermes keys list --chain $OSMO_CHAIN || true
```

Tambahkan jika belum:

```bash
# Cosvian
read -rsp "Mnemonic relayer cosvian: " M1; echo
printf '%s\n' "$M1" > /tmp/b_relayer.mn
hermes keys add --chain $COSVIAN_CHAIN --key-name $COSVIAN_RELAYER_KEY --mnemonic-file /tmp/b_relayer.mn --overwrite
rm -f /tmp/b_relayer.mn

# Osmosis
read -rsp "Mnemonic relayer osmo: " M2; echo
printf '%s\n' "$M2" > /tmp/o_relayer.mn
hermes keys add --chain $OSMO_CHAIN --key-name $OSMO_RELAYER_KEY --mnemonic-file /tmp/o_relayer.mn --overwrite
rm -f /tmp/o_relayer.mn
```

## 3. Fund Relayer Cosvian

```bash
RELAYER_ADDR=$(hermes keys list --chain $COSVIAN_CHAIN | awk -v K=$COSVIAN_RELAYER_KEY '$2==K {gsub(/[()]/,"",$3); print $3}')
echo $RELAYER_ADDR
cosviand tx bank send $FUND_SOURCE $RELAYER_ADDR $RELAYER_FUND_AMOUNT \
  --chain-id $COSVIAN_CHAIN --keyring-backend test --fees $RELAYER_FEE -y
cosviand q bank balances $RELAYER_ADDR
```

## 4. Connection

```bash
hermes query connections --chain $COSVIAN_CHAIN | grep connection-0 \
  || hermes create connection --a-chain $COSVIAN_CHAIN --b-chain $OSMO_CHAIN
hermes query connections --chain $COSVIAN_CHAIN | grep connection-0
```

## 5. Channel ICQ (UNORDERED)

```bash
hermes query channels --chain $COSVIAN_CHAIN | grep icqcontroller || hermes create channel \
  --a-chain $COSVIAN_CHAIN \
  --a-connection connection-0 \
  --a-port icqcontroller \
  --b-port icqhost \
  --order unordered \
  --channel-version icq-1
hermes query channels --chain $COSVIAN_CHAIN | grep icqcontroller
```

## 6. Governance (Set Params osmosisicq)

Gunakan helper script (legacy msg patch sudah ditambahkan). Dua opsi:

### Opsi A: Skrip otomatis submit + vote (robust polling)

```bash
JSON=/tmp/osmo_params.json \
FUND_SOURCE=$FUND_SOURCE \
COSVIAN_CHAIN=$COSVIAN_CHAIN \
RELAYER_FEE=$RELAYER_FEE \
./scripts/osmosisicq_make_proposal.sh \
  --connection connection-0 \
  --pool 666 \
  --base uosmo \
  --quote ibc/... \
  --interval 30 \
  --min-liq 0 \
  --max-dev 0.8 \
  --twap-window 300 \
  --deposit 10000000ucsv \
  --submit --vote

```

Script ini:

- Submit (sync)
- Poll TX Deliver
- Deposit (top-up)
- Vote YES
- Exit setelah PASSED (output final: Proposal ID)

### Opsi B: Buat & submit proposal kustom (multi parameter)

```bash
./scripts/osmosisicq_make_proposal.sh \
  --connection connection-0 \
  --pool 666 \
  --base uosmo \
  --quote ibc/C5B7196709BDFC3A312B06D7292892FA53F379CD3D556B65DB00E1531D471BBA \
  --interval 30 \
  --min-liq 0 \
  --max-dev 0.8 \
  --twap-window 300 \
  --submit --vote
```

Cek status kalau perlu:

```bash
cosviand q gov proposals -o json | jq '.proposals | length'
```

Setelah PASSED verifikasi:

```bash
cosviand q osmosisicq params -o json | jq .
```

Harus tampil nilai:

- connection_id: connection-0
- pool_id: 666
- update_interval_seconds: 30
- base_denom / quote_denom sesuai
- use_twap false/true sesuai (sesuai script)
- max_deviation 0.8

## 7. Start Hermes Relayer

Terminal terpisah:

```bash
hermes start --chain $COSVIAN_CHAIN --chain $OSMO_CHAIN
```

## 8. Pantau Harga ICQ

```bash
tail -f ignite_run.log | grep -E 'osmosisicq|icq_|price'
```

Pastikan harga menjadi non‑fallback setelah paket ACK.

## 9. Uji Fee Pakai Oracle

```bash
RELAYER_ADDR=$(hermes keys list --chain $COSVIAN_CHAIN | awk -v K=$COSVIAN_RELAYER_KEY '$2==K {gsub(/[()]/,"",$3); print $3}')
cosviand tx bank send $FUND_SOURCE $RELAYER_ADDR 1ucsv \
  --chain-id $COSVIAN_CHAIN --fees 50000ucsv -y -o json --broadcast-mode=sync \
  | jq '.events[] | select(.type|test("oracle|fee|price|fee_charged"))'
```

Pastikan event tidak menunjukkan fallback (misal is_fallback=false apabila ada field serupa).

## 10. Troubleshooting Cepat

| Gejala                                          | Aksi                                                    |
| ----------------------------------------------- | ------------------------------------------------------- |
| Proposal gagal code=10 “message not recognized” | Pastikan rebuild dengan legacy msg patch & restart node |
| Tidak muncul subcommand osmosisicq              | Gunakan binary dengan tag icq_async, periksa PATH       |
| Harga tetap fallback                            | Pastikan channel OPEN, Hermes aktif, params benar       |
| TX fee event tidak muncul                       | Naikkan fees (RELAYER_FEE), cek log hybrid fee          |

Log tx detail:

```bash
cosviand q tx <TXHASH> -o json | jq '{code,raw_log,events:([.events[].type]|unique)}'
```

## 11. Reset (Jika Perlu)

```bash
pkill cosviand || true
rm -rf $COSVIAN_HOME
ignite chain serve --reset-once --build.tags "icq_async"
```

Selesai.
