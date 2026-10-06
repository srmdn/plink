(() => {
  const key = 'plink-favorites-v1';
  const read = () => { try { const value = JSON.parse(localStorage.getItem(key) || '[]'); return new Set(Array.isArray(value) ? value.filter(v => typeof v === 'string') : []); } catch { return new Set(); } };
  let favorites = read();
  const status = document.querySelector('.favorite-status');
  let feedbackTimer;
  function feedback(message) {
    if (!status) return;
    clearTimeout(feedbackTimer); status.textContent = message; status.classList.add('is-visible');
    feedbackTimer = setTimeout(() => status.classList.remove('is-visible'), 3000);
  }
  function sync() {
    document.querySelectorAll('[data-favorite]').forEach(button => { const saved = favorites.has(button.dataset.favorite); button.setAttribute('aria-pressed', String(saved)); button.classList.toggle('is-saved', saved); if (!button.dataset.saveLabel) button.dataset.saveLabel = button.getAttribute('aria-label'); button.setAttribute('aria-label', saved ? button.dataset.saveLabel.replace(/^Simpan /, 'Hapus ').replace(/ ke favorit$/, ' dari favorit') : button.dataset.saveLabel); });
    let visible = 0;
    document.querySelectorAll('[data-favorite-card]').forEach(card => { const saved = favorites.has(card.dataset.favoriteCard); card.classList.toggle('is-favorite', saved); if (saved) visible++; });
    document.querySelectorAll('[data-favorite-count]').forEach(el => { el.textContent = favorites.size || ''; });
    const empty = document.querySelector('[data-favorites-empty]'); if (empty) empty.hidden = visible > 0;
  }
  document.addEventListener('click', event => {
    const button = event.target.closest('[data-favorite]'); if (!button) return;
    favorites = read(); const slug = button.dataset.favorite;
    favorites.has(slug) ? favorites.delete(slug) : favorites.add(slug);
    try { localStorage.setItem(key, JSON.stringify([...favorites])); feedback(favorites.has(slug) ? 'Disimpan ke favorit.' : 'Dihapus dari favorit.'); }
    catch { favorites = read(); feedback('Browser tidak mengizinkan penyimpanan favorit.'); }
    sync();
  });
  window.addEventListener('storage', event => { if (event.key === key || event.key === null) { favorites = read(); sync(); } });
  sync();
})();
