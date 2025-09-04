Rencana Implementasi Oracle Harga USD Multi-Sumber (Bitora)

Bitora akan mengintegrasikan oracle harga USD multi-sumber untuk token BTO dengan prioritas dan fallback sebagai berikut: (1) harga DEX Osmosis (pasar on-chain), (2) Band Protocol (oracle desentral via IBC), dan (3) CoinGecko (data off-chain). Harga-harga ini dikonsolidasi di modul x/oracle on-chain dengan mekanisme Last Good Price (LGP) sebagai fallback untuk menjaga stabilitas, kemudian disajikan ke modul fees agar perhitungan biaya transaksi (USD→ubto) akurat dan tahan gangguan.

Tahap 1 — Osmosis: ICS20 + Pool + Feeder TWAP (Prioritas Utama)

Tujuan: secepatnya mendapatkan harga BTO/USD dari Osmosis (on-chain DEX) dan memakainya di modul fees via x/oracle.

Langkah:

- ICS20 ke Osmosis: pastikan relayer siap dan jalur transfer Bitora ↔ Osmosis aktif. Daftarkan denom USDC (ibc/…) dan BTO.
- Listing & Likuiditas: buat/daftarkan pool BTO/USDC di Osmosis dan siapkan likuiditas awal untuk price discovery.
- Feeder TWAP (off-chain sementara): service off-chain ambil harga TWAP dari Osmosis (gamm/twap) untuk pasangan BTO/USDC. Jika 2-hop, gunakan BTO/OSMO dan OSMO/USDC lalu komposisi. Rekomendasi window 5–15 menit, cek min liquidity dan deviation guard.
- Kirim ke on-chain: feeder membentuk MsgSetPrice (modul x/oracle) dengan source "osmosis" dan nilai BTO per USD (BTO/USD). Hanya alamat dalam AuthorizedFeeders yang diterima.
- Params yang perlu disiapkan (governance/konfigurasi): pool_id, twap_window, min_liquidity, max_deviation, update_interval, AuthorizedFeeders.

Catatan IBC v10: untuk modul `pricefeed`, tidak perlu port binding manual via PortKeeper seperti versi lama; routing IBC sudah didaftarkan di `app/ibc.go`. Tahap Band (lihat di bawah) cukup memastikan router dan version "pricefeed-1" saat handshake.

Tahap 2 — Persistensi Harga & Sinkronisasi dengan Modul x/oracle

Expected Keeper Oracle: Tambahkan dependency OracleKeeper ke modul pricefeed agar bisa menyimpan harga yang diterima. Definisikan interface minimal di x/pricefeed/types/expected_keepers.go (untuk menghindari circular dependency) dengan method seperti SetBTOPerUSD(ctx sdk.Context, price sdk.Dec) error. Pastikan modul oracle mengimplementasi method ini.

Update ProcessPriceResponse: Di fungsi ProcessPriceResponse milik pricefeed, tambahkan logika untuk mem-parsing hasil dari Band. Paket OracleResponsePacketData berisi field `Prices` (string/JSON) dan `Error`. Jika `Error != ""`, abaikan update dan emit event error. Untuk `Prices`, parse JSON map sederhana, misal: `{ "BTO": "0.123456" }` atau `{ "BTO/USD": "0.123456" }` (pastikan kesepakatan format dengan oracle script Band). Konversi nilai ke desimal (sdk.Dec atau math.LegacyDec), validasi non-zero/non-negative, lalu panggil `OracleKeeper.SetBTOPerUSD(ctx, price)` untuk menyimpan harga ke modul oracle.

Emit Event & Staleness: Setelah update, emit event price_update dengan atribut seperti sumber, harga, request_id, timestamp, dsb untuk monitoring. Selain itu, simpan timestamp update tersebut (mis. di state modul oracle) untuk mendukung pengecekan stale price (usia data) sebelum digunakan oleh modul lain.

Tahap 3 — Desain Aggregator Multi-Sumber di x/oracle

State Harga Multi-Sumber: Perluasan state modul x/oracle untuk menyimpan data dari berbagai sumber secara terpisah dan agregat. Tambahkan struktur:

SourcePrices: map yang memetakan nama sumber -> PriceRecord berisi {price (sdk.Dec), timestamp (int64), confidence (sdk.Dec), origin (enum/label sumber)}. Setiap kali sumber (Band, Osmosis, dll.) mengirim update, record ini diperbarui.

CurrentPrice: harga akhir BTO/USD saat ini (sdk.Dec) hasil agregasi terkini.

