# Analytics backlog

Catatan ini menyimpan opsi lanjutan yang sengaja ditunda dari release analytics
dashboard saat ini. Fitur inti tetap fokus pada pengelolaan link, ranking
provider, dan pembacaan performa berdasarkan slug.

## Status

- [x] **Trend yang kontekstual** — kolom Trend disembunyikan ketika filter
  periode berada di `all`, karena tidak ada periode pembanding yang bermakna.
  Dirilis di v1.3.0.
- [ ] **Bulk edit metadata** — ubah provider, channel, campaign, atau kategori
   untuk beberapa link sekaligus.
- [ ] **Status strategi promosi** — tambahkan status manual `push`, `observe`, dan
   `pause` agar ranking bisa langsung menjadi daftar kerja promosi.
- [ ] **Conversion dan revenue tracking** — hubungkan klik dengan conversion,
   komisi, dan revenue supaya ranking tidak hanya berdasarkan traffic.

Trend kontekstual sudah dirilis di v1.3.0. Tiga item tersisa belum dikerjakan
dan perlu dievaluasi setelah dashboard dipakai dengan data produksi.
