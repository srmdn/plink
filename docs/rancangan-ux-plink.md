# Rancangan UX Plink: tautan, katalog, dan mini-shop

Status: **usulan untuk direview**, belum menjadi perilaku aplikasi.

Plink tetap dapat digunakan sebagai pemendek tautan, katalog rekomendasi,
mini-shop dengan pemesanan manual, atau portofolio. Pemilik memilih fitur yang
dibutuhkan; pengunjung melihat navigasi dan tindakan yang sesuai dengan isi situs.

## Kondisi sekarang

- Admin memiliki menu **Tautan**, **Katalog**, **Statistik**, dan **Pengaturan**.
- Katalog menyimpan item produk, referral, jasa, resource, artikel, dan proyek.
  Jasa dan proyek sudah memiliki tampilan publik masing-masing.
- Kartu produk dan referral saat ini mengarah ke slug pendek, lalu ke tujuan.
  Kartu proyek dan artikel mengarah langsung ke URL tujuan.
- Tautan WhatsApp dapat digunakan sebagai tujuan, tetapi harga, detail produk,
  pilihan varian, dan pembuat pesan pesanan belum tersedia sebagai fitur khusus.
- Statistik mencatat klik pengalihan; belum mengukur pesanan, pembayaran, atau
  komisi. Favorit disimpan di browser pengunjung.
- Fitur publik sudah dapat dinyalakan dan dimatikan di Pengaturan. Perubahan
  lokal yang sedang dikerjakan juga menambahkan tautan tidak tercantum.

## Prinsip keputusan

1. **Pisahkan maksud pengunjung.** Orang yang berbelanja, mencari jasa, atau
   melihat portofolio dapat menuju bagian yang sesuai.
2. **Satukan pengelolaan tanpa menyatukan semua tampilan.** Satu item katalog
   tetap punya jenis dan dapat muncul di bagian publik yang sesuai.
3. **Jelaskan tujuan tombol.** Pengunjung tahu apakah akan membuka WhatsApp,
   situs penyedia, detail produk, atau situs proyek.
4. **Tampilkan informasi sesuai jenis item.** Produk fisik, produk digital,
   referral, jasa, dan proyek tidak dipaksa memakai semua isian yang sama.
5. **Pertahankan perilaku lama secara default.** Pengelola memilih sendiri
   apakah item lama memakai alur baru; slug lama dan tujuan tidak berubah.

## Navigasi publik

Usulan bawaan untuk situs dengan semua bagian aktif:

```text
Beranda   Katalog   Jasa   Proyek   Favorit
```

**Katalog** menggantikan label **Kategori** karena halaman tersebut memuat
item, pencarian, dan filter. Kategori penting tetap terlihat di beranda atau
di bagian atas katalog, misalnya **VPS**, **Produk digital**, dan **Aksesori**.
Nama dan urutan kategori berasal dari data pemilik, bukan ditentukan Plink.

Jasa dan Proyek tetap merupakan halaman berbeda. Pemilik dapat menonaktifkan
fitur yang tidak dipakai melalui pengaturan yang sudah ada. Untuk mini-shop
sederhana, susunannya dapat menjadi **Beranda · Katalog**, dengan kontak yang
mudah ditemukan. Situs campuran dapat mengaktifkan kembali Jasa dan Proyek.

Penggantian label navigasi bisa menjadi perubahan awal. Pengaturan nama dan
urutan menu per instalasi memerlukan desain dan pengujian tersendiri; jangan
anggap fitur itu sudah ada. URL lama tetap dipertahankan saat label berubah.

Favorit tetap opsional dan harus menjelaskan bahwa isinya tersimpan di browser
tersebut. Tombol kontak mengambang bersifat opsional, punya nama yang jelas,
dan tidak boleh menutupi navigasi bawah, konten, atau tombol utama di layar
kecil. Saat fitur kontak dimatikan, informasi kontak lain tetap bisa disediakan
pemilik melalui profil atau halaman yang tersedia.

## Sketsa halaman publik

Contoh mini-shop, dengan data fiktif:

```text
BERANDA
[Logo toko]
Perlengkapan dan panduan pilihan
[Cari produk atau layanan             ] [Cari]

Kategori: [Semua] [Digital] [Aksesori] [VPS]

Pilihan utama
[Gambar]                  [Gambar]
Panduan fotografi         Tas kamera
Rp79.000                  Rp240.000
[Lihat detail]            [Lihat detail]

Punya pertanyaan? [Hubungi kami]
-------------------------------------------------
Beranda                 Katalog
```

```text
DETAIL PRODUK DIGITAL
‹ Kembali ke katalog
[Gambar panduan]
Panduan fotografi
Rp79.000
PDF · dikirim setelah pembayaran dikonfirmasi

Ringkasan isi dan cara menerima produk
[Pesan via WhatsApp]
Pesanan dan pembayaran dikonfirmasi lewat chat.
```

