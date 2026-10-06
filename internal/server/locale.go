package server

import "strings"

// Localize presentation values without changing filter keys, stored metadata,
// analytics grouping, or API responses.
func uiText(value string) string {
	labels := map[string]string{
		"visible links": "Tautan ditampilkan", "matching links": "Tautan cocok", "clicked links": "Tautan diklik", "active links": "Tautan aktif", "paused links": "Tautan nonaktif", "total clicks": "Total klik",
		"last 7 days": "7 hari terakhir", "last 30 days": "30 hari terakhir", "last 90 days": "90 hari terakhir", "all time": "Sepanjang waktu", "by link": "per tautan", "by provider": "per penyedia", "by channel": "per channel", "by campaign": "per kampanye",
		"Other": "Lainnya", "Unassigned": "Belum diisi", "new": "Baru", "Direct / no referrer": "Langsung / tanpa sumber", "Yesterday": "Kemarin", "no clicks yet": "Belum ada klik",
		"active": "Aktif", "paused": "Dijeda", "ended": "Berakhir", "expired": "Kedaluwarsa", "upcoming": "Belum dimulai", "hidden": "Disembunyikan", "needs homepage link": "Butuh tautan kartu",
		"site name is required": "Nama halaman wajib diisi", "featured slides must be between 1 and 3": "Jumlah pilihan utama harus antara 1 dan 3", "slug and url are required": "Slug dan URL wajib diisi", "slug is reserved": "Slug ini dipakai oleh aplikasi", "title is required": "Nama item wajib diisi", "homepage slug is required": "Slug kartu wajib diisi", "priority must be a non-negative number": "Prioritas harus berupa angka nol atau lebih", "invalid program status": "Status program tidak valid", "invalid behavior after program ends": "Pilihan saat program berakhir tidak valid", "end date must be on or after start date": "Tanggal akhir harus sama atau setelah tanggal mulai", "a destination URL is required for redirect after the program ends": "URL tujuan wajib diisi untuk pengalihan setelah program berakhir", "image URL must be an HTTPS URL or a site path": "URL gambar harus berupa HTTPS atau path di situs", "slug may not have surrounding spaces or be a dot path": "Slug tidak boleh memiliki spasi di awal/akhir atau berupa path titik", "slug may contain only letters, numbers, hyphens, underscores, and dots": "Slug hanya boleh berisi huruf, angka, tanda hubung, garis bawah, dan titik",
	}
	if translated, ok := labels[value]; ok {
		return translated
	}
	for _, prefix := range []struct{ old, translated string }{{"daily clicks · ", "Klik harian · "}, {"clicks · ", "Klik · "}, {"Today ", "Hari ini "}} {
		if strings.HasPrefix(value, prefix.old) {
			return prefix.translated + strings.TrimPrefix(value, prefix.old)
		}
	}
	return strings.NewReplacer(" must start with http:// or https://", " harus diawali http:// atau https://", " must be a valid date", " harus berupa tanggal yang valid", " must be ", " harus maksimal ", " characters or fewer", " karakter", "source URL", "URL sumber", "fallback URL", "URL pengganti", "affiliate URL", "URL tujuan", "site description", "deskripsi halaman", "site name", "nama halaman", "affiliate disclosure", "keterangan referral", "start date", "tanggal mulai", "end date", "tanggal akhir", "status effective date", "tanggal perubahan status", "last verified date", "tanggal pemeriksaan", "notice message", "pesan pengunjung", "button label", "teks tombol").Replace(value)
}