LastGoodPrice: harga terakhir yang valid (LGP) yang berhasil lolos semua cek (digunakan sebagai fallback).

LastUpdateTime: timestamp blok terakhir ketika CurrentPrice diperbarui.

Parameter (Governance): Definisikan Params dalam modul oracle untuk mengonfigurasi algoritma agregasi:

SourceWeights: bobot kontribusi masing-masing sumber terhadap harga final (mis. Band 0.5, Osmosis 0.3, CoinGecko 0.2, disesuaikan total 1.0).

MaxSourceAge: batas usia maksimal data per sumber (detik) untuk dianggap “fresh”. Contoh: Band = 180s, Osmosis = 120s, CoinGecko = 300s.

MaxDeviation: ambang deviasi maksimum untuk outlier filtering (dalam desimal, mis. 0.1 berarti 10%).

MinFreshSources: jumlah minimal sumber fresh yang dibutuhkan agar agregasi dianggap valid (mis. 2 sumber).

MaxStaleLGP: durasi maksimal (detik) penggunaan LastGoodPrice saat tidak ada sumber fresh sebelum menandai oracle “unavailable” (mis. 3600 detik = 1 jam).

UseMedian: bool untuk menentukan apakah akan menggunakan median dari harga-harga sumber sebagai basis sebelum penghitungan rata-rata berbobot (untuk mengurangi pengaruh outlier).

AuthorizedFeeders: daftar alamat yang diizinkan untuk mengirim pesan feed harga eksternal (mis. feeder off-chain yang mengirim MsgSetPrice). Hanya transaksi dari alamat ini yang boleh memperbarui harga melalui pesan tersebut.

Msg SetPrice & Otorisasi Feeder: Buat message baru semisal MsgSetPrice dalam modul oracle untuk menerima input harga dari feeder off-chain (untuk sumber Osmosis dan CoinGecko). Message ini mencakup field: denom/symbol atau enum sumber (contoh: "osmosis" atau "coingecko"), harga BTO/USD (sdk.Dec), timestamp atau metadata waktu, serta confidence level (opsional, merefleksikan kepercayaan pada sumber, bisa diisi 1.0 jika tidak digunakan). Tambahkan validasi di handler: hanya MsgSetPrice dari alamat yang terdaftar di AuthorizedFeeders yang dapat diproses (jika tidak, return error unauthorized). Setiap MsgSetPrice berhasil akan memperbarui entry di SourcePrices[sumber] dan bisa langsung memicu reaggregasi.

Detail pesan & validasi:

- Skema MsgSetPrice (proto): `source` (enum/string), `price` (string desimal, hingga 18 desimal), `timestamp` (int64, unix seconds), `confidence` (string desimal), `note/path` (opsional).
- Validasi: `price > 0`, `timestamp` tidak mundur (anti-replay sederhana), `confidence` dalam [0,1] (opsional), `source` dikenali. Tolak jika pengirim bukan AuthorizedFeeder.

Algoritma Agregasi On-Chain: Setiap kali terdapat update harga dari salah satu sumber (atau dipanggil secara berkala tiap blok di EndBlock), lakukan agregasi deterministik:

Kumpulkan Sumber Fresh: Ambil semua nilai dari SourcePrices yang masih fresh (usia data ≤ MaxSourceAge untuk sumber tersebut). Abaikan sumber yang datanya sudah kadaluarsa.

Hitung Median & Filter Outlier: Dari kumpulan harga fresh, hitung median sebagai nilai acuan (jika UseMedian = true, gunakan median; jika false, bisa pakai mean sederhana sebagai acuan). Lalu drop semua harga sumber yang berbeda lebih dari MaxDeviation dari median ini (dianggap outlier). Langkah ini mengeliminasi sumber yang mungkin error atau terlampau menyimpang.

Weighted Average: Terhadap harga-harga yang tersisa setelah filter, hitung rata-rata berbobot menggunakan bobot masing-masing di SourceWeights untuk memperoleh nilai CurrentPrice baru. Bobot yang tidak terpakai (jika suatu sumber ter-drop atau stale) bisa dinormalisasi ulang relatif terhadap total bobot sumber tersisa.

Fallback LGP jika Sumber Kurang: Jika setelah penyaringan, jumlah sumber fresh yang tersisa < MinFreshSources, artinya data tidak cukup untuk menentukan harga yang andal. Dalam kasus ini, jangan update CurrentPrice. Sebagai gantinya, periksa apakah masih bisa menggunakan LastGoodPrice: jika now - LastUpdateTime <= MaxStaleLGP, pertahankan CurrentPrice = LastGoodPrice untuk sementara; jika LGP juga sudah usang melebihi batas, tandai bahwa oracle tidak memiliki harga terkini (mis. set suatu flag OracleAvailable = false).

