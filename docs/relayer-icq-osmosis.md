# ICQ Relayer (Hermes) – Osmosis

Goal: Open an IBC channel for ICS‑31 (async‑icq) between Bitora (controller port `icqcontroller`) and Osmosis (host port `icqhost`), version `icq-1`, and relay packets with Hermes.

Prereqs

- Bitora node running with IBC and `icqcontroller` route (this repo).
- Osmosis chain with async‑icq host enabled and allowing your queries.
- Hermes v1+ installed and funded relayer keys on both chains.

Config

- Use `hermes/config.toml` from this repo as a starting point. Adjust RPC/GRPC endpoints and key names if needed.
- Enable workers for packets if you want automatic ICQ packet relaying:
  - In `config.toml`, set:
    - `[mode.channels].enabled = true`
    - `[mode.packets].enabled = true`
    - Optionally `tx_confirmation = true`

Keys

- Import keys (update file paths and names):

```bash
hermes keys add --chain bitora --mnemonic-file <bitora-mnemonic.txt>
hermes keys add --chain osmo-test-5 --mnemonic-file <osmosis-testnet-mnemonic.txt>
# or mainnet
# hermes keys add --chain osmosis-1 --mnemonic-file <osmosis-mainnet-mnemonic.txt>
```

Clients + Connection

- Create clients and a connection between the chains.

Testnet:

```bash
hermes create connection --a-chain bitora --b-chain osmo-test-5
```

Mainnet:

```bash
hermes create connection --a-chain bitora --b-chain osmosis-1
```

Channel (ICS‑31)

- Open the channel with explicit ports and version. async‑icq typically uses ORDERED channels.

Testnet:

```bash
hermes create channel \
  --a-chain bitora \
  --a-port icqcontroller \
  --b-chain osmo-test-5 \
  --b-port icqhost \
  --order ordered \
  --channel-version icq-1
```

Mainnet:

```bash
hermes create channel \
  --a-chain bitora \
  --a-port icqcontroller \
  --b-chain osmosis-1 \
  --b-port icqhost \
  --order ordered \
  --channel-version icq-1
```

Verify

```bash
hermes query channels --chain bitora | grep icqcontroller || true
hermes query channels --chain osmo-test-5 | grep icqhost || true
# or mainnet
hermes query channels --chain osmosis-1 | grep icqhost || true
```

Relay Packets

- To relay packets continuously (ICQ queries and acks), run Hermes workers:

Testnet:

```bash
hermes start --chains bitora,osmo-test-5
```

Mainnet:

```bash
hermes start --chains bitora,osmosis-1
```

Notes

- Port IDs: controller `icqcontroller` (Bitora), host `icqhost` (Osmosis async‑icq).
- Version: `icq-1` must match both sides.
- Ordering: if handshake is rejected for ordering, retry with `--order unordered` (depends on host config).
- Ensure host allowlists (stores/keys) permit your queries; otherwise packets will ack with errors.
- Capability for `ports/icqcontroller` is owned at genesis by this repo, so handshake auth should pass.

Troubleshooting

- `invalid counterparty version` → use `--channel-version icq-1`.
- `channel open failed: unauthorized` → verify Bitora owns `ports/icqcontroller` and Hermes is using the correct port IDs.
- Client/connection creation fails → verify `rpc/grpc` endpoints and chain IDs in `hermes/config.toml`.
