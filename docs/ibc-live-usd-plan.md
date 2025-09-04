# Multi-Source Live USD Price Plan (IBC + Off-chain Aggregation)

Dokumen ini merangkum kondisi saat ini di repo `bitora-blockchain`, celah integrasi, serta rencana implementasi bertahap untuk mendapatkan harga USD “live” dari multi sumber dan mengalirkannya ke perhitungan biaya (fees) on-chain.

## Tujuan

- Mendapatkan harga BTO/USD “live” dari tiga sumber, dengan prioritas dan fallback:
  1. Pool USDC di Osmosis (on-chain DEX price)
  2. Band Protocol (via IBC custom app `pricefeed`)
  3. CoinGecko (off-chain API)
- Menyimpan dan mengkonsolidasikan harga multi-sumber secara on-chain (di `x/oracle`), termasuk mekanisme Last Good Price (LGP) untuk fallback.
- Mengekspos harga final ke modul `fees` agar konversi USD→ubto akurat dan tahan gangguan.

## Ringkasan arsitektur yang ada

- App & IBC wiring
  - `app/ibc.go` sudah menginisialisasi IBC v1 dan v2 (ICS20 transfer, ICS27 ICA, wasm) dan menambahkan route `pricefeed` ke IBC router.
  - Light client Tendermint & SoloMachine sudah diregistrasi. Transfer & ICA siap dipakai.
- Modul `x/pricefeed` (custom IBC app)
  - IBC module: `x/pricefeed/module/module_ibc.go` menangani paket `OracleRequestPacket` dan `OracleResponsePacket` (OnRecv/Ack/Timeout).
  - Keeper: `TransmitOracleRequest/Response` (SendPacket) dan helper `ProcessPriceResponse` yang saat ini hanya emit event “price_updated” (belum persist harga).
  - Types: `PortID = "pricefeed"`, `Version = "pricefeed-1"`, message `MsgSendOracleRequest/Response`, paket request/response.
  - Genesis: simpan `PortId` dan `Params` (binding port belum dilakukan).
- Modul `x/oracle`
  - Menyediakan API harga internal: `GetBTOPerUSD()`, `SetBTOPerUSD(ctx, price)`, `GetExchangeRate(ctx, "BTO")`.
  - `GetBTOPerUSD()` membaca nilai tersimpan dari KV (fallback 0 jika belum ada).
- Modul `x/fees`
  - Mengkonsumsi `OracleKeeper.GetExchangeRate(ctx, "BTO")` untuk konversi USD→ubto (fixed-fee sesuai docs wallet).
- Relayer
  - Folder `relayer/` punya config chain Band (mainnet/testnet) sebagai indikasi jalur integrasi.

## Celah yang perlu ditutup

1. Port binding `pricefeed` belum dilakukan (claim/BindPort + capability) → channel handshake bisa gagal.
2. Persistensi harga belum ada → `ProcessPriceResponse` hanya emit event.
3. Integrasi `pricefeed` → `oracle` belum ditautkan → modul fees belum dapat harga live.
4. Query harga (opsional) → disarankan gunakan `oracle` sebagai sumber kebenaran (single source of truth).
5. Format payload Band (Symbols/Calldata) perlu disesuaikan dengan ekspektasi counterparty.
6. Operasional relayer/channel (open channel, version match `pricefeed-1`, timeout policy) perlu didokumentasi dan dijalankan.
7. Belum ada jalur ingest dari Osmosis atau CoinGecko (perlu off-chain aggregator atau ICQ). Saat ini chain belum mengaktifkan Interchain Queries (ICQ), sehingga jalur paling praktis adalah via layanan off-chain (lihat bagian “Aggregator”).

## Rencana implementasi bertahap (Multi-Source)

### Tahap 1 — Wiring IBC app (Port Binding)

- Pada init genesis modul `pricefeed`, lakukan bind port `"pricefeed"` jika belum bound, lalu klaim capability.
- Validasi version `"pricefeed-1"` pada OnChanOpen\* (sudah ada check di module IBC).