Update LastGoodPrice: Jika langkah (3) menghasilkan CurrentPrice baru yang valid (jumlah sumber mencukupi) dan lulus sanity check (mis. tidak nol, tidak deviasi ekstrim dibanding LGP sebelumnya di luar batas wajar), maka perbarui LastGoodPrice = CurrentPrice dan catat LastUpdateTime sekarang. Jika harga baru anomali (mis. berubah > X% dalam 1 blok tanpa flag tertentu), bisa abaikan update tersebut demi stabilitas dan tetap pakai LGP.

Governance & Params: Pastikan modul oracle mendukung update parameter via governance (ParamChangeProposal) agar nilai-nilai seperti bobot dan ambang dapat disesuaikan jika diperlukan. Juga, event-event penting (harga berubah, sumber dianggap outlier, fallback digunakan, dll.) di-emit untuk kemudahan pemantauan.

Catatan migrasi:

- Tambahkan KeyTable params untuk kunci: `SourceWeights`, `MaxSourceAge`, `MaxDeviation`, `MinFreshSources`, `MaxStaleLGP`, `UseMedian`, `AuthorizedFeeders`.
- Bump `ConsensusVersion` modul oracle dan sediakan migrasi state untuk field baru (`SourcePrices`, `LastGoodPrice`, dsb.).

Tahap 4 — API / Query Harga

Query Price: Sediakan endpoint query di x/oracle untuk mendapatkan data harga terkini. Misalnya, buat Query/Price (gRPC dan CLI) yang mengembalikan struktur: current_price (BTO per USD), last_good_price, last_update_time (timestamp blok terakhir update), dan daftar sources (harga masing-masing sumber + usia).

Implementasi: Tambahkan method query di keeper dan protobuffer untuk respon. Query ini memungkinkan modul lain atau pengguna off-chain untuk melihat status oracle secara real-time. Pastikan juga modul fees mengambil harga dari sumber tunggal ini (tidak langsung dari modul pricefeed atau sumber lain) untuk konsistensi (single source of truth).

Observability:

- Emit event dengan atribut standar: `source`, `price`, `age`, `used_in_aggregate`, `dropped_as_outlier`, `used_lgp`.
- Tambahkan metrik Prometheus (opsional via telemetry): jumlah sumber fresh, jumlah outlier, penggunaan LGP, umur data rata-rata, status `oracle_available`.

Tahap 5 — Operasional Relayer & Channel IBC (Band Protocol)

Setup Relayer & Channel: Konfigurasikan relayer (contoh: Hermes) dengan menambahkan chain Bitora dan BandChain. Gunakan konfigurasi di folder relayer/ proyek (jika tersedia) sebagai dasar. Langkah yang diperlukan: membuat client IBC ke Band, membuka connection, lalu membuka channel dengan PortID "pricefeed" di Bitora ke port target Band (port oracle Band, biasanya juga bernama "oracle" atau sesuai modul Band). Gunakan channel UNORDERED untuk mengirim paket oracle (tidak masalah urutan) dan pastikan versi aplikasi diset ke "pricefeed-1" di kedua sisi saat handshake.

Kirim Permintaan Oracle (Band): Setelah channel terbuka (mis. channel-0 di Bitora terhubung ke channel di Band), kita bisa mengirim pesan permintaan data. Panggil x/pricefeed message MsgSendOracleRequest dari Bitora. Isi field sesuai kebutuhan BandChain:

OracleScriptID: ID unik skrip oracle di BandChain yang menyediakan harga BTO/USD. Tentukan ID ini (misal Band memiliki oracle script untuk harga kripto) sesuai dokumentasi Band.

Calldata: Data yang dibutuhkan skrip Band. Contohnya, Band memiliki data source yang memerlukan list simbol dan multiplier. Bungkus simbol BTO dan multiplier (mis. 1000000 untuk 6 desimal) dalam format yang benar. Hindari membangun Calldata dengan format string Go default (fmt.Sprintf array, dsb.) yang tidak cocok. Sebagai gantinya, jika format ekspektasi adalah JSON, encoding-lah JSON array simbol lalu konversi ke string hex untuk diisi di Calldata
docs.bandchain.org
. (Contoh: ["BTO", "USDC"] dijadikan JSON bytes lalu hex).

