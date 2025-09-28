# 🚧 COSVIAN - Implementasi Modul Token CSV

Berdasarkan hasil evaluasi kode pada Phase 1, berikut adalah pekerjaan yang harus diselesaikan agar modul token **CSV** dianggap lengkap dan Phase 1 benar-benar rampung.

---

## 📋 TODO List Pengembangan Modul Token CSV

### 1. 📦 Struktur Modul

- [ ] Buat direktori `/x/cosvian/` (jika modul token dipisah dari `cosvian`)
- [ ] Buat struktur standar: `keeper/`, `types/`, `module.go`, `handler.go`, dll.

### 2. 📜 Protobuf Definition

- [ ] Tambahkan file `tx.proto` dan `query.proto` untuk modul CSV
- [ ] Definisikan pesan:
  - [ ] `MsgMint`
  - [ ] `MsgBurn`
- [ ] Update `types/keys.go` untuk tipe message baru

### 3. 🔧 Keeper & Logic

- [ ] Buat keeper untuk mengelola token (mint, burn)
- [ ] Implementasi logic `MintCoin`, `BurnCoin` di keeper
- [ ] Tambahkan error handling & event emitting

### 4. 🎯 Handler

- [ ] Tambahkan switch case untuk `MsgMint` dan `MsgBurn` di `handler.go`
- [ ] Pastikan validasi signer dan logic dijalankan dengan baik

### 5. 🧩 Integrasi ke Aplikasi

- [ ] Register modul `csv` di `app.go`
- [ ] Tambahkan keeper `BtoKeeper`
- [ ] Tambahkan modul ke `ModuleBasics` dan `app.ModuleManager`

### 6. ⚙️ Genesis

- [ ] Tambahkan state awal CSV token jika diperlukan di `genesis.go`
- [ ] Buat `DefaultGenesis()` dan `ValidateGenesis()`

### 7. 🛠️ CLI Support

- [ ] Tambahkan `CmdMint` dan `CmdBurn` di `client/cli`
- [ ] Tambahkan command query balance/token supply

### 8. ✅ Testing

- [ ] Jalankan `ignite chain serve` untuk uji end-to-end
- [ ] Coba `tx` dan `query` melalui CLI

---

## 🧠 Catatan Tambahan

- Kamu bisa menggunakan modul `bank` Cosmos SDK untuk leverage fungsi `mint` dan `burn` bawaan.
- Jangan lupa tambahkan `access control` agar hanya account tertentu yang bisa melakukan mint (misal: admin module).

---

## ⏭️ Langkah Berikutnya

Setelah semua checklist di atas selesai:

- Tandai Phase 1 benar-benar selesai
- Lanjutkan ke Phase 2: Governance, Token Utility, atau Interchain Features
