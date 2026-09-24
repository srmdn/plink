# Analytics backlog

Catatan ini menyimpan opsi lanjutan yang sengaja ditunda dari release analytics
dashboard saat ini. Fitur inti tetap fokus pada pengelolaan link, ranking
provider, dan pembacaan performa berdasarkan slug.

## Status

- [x] **Trend yang kontekstual** — kolom Trend disembunyikan ketika filter
  periode berada di `all`, karena tidak ada periode pembanding yang bermakna.
  Sudah diterapkan di redesign lokal; belum dirilis.
- [ ] **Bulk edit metadata** — ubah provider, channel, campaign, atau kategori
   untuk beberapa link sekaligus.
- [ ] **Status strategi promosi** — tambahkan status manual `push`, `observe`, dan
   `pause` agar ranking bisa langsung menjadi daftar kerja promosi.
- [ ] **Conversion dan revenue tracking** — hubungkan klik dengan conversion,
   komisi, dan revenue supaya ranking tidak hanya berdasarkan traffic.

Belum ada perubahan backlog ini yang masuk ke production. Tiga item tersisa
perlu dievaluasi setelah dashboard dipakai dengan data produksi.