AskCount/MinCount: Setel jumlah validator Band yang diminta memberikan jawaban (AskCount) dan minimal jawaban agar dianggap sukses (MinCount). Contoh, AskCount = 4, MinCount = 3 untuk toleransi 1 kegagalan. Pastikan MinCount <= AskCount.

FeeLimit: Tentukan batas fee (dalam koin BAND) yang bersedia dibayar untuk request ini. Referensi dari Band docs untuk fee sumber data terkait.

PrepareGas & ExecuteGas: Tentukan alokasi gas di Band untuk mengeksekusi skrip oracle. Gunakan nilai standar atau rekomendasi Band (mis. PrepareGas 50_000, ExecuteGas 200_000 tergantung kompleksitas).

ClientID: Berikan ID unik (string) untuk mengidentifikasi request (bisa diisi mis. "BTOPrice1"). Ini akan dipantulkan di response untuk korelasi.

Timeout & Retries: Setel timeout packet IBC yang realistis pada MsgSendOracleRequest (mis. beberapa menit) sesuai perkiraan waktu Band menyelesaikan request. Jika dalam jangka waktu ini paket belum mendapat response, akan terjadi timeout (harus ditangani jika perlu retry).

Proses Response: Ketika BandChain mengirim OracleResponsePacketData kembali (via relayer), modul pricefeed Bitora akan menerima paket tersebut (di OnRecvPacket). Pastikan handler OnRecvOracleResponsePacket memanggil ProcessPriceResponse yang telah diupdate di Tahap 2. Jika Result dari Band berisi error atau status gagal, log error dan jangan update harga. Jika sukses, modul oracle akan di-update dengan harga dari Band. Emit event untuk menandakan data Band masuk.

Verifikasi Format: Uji coba alur ini di environment test: kirim satu request dan pastikan Band merespon dengan data yang bisa di-parse. Cocokkan format Result dengan parsing logic (contoh: apakah Result harus diinterpretasi sebagai big-endian integer, JSON string, dll sesuai oracle script). Gunakan info dari dokumentasi Band untuk decode hasil (Band umumnya mengembalikan nilai agregat dalam byte array pada field Result di paket response
docs.bandchain.org
docs.bandchain.org
).

Operasional relayer tambahan:

- Sertakan contoh konfigurasi Hermes (client/connection/channel) untuk port `pricefeed`. Monitor acknowledgement dan timeout. Pertimbangkan redundant relayer untuk keandalan.
- Di sisi ante handler, manfaatkan `RedundantRelayDecorator` (sudah terpasang di app) untuk meningkatkan keandalan relay paket IBC.

Tahap 6 — Ingest Data Osmosis & CoinGecko (via Off-chain Feeder)

Motivasi Off-chain Feeder: Karena Bitora belum memiliki modul Interchain Query (ICQ) untuk mengambil data langsung dari Osmosis, solusi praktis adalah menggunakan service off-chain (feeder/aggregator) yang menarik data harga dari Osmosis dan CoinGecko, lalu mengirimkannya on-chain. Service ini dijalankan terpisah (bisa sebagai cron job atau daemon) dan berinteraksi dengan chain melalui transaksi.

Harga Osmosis (On-chain DEX): Feeder mengambil harga BTO dari DEX Osmosis:

Gunakan API gRPC/REST Osmosis. Untuk harga yang tahan manipulasi, gunakan modul TWAP `osmosis.twap.v1beta1` dengan endpoint gRPC `Query/ArithmeticTwapToNow` (parameter: `pool_id`, `base_asset`, `quote_asset`, window; atau gunakan request yang sesuai versi Osmosis saat ini). Rekomendasi window 5–15 menit untuk mengurangi volatilitas jangka pendek.

Jika tersedia pool BTO/USDC langsung di Osmosis, ambil TWAP harga BTO→USDC dari pool tersebut. Karena USDC ~ $1, metrik ini langsung merepresentasikan BTO per USD.

Jika pool langsung BTO/USDC tidak ada, gunakan rute 2-hop: BTO→OSMO dan OSMO→USDC. Dapatkan TWAP untuk pool BTO/OSMO dan pool OSMO/USDC, lalu kalkulasi `BTO/USD = (BTO/OSMO) * (OSMO/USD)`. Normalisasi desimal dengan benar (mis. ubto 1e6, uosmo 1e6, uusdc 1e6).

