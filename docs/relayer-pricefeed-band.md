# Relayer setup: pricefeed <-> Band (IBC v10)

This guide opens an UNORDERED channel between Bitora `pricefeed` port and BandChain oracle port to exchange oracle request/response packets.

Prereqs:

- Hermes (or rly) installed and configured.
- Both chains running and reachable by relayer RPC.
- Bitora app already includes IBC routes for `pricefeed` (done in `app/ibc.go`).

Key params:

- PortID (Bitora): `pricefeed`
- App version: `pricefeed-1` (see `x/pricefeed/types.Version`)
- Order: UNORDERED

Example (Hermes):

1. Create clients and connection (example names; adapt IDs):
   - hermes create client --host-chain bitora --reference-chain band
   - hermes create client --host-chain band --reference-chain bitora
   - hermes create connection --a-chain bitora --b-chain band
2. Create channel with custom port and version:
   - hermes create channel --a-chain bitora --a-connection connection-0 \
     --a-port pricefeed --b-port oracle --order unordered --version pricefeed-1

Notes:

- Replace `oracle` on Band with the actual counterparty port for their oracle app.
- Ensure relayer has sufficient fees on both chains.
- If version mismatch occurs, verify both sides use `pricefeed-1`.

Troubleshooting:

- handshake fails: check ports exist in each app’s router; verify order=unordered; version matches.
- packets time out: bump timeoutTimestamp when sending requests; ensure relayer is running.