```text
DETAIL PRODUK FISIK
‹ Kembali ke katalog
[Gambar tas kamera]
Tas kamera
Rp240.000
Warna: hitam
Ongkir dikonfirmasi saat pemesanan
[Pesan via WhatsApp]
```

Contoh ini hanya ilustrasi. Jika harga belum diisi, jangan tampilkan `Rp0`;
gunakan keterangan seperti **Tanyakan harga** bila dipilih pemilik. Jangan
menyatakan stok tersedia atau memberi estimasi ongkir tanpa data dari pemilik.
Sediakan informasi pemenuhan dan kebijakan pengembalian sesuai jenis produk.

## Alur pengunjung per skenario

| Skenario | Alur yang diusulkan | Keterangan |
| --- | --- | --- |
| Produk fisik | Katalog → detail produk → Pesan via WhatsApp | Tampilkan harga jika tersedia, varian yang relevan, dan keterangan ongkir atau cara konfirmasinya. |
| Produk digital | Katalog → detail produk → Pesan via WhatsApp | Jelaskan format, isi, serta cara dan waktu penyerahan setelah pembayaran dikonfirmasi. |
| Referral | Katalog → Lihat penawaran → situs penyedia | Tampilkan identitas penyedia dan pengungkapan afiliasi yang sesuai. Jangan menyamakan klik dengan pembelian. |
| Jasa | Jasa → detail layanan atau Hubungi → kanal kontak | Jelaskan layanan, kisaran harga jika tersedia, dan langkah berikutnya. |
| Proyek | Proyek → detail proyek atau situs proyek | Bedakan portofolio dari produk yang bisa dipesan. |

Untuk WhatsApp, pesan awal dapat memuat nama produk, URL detail, dan pilihan
yang sudah tersedia. Pengunjung tetap meninjau dan mengirim pesan sendiri.
Contoh: “Halo, saya tertarik dengan Tas kamera. Apakah masih tersedia?”
Jangan memasukkan data pribadi atau mengklaim pesanan sudah berhasil otomatis.

Tombol pada kartu harus sesuai tindakan: **Lihat detail** untuk halaman produk,
**Lihat penawaran** untuk referral, dan **Lihat proyek** untuk portofolio.
Kontak mengambang adalah jalur bantuan umum; tombol pesan pada produk membawa
konteks produk yang dipilih. Keduanya tidak saling menggantikan.

## Rancangan admin

Pertahankan empat menu utama, dengan deskripsi yang muncul pada halaman kosong
dan melalui bantuan singkat:

| Menu | Pertanyaan yang dijawab |
| --- | --- |
| **Tautan** | URL pendek apa yang saya bagikan dan berapa kali diklik? |
| **Katalog** | Kartu, produk, jasa, dan proyek apa yang saya tampilkan? |
| **Statistik** | Tautan, penyedia, atau channel mana yang mendapat klik? |
| **Pengaturan** | Bagaimana identitas, kontak, dan bagian publik situs diatur? |

Contoh hubungan yang ditampilkan di admin: **GreenCloud** adalah satu item
katalog; `greencloud-blog` dan `greencloud-instagram` adalah tautan yang
terhubung untuk membedakan sumber klik. Tautan mandiri tidak wajib punya item.

### Tambah item

```text
Tambah item

Apa yang ingin Anda tambahkan?
[Produk]       [Rekomendasi / referral]
[Jasa]         [Proyek]
[Artikel]      [Resource / tautan]
```

Memilih jenis membuka formulir dengan isian utama terlebih dahulu:

- **Produk:** nama, gambar, deskripsi, jenis fisik/digital, harga opsional,
  dan cara pemesanan. Pengaturan varian dan pemenuhan mengikuti jenis produk.
- **Rekomendasi:** nama, penyedia, gambar, deskripsi, dan URL tujuan.
- **Jasa:** nama layanan, deskripsi, harga opsional, dan cara menghubungi.
- **Proyek:** judul, gambar, deskripsi, dan URL proyek jika ada.
- **Artikel/resource:** nama, deskripsi, dan tujuan sesuai jenis konten.

**Lanjutan** menampung prioritas, tanggal, status program, channel, dan
kampanye yang relevan. Nilai yang sudah ada harus tetap terlihat dan tersimpan
saat formulir diedit; menyembunyikan bagian lanjutan tidak boleh menghapusnya.
Produk tanpa gambar, harga, atau varian tetap bisa dibuat jika pemilik memilih
itu. Validasi harus memberi tahu isian yang perlu diperbaiki dan menjaga input.

### Status yang terbaca

| Label | Arti |
| --- | --- |
| **Kartu disembunyikan** | Kartu item tidak muncul di katalog; status tautan dan program tetap berlaku. |
| **Tidak tercantum** | Tautan mandiri aktif tidak muncul di daftar publik, tetapi dapat dibuka lewat URL. |
| **Tautan nonaktif** | Tautan pendek mengembalikan 404. |
| **Program dijeda/berakhir** | Slug yang terkait mengikuti pemberitahuan atau aturan tujuan pengganti. |