Terapkan sanity check pada hasil Osmosis: jika volume pool sangat kecil atau harga yang didapat outlier dibanding sumber lain (Band/CG), tandai confidence rendah atau skip update tersebut. Pastikan nilai tidak nol atau tidak valid sebelum dikirim.

Harga CoinGecko (Off-chain API): Feeder juga mengambil harga dari CoinGecko:

Query endpoint CoinGecko untuk harga BTO (jika token BTO terdaftar di CG). Misalnya `GET /api/v3/simple/price?ids=<id_bto>&vs_currencies=usd`. Siapkan mapping denom/token → CoinGecko ID (konfigurasi). Tangani rate-limit (HTTP 429) dengan backoff dan caching (ETag/If-None-Match, max-age) untuk efisiensi.

Bila BTO tidak langsung tersedia di CG (misal belum listing), gunakan pendekatan komposit: ambil harga BTO terhadap aset lain yang tersedia. Contoh, jika CG menyediakan BTO/OSMO dan OSMO/USD, feeder bisa menghitung `BTO/USD = (BTO/OSMO) * (OSMO/USD)` mirip metode Osmosis di atas.

Periksa juga staleness data CG: gunakan `last_updated_at` jika tersedia. Jika data terlalu lama atau API error, jangan kirim update. Patuhi rate-limit (umum ~50-100 calls/menit tergantung endpoint) dan gunakan backoff.

Dapat juga ditambahkan confidence score untuk data CG, misal berdasarkan trust_score exchange atau volume perdagangan, jika info tersedia. confidence ini bisa diisi dalam MsgSetPrice untuk digunakan on-chain (mis. CG confidence lebih rendah daripada Band).

Opsional Band API Cross-check: Meskipun harga Band utama masuk lewat IBC (Tahap 5), feeder off-chain dapat pula memanfaatkan Band public API/RPC untuk cross-check cepat. Jika deviasi Band vs Osmosis/CG > MaxDeviation, feeder dapat menurunkan confidence sumber tertentu atau menunda update.

Kirim MsgSetPrice ke Bitora: Setelah mengumpulkan data:

Feeder membentuk transaksi ke chain Bitora dengan pesan oracle.MsgSetPrice untuk setiap sumber. Misal, satu MsgSetPrice{source:"Osmosis", price: <harga_BTO_USD>, confidence: <c>, ...} dan satu lagi MsgSetPrice{source:"CoinGecko", ...}. Masing-masing menyertakan timestamp/metadata asal jika diperlukan.

Pastikan akun pengirim pesan ini adalah feeder yang diotorisasi. Gunakan param AuthorizedFeeders di modul oracle (lihat Tahap 3) untuk membatasi. Sebelum operasi, daftarkan address tersebut via governance (jika param on-chain) atau hardcode sementara untuk devnet.

Gunakan mekanisme Feegrant: Agar feeder dapat mengirim tx berkala tanpa perlu memegang banyak ubto, berikan grant dari akun treasury/ops ke feeder address (type BasicAllowance per periode waktu, dsb). Ini memungkinkan tx tanpa gas fee yang signifikan dibebankan ke feeder.

Atur frekuensi update: misal feeder mengirim update Osmosis tiap 1 menit dan CoinGecko tiap 2 menit, atau adaptif tergantung volatilitas. Jangan terlalu sering mengirim jika data tidak berubah signifikan, untuk efisiensi.

Keamanan & Monitoring: Implementasikan retry dengan backoff di feeder jika tx gagal (contoh: jika channel IBC belum siap untuk Band, atau node Bitora gagal). Log setiap harga yang dikirim dan respon tx. Pasang alert jika feeder menemui error berulang (tanda mungkin ada problem di chain atau API). Rate limiting: jangan spam chain dengan tx; patuhi misal 1 tx per blok per feeder.

Keamanan feeder yang disarankan:

- Simpan kunci feeder di KMS/HSM. Gunakan account khusus dengan izin terbatas. Terapkan anti-replay: tolak `timestamp` yang mundur.
- Gunakan FeeGrant dengan limit ketat (jumlah/kadaluarsa). Batasi frekuensi (rate-limit) pengiriman transaksi dan lakukan batching bila perlu.

Catatan: Solusi off-chain ini bersifat interim. Setelah modul Interchain Queries (ICQ) tersedia, Bitora dapat secara langsung mengambil harga Osmosis on-chain tanpa perantara off-chain, meningkatkan desentralisasi (implementation-nya akan jauh lebih kompleks dan memerlukan perubahan arsitektur yang signifikan, sehingga di luar scope iterasi ini).

Tahap 7 — Integrasi ke Modul Fees & Fallback Policy