### Tahap 2 — Persistensi harga & sinkronisasi dengan `x/oracle`

- Tambahkan dependency `OracleKeeper` ke `pricefeed` via depinject dengan interface minimal (di `x/pricefeed/types/expected_keepers.go`):
  - `SetBTOPerUSD(ctx sdk.Context, price math.LegacyDec) error`
- Update `ProcessPriceResponse`:
  - Parse `response.Prices` (JSON) → ambil harga BTO/USD.
  - Konversi ke `math.LegacyDec` → panggil `OracleKeeper.SetBTOPerUSD(ctx, price)`.
  - Emit event yang kaya konteks (symbol, price, source, request_id, timestamp).
- (Opsional) Simpan timestamp/TTL untuk stale detection.

### Tahap 3 — Desain Aggregator di `x/oracle` (multi-sumber + LGP)

- Tambah struktur data dan params di `x/oracle`:
  - State:
    - `CurrentPrice` (BTO/USD), `LastGoodPrice` (BTO/USD), `LastUpdateTime`.
    - `SourcePrices` map[source]PriceRecord { price (Dec), timestamp (int64), confidence (Dec), origin (enum) }.
  - Params:
    - `SourceWeights` (map: Band, Osmosis, CoinGecko -> weight Dec).
    - `MaxSourceAge` per sumber (detik), mis. Band: 180s, Osmosis: 120s, CoinGecko: 300s.
    - `MaxDeviation` (Dec) untuk deteksi outlier (mis. 0.1 = 10%).
    - `MinFreshSources` (mis. 2) agar final price valid.
    - `MaxStaleLGP` durasi maksimal menggunakan LGP (mis. 1h) sebelum menandai oracle unavailable.
    - `UseMedian` (bool) untuk median-of-sources sebelum weighting (opsional).
- Algoritma agregasi on-chain:
  1. Kumpulkan harga dari sumber yang masih “fresh” (age ≤ MaxSourceAge[sumber]).
  2. Hitung median sebagai nilai acuan; drop outlier di luar `MaxDeviation` dari median.
  3. Dari sisa sampel, hitung weighted average berdasarkan `SourceWeights` untuk `CurrentPrice`.
  4. Jika jumlah sumber fresh < `MinFreshSources`, gunakan `LastGoodPrice` jika belum stale (`now - LGP.ts ≤ MaxStaleLGP`), jika tidak: tandai oracle unavailable.
  5. Update `LastGoodPrice` hanya jika `CurrentPrice` baru valid dan lolos sanity checks (non-zero, tidak out-of-bounds vs LGP > X%).
- Tambah Msg/handler (jika perlu) untuk set params secara governance.

### Tahap 4 — API/Query

- Pastikan `x/oracle` menyediakan query untuk ambil harga BTO/USD terbaru (atau tambahkan sederhana jika dibutuhkan).
  - Tambahkan endpoint: `Query/Price` (mengembalikan `current_price`, `last_good_price`, `last_update`, `sources[]`).

### Tahap 5 — Operasional Relayer & Channel (Band)

- Gunakan konfigurasi di `relayer/` untuk:
  - Create client → connection → open channel (port `pricefeed`, biasanya unordered) ke Band.
  - Pastikan version match `pricefeed-1`.
- Kirim `MsgSendOracleRequest` dari Bitora:
  - Perbaiki `Calldata`/`Symbols` agar sesuai protokol Band (hindari `fmt.Sprintf("%v", symbols)` jika tidak sesuai; gunakan encoding yang diharapkan, mis. JSON array di-hex ke `Calldata`).
  - Set timeout realistis (mis. beberapa menit).

### Tahap 6 — Ingest dari Osmosis & CoinGecko (via Off-chain Aggregator)

Karena chain belum memiliki ICQ, jalur termudah dan stabil adalah aggregator off-chain yang:

- Menarik data harga dari:
  1. Osmosis USDC Pool (on-chain DEX):
     - Ambil data pool(s) relevan via gRPC/REST Osmosis (mis. `gamm`/`poolmanager`) atau modul TWAP `osmosis.twap.v1beta1` untuk harga yang lebih tahan manipulasi.
     - Opsi 1: Jika ada pool langsung `BTO/USDC`, gunakan spot price atau TWAP terbaru.
     - Opsi 2: Jika tidak ada, gunakan jalur pembentuk harga 2-hop, mis. `BTO/OSMO` dan `OSMO/USDC` → `BTO/USDC = (BTO/OSMO) * (OSMO/USDC)`; lebih baik gunakan TWAP untuk tiap hop.
     - Normalisasi ke BTO/USD (ingat USDC ~ USD peg; tambah sanity check deviasi terhadap Band/CoinGecko).
  2. Band Protocol (opsional via API publik hanya untuk cross-check; sumber utama dari Band tetap via IBC packet).
  3. CoinGecko:
     - Ambil `BTO`→USD kalau listing tersedia; jika tidak tersedia, gunakan komposisi dari pair lain (mis. `BTO/OSMO` × `OSMO/USD`).
- Setelah mendapat harga, aggregator menandatangani transaksi `MsgSetPrice` (di `x/oracle`) dengan metadata {source, timestamp, confidence, path}, lalu broadcast ke Bitora.
- Keamanan: governance whitelist address “feeder” (param `AuthorizedFeeders`), gunakan feegrant untuk biaya tx, rate limit dan backoff di sisi service.

Catatan: Jika di masa depan ingin murni on-chain tanpa off-chain aggregator, pertimbangkan integrasi ICQ (Interchain Queries) module untuk menarik state Osmosis langsung on-chain. Ini perubahan besar dan di luar lingkup iterasi cepat.

### Tahap 7 — Integrasi ke Fees

- Karena `fees` membaca `OracleKeeper.GetExchangeRate(ctx, "BTO")`, setelah `oracle` ter-update maka kalkulasi USD→ubto akan otomatis memanfaatkan harga live.
- Tambahkan fallback & guard rails:
  - Jika harga stale/0 → gunakan fallback param, tolak transaksi, atau pakai default fee sesuai kebijakan.

## Detail teknis penting

- Port binding pola umum:
  - Saat `InitGenesis`: jika port belum bound, panggil `PortKeeper.BindPort(ctx, types.PortID)` lalu klaim capability lewat `ScopedKeeper` modul `pricefeed`.
  - Simpan status port bila dibutuhkan, dan pastikan OnChanOpen\* tidak mengizinkan user menutup channel (sudah ditangani).
- Expected keeper untuk `oracle`:
  - Definisikan interface minimal di `x/pricefeed/types/expected_keepers.go` untuk menghindari circular deps.
- `ProcessPriceResponse`:
  - Validasi `data.Error` terlebih dahulu.
  - Parse `data.Prices` → ambil key yang disepakati (mis. "BTO" atau "BTO/USD").
  - Update `oracle` dengan `SetBTOPerUSD` dan emit event.

### Detail Osmosis (DEX/TWAP)

- Spot price formula (Constant Product AMM dua aset A/B): `price(A/B) = reserve_B / reserve_A` (di-normalisasi oleh precision/decimals).
- Untuk komposisi dua hop: `BTO/USDC = (BTO/OSMO) * (OSMO/USDC)`.
- Lebih aman gunakan TWAP dari modul `twap` Osmosis: query gRPC `ArithmeticTwapToNow` untuk window (mis. 5–15 menit) guna tahan manipulasi jangka pendek.
- Normalisasi decimals (BTO 6 desimal `ubto`, USDC 6 desimal) dan validasi nilai non-zero.

### Detail Band (IBC)

- `MsgSendOracleRequest` harus mengisi `OracleScriptId`, `Calldata` (hex), dan parameter (Ask/MinCount, Gas, FeeLimit) sesuai requirement Band.
- Pastikan channel `pricefeed` established dan version cocok.
- `OnRecvOracleResponsePacket` → `ProcessPriceResponse` → update `oracle`.

### Detail CoinGecko