Saat tautan tidak tercantum dipakai sebagai tautan kartu, jelaskan bahwa slug
bisa muncul di publik jika kartunya ditampilkan. Saat mengubah status program,
jelaskan bahwa seluruh slug terkait dapat terpengaruh. Jangan menggunakan kata
“privat” untuk tautan yang masih bisa dibuka siapa pun dengan URL-nya.

## Tahapan dan batas

### Tahap 1: kejelasan tanpa mengubah alur lama

- Ganti label publik **Kategori** menjadi **Katalog**, pertahankan URL.
- Uji akses kategori penting dari beranda dan katalog.
- Rapikan bantuan Tautan/Katalog, pilihan jenis item, dan penjelasan status.
- Susun ulang formulir secara bertahap tanpa menghapus nilai atau mengubah
  tindakan tombol item lama. Uji ponsel, desktop, dan keyboard.

### Tahap 2: mini-shop WhatsApp

- Rancang dan implementasikan harga, jenis fisik/digital, detail produk, dan
  cara pemesanan; simpan harga sebagai nilai terstruktur dengan mata uang,
  bukan teks angka yang ambigu. Harga kosong berbeda dari harga nol.
- Tambahkan opsi detail produk dan pembuat tautan WhatsApp dengan pesan awal.
  Nomor dinormalisasi sesuai format internasional; teks pesan di-URL-encode.
- Definisikan alur stok, varian, ongkir, dan penyerahan sebagai informasi yang
  diisi pemilik; jangan menjanjikan penghitungan atau pemenuhan otomatis.
- Item lama tetap memakai tujuan dan perilaku yang sudah dipublikasikan hingga
  pemilik memilih alur baru. Detail halaman harus punya URL sendiri yang tidak
  mengambil alih atau merusak slug pendek yang sudah dibagikan.
- Pisahkan metrik **klik ke WhatsApp** dari pesanan dan pembayaran. Tidak ada
  sistem pembayaran, manajemen pesanan, atau inventaris otomatis pada tahap ini.

### Tahap 3: evaluasi berdasarkan penggunaan

- Evaluasi apakah pemilik membutuhkan pengaturan urutan/nama navigasi, varian
  terstruktur, beberapa gambar, keranjang, atau integrasi transaksi.
- Jangan menambah kompleksitas tersebut sebelum skenario toko sederhana diuji.

Perubahan yang memerlukan kolom database atau rute baru perlu rancangan migrasi,
kompatibilitas versi lama, backup, dan pengujian rollback tersendiri sebelum
release. Rancangan ini tidak memberi izin untuk deploy atau mengubah data
production.

## Rencana uji kegunaan

Gunakan data fiktif pada instalasi lokal yang bersih. Uji dengan setidaknya satu
orang yang belum mengenal Plink untuk setiap skenario; hasil awal bersifat
kualitatif dan bukan bukti statistik. Amati tanpa memberi petunjuk menu.

1. **Pemilik baru:** buat tautan pendek, lalu jelaskan perbedaan tautan aktif,
   tidak tercantum, dan nonaktif.
2. **Pemilik toko:** buat produk digital dan produk fisik, isi harga, dan
   siapkan pemesanan via WhatsApp tanpa menganggap ada pembayaran otomatis.
3. **Pengunjung toko:** temukan produk dari kategori, pahami harga dan cara
   penyerahan, lalu buka pesan WhatsApp dengan produk yang benar.
4. **Pemilik afiliasi:** buat satu item dan dua slug channel; temukan statistik
   per channel tanpa menganggap klik sebagai pembelian.
5. **Pengunjung portofolio:** temukan proyek dan bedakan proyek dari jasa.
6. **Aksesibilitas:** gunakan keyboard dan pembaca layar; periksa nama kontrol,
   fokus, kontras, ukuran sentuh, serta tab bawah dan tombol kontak di ponsel.

Catat keberhasilan, langkah yang membingungkan, dan masalah yang berulang.
Periksa juga item lama, slug lama, pilihan fitur nonaktif, layar sempit, teks
panjang, gambar kosong, dan item tanpa harga sebelum menyetujui implementasi.

## Landasan dan batas riset

- [Baymard: navigasi kategori pada perangkat mobile](https://baymard.com/research-articles/main-navigation-product-categories)
  menunjukkan pentingnya kategori yang mudah ditemukan; riset tersebut juga
  mencatat satu pintu “Shop” dapat bekerja untuk katalog kecil.
- [Baymard: informasi biaya pengiriman pada halaman produk](https://baymard.com/research-articles/show-shipping-costs-on-product-pages)
  menunjukkan mengapa informasi ongkir membantu keputusan pembelian.
- [WhatsApp: click to chat](https://faq.whatsapp.com/5913398998672934)
  menjelaskan format nomor internasional dan pesan awal yang di-URL-encode.

Riset tersebut menjadi dasar hipotesis desain, bukan bukti bahwa rancangan ini
sudah mudah digunakan oleh pengguna Plink. Validasi langsung tetap diperlukan.