Penggunaan Harga di x/fees: Modul x/fees Bitora saat ini menghitung biaya dalam ubto berdasarkan nilai USD dengan asumsi kurs tertentu. Setelah oracle multi-sumber aktif, ubah konfigurasi modul fees untuk mengambil harga terkini dari OracleKeeper. Misalnya, tiap kali perlu konversi USD→ubto, panggil OracleKeeper.GetExchangeRate(ctx, "BTO"). Dengan demikian, begitu oracle.CurrentPrice ter-update, modul fees otomatis menggunakan kurs live tersebut untuk menghitung fee. Ini memastikan fee blockchain (yang ditentukan dalam USD) dikonversi ke jumlah ubto sesuai market rate terbaru.

Fallback & Guard Rails: Tambahkan mekanisme penanganan jika oracle tidak tersedia atau harga stale:

Param Fallback Price & Mode: Definisikan parameter konfigurasi (bisa di modul oracle atau fees) untuk kebijakan fallback. Contoh: FallbackPrice (harga tetap BTO/USD yang konservatif, digunakan jika oracle gagal) dan FallbackMode (modus tindakan). FallbackMode bisa berupa: "use_last_good_price" (gunakan LGP selama masih dalam batas waktu tertentu), "use_fixed_price" (gunakan FallbackPrice yang ditetapkan governance), atau "reject_transactions" untuk kasus ekstrem. Secara default bisa pilih pakai LGP dulu, lalu fallback price.

Staleness Check: Jika current_price terakhir sudah melewati TTL tertentu (mis. >5 menit atau sesuai MaxSourceAge global) dan LGP juga sudah melewati MaxStaleLGP, sistem menandai oracle_unavailable. Dalam status ini, sesuai kebijakan, modul fees dapat:

Menggunakan nilai FallbackPrice (mis. kurs tetap terakhir diset governance) untuk konversi fee, atau

Menolak transaksi tertentu yang sensitif (mis. yang fee-nya besar atau berkaitan dengan keamanan) sambil memberikan pesan error bahwa oracle offline.

Untuk transaksi nominal kecil, mungkin masih diperbolehkan dengan menggunakan fallback price agar chain tetap liveness.

Logging: Ketika fallback dipakai, emit event atau log peringatan. Validator/operator harus waspada jika sering terjadi, karena menandakan masalah pada mekanisme oracle.

Evaluasi & Penyesuaian: Monitor efek dari harga live terhadap biaya. Pastikan ada ceil/floor kecil jika diperlukan (mis. tidak membiarkan fee terlalu rendah jika harga melonjak turun tiba-tiba, untuk mencegah abuse). Atur ulang parameter via governance sesuai kebutuhan (mis. jika volatilitas tinggi, mungkin MaxDeviation diperkecil atau MinFreshSources ditingkatkan).

Testing & Quality Gates:

- Unit tests: agregasi (median, outlier filtering, weighting), LGP fallback, staleness, validasi MsgSetPrice, dan handler Band response (ack/error/timeout).
- Integration/E2E: alur Band IBC (request→response), feeder Osmosis 2-hop/TWAP, feeder CoinGecko, dan efek pada modul fees.
- Property/fuzz tests: robustness terhadap input ekstrem (harga 0/NaN, deviasi ekstrem, timestamp skew), dan memastikan determinisme agregasi.

Dengan tujuh tahap di atas, Bitora akan memiliki oracle multi-sumber yang deterministik dan robust: terhubung melalui IBC ke Band, memanfaatkan data DEX Osmosis, serta sumber publik seperti CoinGecko, semuanya dikoordinasikan on-chain dengan fallback Last Good Price untuk menjaga stabilitas harga. Implementasi ini memastikan perhitungan fee selalu berbasis harga pasar terbaru dengan mitigasi saat data terganggu.

Edge cases tambahan yang perlu diantisipasi:

- Channel IBC terputus (Band) → gunakan Osmosis/CG + LGP; lakukan retry pembukaan channel via relayer.
- Likuiditas Osmosis tipis atau pool regenesis → drop sumber ini sementara (confidence rendah).
- CoinGecko rate-limit/ban → gunakan cache lokal, perbesar interval, atau drop sementara.
- Perbedaan desimal/precision antar sumber → normalisasi sebelum agregasi untuk hindari bias.
- Keterbatasan gas saat banyak sumber → batasi jumlah sumber aktif atau frekuensi agregasi (mis. reaggregate on-demand saat MsgSetPrice masuk).
