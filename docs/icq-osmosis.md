# ICQ Osmosis (pure on-chain) – ringkas

Tujuan: Ambil harga BTO/USD langsung dari Osmosis on-chain via Interchain Queries (ICQ), tanpa backend/off-chain feeder, lalu simpan ke `x/oracle` untuk dipakai `x/fees`.

Komponen:

- ICQ Controller di Bitora (IBC app): mendaftarkan dan menerima hasil query.
- ICQ Host di Osmosis: menyediakan data KV + proof untuk dijawab.
- ICQ Relayer: proses paket ICQ (berbeda dari Hermes).

Alur tinggi:

1. Wiring ICQ Controller (Bitora)

   - Tambah dependensi module ICQ (interchain-queries) di `go.mod`.
   - Daftarkan keeper + store key + IBC route (port ICQ controller) di `app/ibc.go`.
   - Genesis: param koneksi default (connection-id ke Osmosis), interval update.

2. Register Query (Osmosis pools)

   - Daftarkan query KV untuk data pool yang relevan:
     - Opsi A (lebih mudah awal): query state pool (reserves) → hitung Spot Price BTO/USD on-chain (cek min liquidity + deviation guard).
     - Opsi B (lanjutan): query TWAP records (prefix twap) → hitung TWAP sederhana dari sampel multi-height.
   - Simpan definisi query (pool_id, base/quote denoms, window) di state ICQ module.

3. Relayer ICQ

   - Jalankan ICQ relayer (contoh: `interchain-queries` relayer dari ibc-apps/strangelove).
   - Konfigurasi chain Bitora dan Osmosis + connection-id yang sama dengan on-chain.

4. Proses Hasil Query → Oracle

   - Handler ICQ receive: verifikasi proof, decode value pool/TWAP.
   - Hitung `BTO/USD` (BTO per USD):
     - Pool BTO/USDC langsung → normalize decimals → price = BTO/USD.
     - Jika 2-hop (BTO/OSMO dan OSMO/USDC) → price = (BTO/OSMO) \* (OSMO/USD).
   - Guard: cek min liquidity, non-zero, max deviation vs LGP.
   - Persist: `OracleKeeper.SetBTOPerUSD(ctx, price)`.

5. Keamanan & Fallback

   - Params: min_liquidity, max_deviation, max_staleness, update_interval.
   - Fallback: gunakan LGP saat data tidak fresh atau gagal verifikasi.

6. Verifikasi
   - Unit test: parsing, kalkulasi harga, guard rails, LGP.
   - E2E: ICQ terdaftar → relayer aktif → harga tersimpan → `x/fees` pakai kurs terkini.

Catatan implementasi cepat:

- Start dengan Spot Price (pool reserves) untuk versi 1; TWAP via multi-sample menyusul.
- Dokumentasikan key-prefix Osmosis yang dipakai (gamm/twap) di kode agar mudah di-audit.
- Gunakan event untuk telemetry: `source=osmosis_icq`, `price`, `liquidity`, `used_lgp`.

Status & Relayer notes

- Query status via AutoCLI:
  - bitorad q osmosisicq status
  - Output: last_update, next_update, last_result_time, map query IDs (twap/spot) jika tersedia.
- Relayer: gunakan ICQ relayer (bukan Hermes). Pastikan connection-id di params cocok dengan relayer config.
- Sementara, `x/osmosisicq` menyediakan stub `OnKVResult(store, key, value)` untuk ingest:
  - store "twap" atau "gamm" akan diarahkan ke handler TWAP/Spot.
  - Nilai stub bisa JSON {"price","liquidity"} atau string "price|liquidity" untuk memudahkan uji coba.
  - Untuk produksi, lakukan decode state Osmosis sebenarnya dan panggil handler dengan harga/likuiditas yang telah dinormalisasi.

ICQ controller binding (depinject)

- Jika modul ICQ controller tersedia, expose implementasi `types.ICQClient` dengan method:
  - RegisterKVQuery(ctx, connectionID, store, key) (string, error)
- `x/osmosisicq` menerima dependency opsional ini melalui depinject (lihat `x/osmosisicq/module/depinject.go`).
- Saat tersedia, scheduler akan mendaftarkan query secara periodik dan menyimpan Query ID ke state.

Real Osmosis TWAP (build tag)

- File `x/osmosisicq/keeper/osmo_twap_real.go` dikompilasi hanya dengan `-tags osmosis_real`.
- Lengkapi implementasi BuildTwapKey/DecodeTwap menggunakan key format & proto Osmosis.
- Build lokal dengan integrasi real:
  - Tambahkan dependency Osmosis v30 ke go.mod: `github.com/osmosis-labs/osmosis/v30` (disarankan v30.0.2).
  - Catatan: menambahkan modul ini bisa memicu upgrade transitif (cosmossdk.io/_, cloud.google.com/_, dll). Lakukan ini di branch tersendiri dan audit perubahan.
  - Contoh:
    - go get github.com/osmosis-labs/osmosis/v30@v30.0.2
    - go test -tags osmosis_real ./x/osmosisicq/...
    - atau build node: go build -tags osmosis_real ./cmd/bitorad
  - Jika dependency drift tidak diinginkan, tetap pakai build default (tanpa tag) yang memakai stub.

Stub ICQ adapter

- `x/osmosisicq/controller/adapter_stub.go` menyediakan adapter in-process yang memenuhi `types.ICQClient`.
- Gunakan untuk prototyping: adapter.RegisterKVQuery(...) → adapter.ReceiveKVResult(...) yang memanggil `keeper.OnKVResult`.

Async ICQ (opsional, di balik build tag)

- Untuk mengaktifkan controller ICQ nyata berbasis `async-icq`, sediakan implementasi `types.ICQClient` dan wiring IBC port/controller di balik build tag, misal `-tags icq_async`.
- Hook yang disediakan:
  - `app/maybeRegisterICQStores` dan `app/registerICQAsync` untuk registrasi store & route IBC.
  - Provider depinject opsional di `x/osmosisicq/module/icq_async_provider_*.go` untuk menyuplai `ICQClient` nyata.
- Build default tanpa tag tetap menggunakan client registrasi minimal dan tidak mengirim paket ICQ.