- Map simbol on-chain ke ID CoinGecko (param konfigurasi di aggregator).
- Rate limit & retry/backoff, verifikasi freshness (timestamp respons) dan deviasi terhadap Band/Osmosis.
- Confidence bisa diturunkan dari kualitas data (mis. response age, market depth proxy, trust score CG jika tersedia) → digunakan sebagai `confidence` di `PriceRecord`.

### Agregasi & LGP (Last Good Price)

- Drop outlier di luar `MaxDeviation` dari median.
- Weighted average berdasarkan `SourceWeights` → hasil `CurrentPrice`.
- Update `LastGoodPrice` hanya ketika `CurrentPrice` valid dan tidak deviasi ekstrem terhadap LGP (mis. > 50% dalam 1 blok kecuali ada flag force).
- Jika tidak ada sumber fresh → gunakan LGP jika belum melewati `MaxStaleLGP`; jika melewati, set status `oracle_unavailable = true` (event) dan izinkan policy fees fallback (lihat bawah).

### Kebijakan Fees saat oracle bermasalah

- Jika `oracle_unavailable = true`:
  - Opsi A: gunakan `LastGoodPrice` selama ≤ `MaxStaleLGP`.
  - Opsi B: gunakan `FallbackFixedPrice` param (mis. governance-set), dengan tag bahwa harga fallback dipakai.
  - Opsi C: tolak transaksi kategori tertentu yang kritis (konfigurable per kategori biaya) sampai harga kembali tersedia.

## Risiko dan penanganan edge case

- Channel belum terbuka/capability tidak diklaim → `SendPacket` gagal → pastikan port binding benar.
- Acknowledgement error/format mismatch → handler mengembalikan error (sudah ada guard).
- Data harga kosong/invalid → jangan update oracle; emit event error; gunakan harga lama/fallback.
- Staleness → simpan timestamp & TTL; fees menolak harga stale jika melewati TTL param.
- Osmosis pool tidak ada/likuiditas tipis → gunakan TWAP yang lebih panjang atau drop sumber tersebut (confidence rendah) dan andalkan Band/CoinGecko.
- CoinGecko tidak melisting BTO → gunakan jalur komposisi pair lain atau drop sumber CG.
- Clock skew antara sumber dan chain → evaluasi freshness berdasarkan timestamp sumber vs waktu block.

## Next Steps (aksi konkrit)

1. Port binding `pricefeed` (genesis) + klaim capability.
2. Tambah expected `OracleKeeper` ke `pricefeed` dan wire via depinject; update `ProcessPriceResponse` untuk set harga ke `oracle`.
3. Perluasan `x/oracle` untuk multi-sumber:
   - State `SourcePrices`, `CurrentPrice`, `LastGoodPrice` + Params (weights, TTL, deviation, min sources, max stale LGP, authorized feeders, useMedian).
   - Msg `MsgSetPrice` extended (tambah fields: source enum, confidence, origin path) + auth: hanya `AuthorizedFeeders`.
   - Agregasi on-chain pada setiap update/setiap akhir blok (opsional via `EndBlock`).
4. Off-chain aggregator (di repo `bitora-backend`):
   - Job Osmosis (gRPC/REST + TWAP) dan CoinGecko; normalisasi & kirim `MsgSetPrice`.
   - Konfigurasi rate-limit, retries, dan safety checks.
5. Relayer untuk Band: open channel `pricefeed` (unordered), version `pricefeed-1`; set jalur request/response periodik atau on-demand.
6. Tambah unit test/integration test:
   - Agregasi, outlier rejection, LGP fallback, staleness, fees behavior ketika oracle down.

## Cakupan kebutuhan (requirements coverage)

- Analisa arsitektur & celah: DONE (dokumen ini).
- Rencana implementasi multi-sumber (Osmosis/Band/CoinGecko): DONE (tahap 1–7).
- Rencana operasi relayer & versi/port IBC: DONE (Tahap 5).
- Jalur off-chain aggregator + keamanan feeder: DONE (Tahap 6).
- Perubahan kode yang dibutuhkan: DIJADWALKAN (Next Steps 1–3).
