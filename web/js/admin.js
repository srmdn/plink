// Plink admin behaviors.
//
// This file is the former inline <script> block from
// web/templates/partials/head.html plus the page scripts that used to be
// inlined in the guide, settings, and offers templates. Behavior is
// unchanged; the file is loaded with `defer`, so it runs after the document
// is parsed but before DOMContentLoaded (deferred scripts run first).
//
// Event handlers that used to be inline markup attributes (onclick=,
// onchange=, onsubmit=) are wired here as delegated document listeners on
// data-* hooks, so the markup carries no inline JavaScript and the CSP can
// drop script-src 'unsafe-inline' once the remaining Go-side SVG handlers
// in internal/server/ui.go are converted.

// ── Link modal ────────────────────────────────────────────────────────────
function openModal() {
  document.getElementById('link-modal').classList.remove('hidden');
}
function closeModal() {
  document.getElementById('link-modal').classList.add('hidden');
}

// Called by the server-rendered chart bars (internal/server/ui.go) through
// inline handlers; kept global until that markup moves to data-* hooks.
function selectReportDate(value) {
  var date = document.getElementById('date-filter');
  if (!date || !value) return;
  date.value = value;
  date.dispatchEvent(new Event('change', {bubbles: true}));
}

// ── Analytics donut tooltips ──────────────────────────────────────────────
function initAnalyticsDonutTooltips() {
  document.querySelectorAll('.analytics-donut').forEach(function(chart) {
    var wrap = chart.closest('.analytics-donut-wrap');
    var tooltip = wrap && wrap.querySelector('.analytics-donut-tooltip');
    if (!tooltip) return;
    chart.querySelectorAll('.donut-segment').forEach(function(segment) {
      if (segment.dataset.tooltipBound) return;
      segment.dataset.tooltipBound = '1';
      function showTooltip() {
        tooltip.textContent = '';
        var label = document.createElement('strong');
        label.textContent = segment.dataset.label || '';
        var detail = document.createElement('span');
        detail.textContent = (segment.dataset.clicks || '0') + ' clicks · ' + (segment.dataset.percent || '0') + '%';
        tooltip.appendChild(label);
        tooltip.appendChild(detail);
        tooltip.classList.add('is-visible');
        tooltip.setAttribute('aria-hidden', 'false');
      }
      function hideTooltip() {
        tooltip.classList.remove('is-visible');
        tooltip.setAttribute('aria-hidden', 'true');
      }
      segment.addEventListener('mouseenter', showTooltip);
      segment.addEventListener('focus', showTooltip);
      segment.addEventListener('mouseleave', hideTooltip);
      segment.addEventListener('blur', hideTooltip);
    });
  });
}
document.addEventListener('DOMContentLoaded', initAnalyticsDonutTooltips);

// ── Clipboard helpers ─────────────────────────────────────────────────────
function publicBaseURL() {
  var configured = document.body && document.body.dataset ? document.body.dataset.publicUrl : '';
  return (configured || location.origin).replace(/\/+$/, '');
}
function linkURL(slug) {
  return new URL('/' + encodeURIComponent(slug), publicBaseURL()).href;
}
function copyTextFallback(text) {
  return new Promise(function(resolve, reject) {
    var textarea = document.createElement('textarea');
    textarea.value = text;
    textarea.setAttribute('readonly', '');
    textarea.style.position = 'fixed';
    textarea.style.opacity = '0';
    document.body.appendChild(textarea);
    textarea.select();
    var copied = false;
    try { copied = document.execCommand('copy'); } catch (e) { copied = false; }
    textarea.remove();
    if (copied) resolve(); else reject(new Error('clipboard unavailable'));
  });
}
function copyText(text) {
  if (navigator.clipboard && window.isSecureContext) {
    var clipboardWrite = navigator.clipboard.writeText(text);
    var clipboardTimeout = new Promise(function(_, reject) {
      setTimeout(function() { reject(new Error('clipboard timeout')); }, 800);
    });
    return Promise.race([clipboardWrite, clipboardTimeout]).catch(function() {
      return copyTextFallback(text);
    });
  }
  return copyTextFallback(text);
}
function feedbackButton(btn, text) {
  if (!btn.dataset.label) btn.dataset.label = btn.textContent;
  btn.textContent = text;
  btn.classList.add('copied');
  clearTimeout(btn._feedbackTimer);
  btn._feedbackTimer = setTimeout(function() {
    btn.textContent = btn.dataset.label;
    btn.classList.remove('copied');
  }, 1500);
}
function adminFeedback(message, isError) {
  var status = document.querySelector('.admin-feedback');
  if (!status) { status = document.createElement('div'); status.className = 'admin-feedback ui-toast'; status.setAttribute('role', 'status'); status.setAttribute('aria-live', 'polite'); document.body.appendChild(status); }
  clearTimeout(status._timer); status.textContent = message;
  status.classList.toggle('is-error', !!isError); status.classList.add('is-visible');
  status._timer = setTimeout(function() { status.classList.remove('is-visible'); }, 3000);
}
document.addEventListener('htmx:response:error', function() { adminFeedback('Changes were not saved. Try again.', true); });
document.addEventListener('htmx:error', function() { adminFeedback('Connection lost. Try again.', true); });
document.addEventListener('closeModal', function() { adminFeedback('Link saved.'); });
document.addEventListener('submit', function(event) {
  var form = event.target;
  if (!form.matches('form[method="POST"]') || form.hasAttribute('hx-post')) return;
  var button = event.submitter;
  if (!button || !button.textContent.includes('Save')) return;
  button.textContent = 'Saving…'; button.setAttribute('aria-busy', 'true');
});
function copyLink(btn) {
  var slug = btn.dataset.slug;
  if (!slug) return;
  feedbackButton(btn, 'Copying…');
  copyText(linkURL(slug)).then(function() {
    feedbackButton(btn, 'Copied!'); adminFeedback('Link copied.');
  }).catch(function() {
    feedbackButton(btn, 'Copy failed'); adminFeedback('The link could not be copied. Try again.', true);
  });
}
function copyURL(btn) {
  var url = btn.dataset.url;
  if (!url) return;
  feedbackButton(btn, 'Copying…');
  copyText(url).then(function() {
    feedbackButton(btn, 'Copied!'); adminFeedback('Link copied.');
  }).catch(function() {
    feedbackButton(btn, 'Copy failed'); adminFeedback('The link could not be copied. Try again.', true);
  });
}
function shareURL(btn) {
  var url = btn.dataset.url;
  if (!url) return;
  feedbackButton(btn, 'sharing…');
  if (navigator.share) {
    navigator.share({title: url, url: url}).then(function() {
      feedbackButton(btn, 'shared!');
    }).catch(function(error) {
      if (error && error.name === 'AbortError') return;
      copyText(url).then(function() {
        feedbackButton(btn, 'Copied!'); adminFeedback('Link copied.');
      }).catch(function() {
        feedbackButton(btn, 'share failed');
      });
    });
    return;
  }
  copyText(url).then(function() {
    feedbackButton(btn, 'Copied!'); adminFeedback('Link copied.');
  }).catch(function() {
    feedbackButton(btn, 'share failed');
  });
}

// ── Export menu (was onclick="toggleExportMenu(event)") ────────────────────
// Handled in the delegated click listeners below.

// ── Native <datalist> dropdowns are unreliable across browsers (the caret ──
// can show without opening in some Safari versions). Enhance list inputs
// into a custom combobox that keeps free-text entry while offering the
// suggestions.
function enhanceComboInputs(root) {
  var liveTokens = {};
  document.querySelectorAll('input[data-combo-token]').forEach(function(el) { liveTokens[el.dataset.comboToken] = true; });
  document.querySelectorAll('.combo-list').forEach(function(el) { if (!liveTokens[el.dataset.comboOwner]) el.remove(); });
  (root || document).querySelectorAll('input[list]').forEach(function(input) {
    if (input.dataset.comboEnhanced === '1') return;
    var listId = input.getAttribute('list');
    var datalist = listId ? document.getElementById(listId) : null;
    var values = datalist ? Array.prototype.map.call(datalist.options, function(o) { return o.value; }).filter(Boolean) : [];
    if (!values.length) return;
    input.dataset.comboEnhanced = '1';
    input.removeAttribute('list');
    input.setAttribute('role', 'combobox');
    input.setAttribute('aria-autocomplete', 'list');
    input.setAttribute('aria-expanded', 'false');
    var wrap = document.createElement('div');
    wrap.className = 'combo';
    input.parentNode.insertBefore(wrap, input);
    wrap.appendChild(input);
    var toggle = document.createElement('button');
    toggle.type = 'button';
    toggle.className = 'combo-toggle';
    toggle.tabIndex = -1;
    toggle.setAttribute('aria-label', 'Show options');
    toggle.innerHTML = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m6 9 6 6 6-6"/></svg>';
    wrap.appendChild(toggle);
    var list = document.createElement('ul');
    list.className = 'combo-list';
    list.setAttribute('role', 'listbox');
    list.hidden = true;
    list.id = 'combo-' + Math.random().toString(36).slice(2, 9);
    var token = 'c' + Math.random().toString(36).slice(2, 10);
    input.dataset.comboToken = token;
    list.dataset.comboOwner = token;
    input.setAttribute('aria-controls', list.id);
    document.body.appendChild(list);
    var visible = [];
    var activeIndex = -1;
    function position() {
      var rect = input.getBoundingClientRect();
      list.style.left = rect.left + 'px';
      list.style.top = (rect.bottom + 4) + 'px';
      list.style.width = Math.max(rect.width, 120) + 'px';
    }
    function render(matchValue) {
      var query = matchValue ? input.value.trim().toLowerCase() : '';
      visible = values.filter(function(value) { return !query || value.toLowerCase().indexOf(query) !== -1; });
      list.textContent = '';
      activeIndex = -1;
      input.removeAttribute('aria-activedescendant');
      if (!visible.length) {
        var empty = document.createElement('li');
        empty.className = 'combo-empty';
        empty.textContent = 'No options';
        list.appendChild(empty);
        return;
      }
      visible.forEach(function(value, index) {
        var option = document.createElement('li');
        option.className = 'combo-option';
        option.id = list.id + '-opt-' + index;
        option.setAttribute('role', 'option');
        option.textContent = value;
        option.addEventListener('mousedown', function(event) { event.preventDefault(); });
        option.addEventListener('click', function() { choose(value); });
        list.appendChild(option);
      });
    }
    function open() {
      if (!list.hidden) { position(); return; }
      render(false);
      position();
      list.hidden = false;
      input.setAttribute('aria-expanded', 'true');
      window.addEventListener('scroll', position, true);
      window.addEventListener('resize', position);
    }
    function close() {
      if (list.hidden) return;
      list.hidden = true;
      input.setAttribute('aria-expanded', 'false');
      input.removeAttribute('aria-activedescendant');
      window.removeEventListener('scroll', position, true);
      window.removeEventListener('resize', position);
    }
    function choose(value) {
      input.value = value;
      close();
      input.dispatchEvent(new Event('change', { bubbles: true }));
      input.focus();
    }
    function highlight() {
      Array.prototype.forEach.call(list.children, function(option, index) {
        var isActive = index === activeIndex;
        option.classList.toggle('is-active', isActive);
        option.setAttribute('aria-selected', String(isActive));
      });
      if (activeIndex >= 0 && list.children[activeIndex]) {
        input.setAttribute('aria-activedescendant', list.children[activeIndex].id);
        list.children[activeIndex].scrollIntoView({ block: 'nearest' });
      } else {
        input.removeAttribute('aria-activedescendant');
      }
    }
    function move(delta) {
      if (list.hidden) { open(); return; }
      if (!visible.length) return;
      activeIndex = (activeIndex + delta + visible.length) % visible.length;
      highlight();
    }
    input.addEventListener('focus', open);
    input.addEventListener('click', open);
    input.addEventListener('input', function() { if (list.hidden) open(); render(true); position(); });
    input.addEventListener('keydown', function(event) {
      if (event.key === 'ArrowDown') { event.preventDefault(); move(1); }
      else if (event.key === 'ArrowUp') { event.preventDefault(); move(-1); }
      else if (event.key === 'Enter') { if (!list.hidden && visible.length) { event.preventDefault(); choose(visible[activeIndex >= 0 ? activeIndex : 0]); } }
      else if (event.key === 'Escape') { close(); }
      else if (event.key === 'Tab') { close(); }
    });
    toggle.addEventListener('mousedown', function(event) { event.preventDefault(); });
    toggle.addEventListener('click', function() { if (list.hidden) { input.focus(); open(); } else { close(); } });
    input.addEventListener('blur', function() {
      window.setTimeout(function() { if (!list.hidden && document.activeElement !== input) close(); }, 0);
    });
    document.addEventListener('click', function(event) {
      if (!wrap.contains(event.target) && !list.contains(event.target)) close();
    });
  });
}
document.addEventListener('DOMContentLoaded', function() { enhanceComboInputs(document); });

// ── File inputs ───────────────────────────────────────────────────────────
function enhanceFileInputs(root) {
  (root || document).querySelectorAll('input[type="file"]:not([data-file-enhanced])').forEach(function(input) {
    input.dataset.fileEnhanced = '1';
    input.classList.add('file-native');
    input.tabIndex = -1;
    if (!input.id) input.id = 'file-' + Math.random().toString(36).slice(2);
    var field = input.closest('.field');
    var label = field && field.querySelector('label');
    var labelText = label ? label.textContent.trim().replace(/\s*\(.*$/, '') : '';
    var picker = document.createElement('span');
    picker.className = 'file-picker';
    var button = document.createElement('button');
    button.type = 'button';
    button.className = 'file-picker-btn';
    button.textContent = 'Choose file';
    if (labelText) button.setAttribute('aria-label', 'Choose file ' + labelText);
    var name = document.createElement('span');
    name.className = 'file-picker-name';
    name.textContent = 'No file chosen';
    name.setAttribute('aria-live', 'polite');
    button.addEventListener('click', function() { input.click(); });
    input.addEventListener('change', function() {
      name.textContent = input.files && input.files.length ? input.files[0].name : 'No file chosen';
    });
    picker.appendChild(button);
    picker.appendChild(name);
    input.insertAdjacentElement('afterend', picker);
  });
}
document.addEventListener('DOMContentLoaded', function() { enhanceFileInputs(document); });

// ── Sticky save bar ────────────────────────────────────────────────────────
function initStickySave() {
  document.querySelectorAll('[data-sticky-save]').forEach(function(bar) {
    var form = document.getElementById(bar.getAttribute('data-form') || '');
    if (!form) return;
    var main = form.querySelector('[data-main-save]');
    if (!main) return;
    var scope = bar.getAttribute('data-scope') ? document.querySelector(bar.getAttribute('data-scope')) : null;
    if (!('IntersectionObserver' in window)) { bar.classList.add('is-visible'); return; }
    var visible = function(el) { var r = el.getBoundingClientRect(); return r.top < window.innerHeight - 110 && r.bottom > 0; };
    var mainVisible = visible(main);
    var scopeVisible = !scope || visible(scope);
    var update = function() { bar.classList.toggle('is-visible', !mainVisible && scopeVisible && window.scrollY > 120); };
    new IntersectionObserver(function(entries) { mainVisible = entries[0].isIntersecting; update(); }, { rootMargin: '0px 0px -110px 0px', threshold: 0 }).observe(main);
    if (scope) new IntersectionObserver(function(entries) { scopeVisible = entries[0].isIntersecting; update(); }, { rootMargin: '0px 0px -110px 0px', threshold: 0 }).observe(scope);
    window.addEventListener('scroll', update, { passive: true });
    update();
  });
}
document.addEventListener('DOMContentLoaded', function() { initStickySave(); });

// ── htmx integration ───────────────────────────────────────────────────────
// Open modals when htmx swaps content into them
document.addEventListener('htmx:after:swap', function(e) {
  var ctx = e.detail && e.detail.ctx;
  var target = ctx && ctx.target;
  if (target && target.id === 'modal-body') { openModal(); enhanceComboInputs(target); }
});
// Close modal after successful form save (triggered by HX-Trigger: closeModal)
document.addEventListener('closeModal', closeModal);

// ── CSRF ──────────────────────────────────────────────────────────────────
function getCsrfToken() {
  var m = document.cookie.match(/(?:^|;\s*)plink_csrf=([^;]*)/);
  return m ? m[1] : '';
}
// Include token in all htmx requests
document.addEventListener('htmx:config:request', function(e) {
  var ctx = e.detail && e.detail.ctx;
  var token = getCsrfToken();
  if (ctx && ctx.request && ctx.request.headers && token) ctx.request.headers['X-CSRF-Token'] = token;
});
// Protect traditional mutation forms without adding tokens to search URLs.
document.addEventListener('DOMContentLoaded', function() {
  var token = getCsrfToken();
  if (!token) return;
  document.querySelectorAll('form').forEach(function(f) {
    if (f.method.toLowerCase() === 'get') return;
    var input = document.createElement('input');
    input.type = 'hidden';
    input.name = 'csrf_token';
    input.value = token;
    f.appendChild(input);
  });
});

// ── Delegated handlers (formerly inline event attributes) ──────────────────
document.addEventListener('click', function(event) {
  var el = event.target.closest ? event.target.closest('[data-copy-link],[data-copy-url],[data-share-url],[data-modal-close]') : null;
  if (!el) return;
  if (el.hasAttribute('data-copy-link')) { copyLink(el); return; }
  if (el.hasAttribute('data-copy-url')) { copyURL(el); return; }
  if (el.hasAttribute('data-share-url')) { shareURL(el); return; }
  if (el.hasAttribute('data-modal-close')) { closeModal(); return; }
});
// Export menu: the toggle flips the menu; any other document click closes it
// (previously stopPropagation on the toggle plus a close-everything listener).
document.addEventListener('click', function(event) {
  var menu = document.getElementById('export-menu');
  if (!menu) return;
  if (event.target.closest && event.target.closest('[data-export-toggle]')) {
    menu.classList.toggle('hidden');
    return;
  }
  menu.classList.add('hidden');
});
// Close the link modal when the overlay itself is clicked.
document.addEventListener('click', function(event) {
  var overlay = event.target.closest ? event.target.closest('[data-overlay-close]') : null;
  if (overlay && event.target === overlay) closeModal();
});
// Analytics period / breakdown / category filters navigate on change.
document.addEventListener('change', function(event) {
  var select = event.target.closest ? event.target.closest('[data-analytics-nav]') : null;
  if (!select) return;
  var mode = select.getAttribute('data-analytics-nav');
  if (mode === 'period') {
    var target = new window.URL(window.location.href);
    target.searchParams.set('period', select.value);
    window.location.assign(target.pathname + target.search);
  } else if (mode === 'group') {
    window.location.assign(select.dataset.baseUrl + (select.value && select.value !== 'link' ? '&group=' + encodeURIComponent(select.value) : ''));
  } else if (mode === 'category') {
    window.location.assign(select.dataset.baseUrl + (select.value ? '&category=' + encodeURIComponent(select.value) : ''));
  }
});
// Destructive forms ask for confirmation before submitting.
document.addEventListener('submit', function(event) {
  var form = event.target;
  if (!form.matches || !form.matches('form[data-confirm]')) return;
  if (!window.confirm(form.getAttribute('data-confirm'))) event.preventDefault();
});

// ── Guide table of contents (was inline in guide-dashboard.html) ──────────
(function () {
  var toggle = document.getElementById('guide-toc-toggle');
  var menu = document.getElementById('guide-toc-menu');
  if (!toggle || !menu) return;
  function closeMenu() { menu.hidden = true; toggle.setAttribute('aria-expanded', 'false'); }
  function openMenu() { menu.hidden = false; toggle.setAttribute('aria-expanded', 'true'); }
  toggle.addEventListener('click', function () { menu.hidden ? openMenu() : closeMenu(); });
  menu.addEventListener('click', function (event) {
    var link = event.target.closest('a');
    if (!link) return;
    var target = document.querySelector(link.getAttribute('href'));
    if (target) { event.preventDefault(); target.scrollIntoView({ behavior: 'smooth', block: 'start' }); }
    closeMenu();
  });
  document.addEventListener('keydown', function (event) { if (event.key === 'Escape') closeMenu(); });
  document.addEventListener('click', function (event) {
    if (menu.hidden) return;
    if (event.target.closest('#guide-toc-menu') || event.target.closest('#guide-toc-toggle')) return;
    closeMenu();
  });
})();

// ── Offer form (was inline in offers-dashboard.html) ────────────────────────
(function () {
  var itemType = document.getElementById('offer-item-type');
  var typeHelp = document.getElementById('offer-type-help');
  var productFields = document.getElementById('product-fields');
  if (itemType && typeHelp && productFields) {
    var typeDescriptions = {
      referral: 'Referrals link to the offer on the provider site.',
      product: 'Product cards can show details before opening the destination URL.',
      service: 'Services appear in the Services section and link to the destination URL.',
      resource: 'Resources appear in the Resources section and link to the destination URL.',
      article: 'Articles appear in the Reading section and open the article URL.',
      project: 'Projects appear in the Projects section and open the project URL.'
    };
    itemType.addEventListener('change', function() {
      typeHelp.textContent = typeDescriptions[itemType.value] || '';
      productFields.hidden = itemType.value !== 'product';
    });
    var offerForm = document.getElementById('offer-form');
    if (offerForm) offerForm.addEventListener('invalid', function(event) {
      var section = event.target.closest('details');
      if (section) section.open = true;
    }, true);
    // Present only on the new-item form.
    var reuseExistingSlug = document.getElementById('reuse-home-slug');
    var newHomepageURL = document.getElementById('home-url');
    var whatsappEnabled = document.getElementById('whatsapp-enabled');
    if (reuseExistingSlug && newHomepageURL && whatsappEnabled) {
      function updateHomepageURL() {
        newHomepageURL.disabled = reuseExistingSlug.checked;
        newHomepageURL.required = !reuseExistingSlug.checked && !(itemType.value === 'product' && whatsappEnabled.checked);
      }
      reuseExistingSlug.addEventListener('change', updateHomepageURL);
      whatsappEnabled.addEventListener('change', updateHomepageURL);
      itemType.addEventListener('change', updateHomepageURL);
      updateHomepageURL();
    }
  }
})();

// ── Settings page (was inline in settings-dashboard.html) ──────────────────
(function () {
  var mode = document.getElementById('feature-mode');
  var flags = document.getElementById('feature-flags');
  if (!mode || !flags) return;
  var boxes = Array.prototype.slice.call(flags.querySelectorAll('input[data-feature]'));
  var presets = {
    full: {feature_storefront: 1, feature_catalog: 1, feature_services: 1, feature_articles: 1, feature_projects: 1, feature_resources: 1, feature_profile: 1, feature_support: 1, feature_favorites: 1, feature_contact: 1},
    biolink: {feature_storefront: 1, feature_resources: 1, feature_profile: 1, feature_support: 1, feature_contact: 1},
    shortener: {}
  };
  function apply(name) {
    var preset = presets[name] || {};
    boxes.forEach(function (box) { box.checked = !!preset[box.name]; });
  }
  function syncLock() {
    var editable = mode.value === 'custom';
    boxes.forEach(function (box) { box.disabled = !editable; });
    flags.classList.toggle('feature-flags-locked', !editable);
  }
  mode.addEventListener('change', function () {
    if (mode.value !== 'custom') apply(mode.value);
    syncLock();
  });
  boxes.forEach(function (box) {
    box.addEventListener('change', function () { mode.value = 'custom'; syncLock(); });
  });
  syncLock();
})();
(function () {
  var rows = document.querySelector('[data-contact-rows]');
  var add = document.querySelector('[data-contact-add]');
  var template = document.getElementById('contact-row-template');
  if (!rows || !add || !template) return;
  function wire(row) {
    var remove = row.querySelector('[data-contact-remove]');
    if (remove) remove.addEventListener('click', function () { row.remove(); });
    var up = row.querySelector('[data-contact-up]');
    if (up) up.addEventListener('click', function () { var prev = row.previousElementSibling; if (prev) rows.insertBefore(row, prev); });
    var down = row.querySelector('[data-contact-down]');
    if (down) down.addEventListener('click', function () { var next = row.nextElementSibling; if (next) rows.insertBefore(next, row); });
  }
  rows.querySelectorAll('.contact-row').forEach(wire);
  add.addEventListener('click', function () {
    var row = document.createElement('div');
    row.className = 'contact-row';
    row.innerHTML = template.innerHTML;
    rows.appendChild(row);
    wire(row);
    var input = row.querySelector('input[name="contact_url"]');
    if (input) input.focus();
  });
})();

// Analytics chart bars: delegated date filter, no inline handlers (CSP-safe).
document.addEventListener('click', function (event) {
  if (!event.target || !event.target.closest) return;
  var day = event.target.closest('.chart-day[data-date]');
  if (day) selectReportDate(day.dataset.date);
});
document.addEventListener('keydown', function (event) {
  if (event.key !== 'Enter' && event.key !== ' ') return;
  if (!event.target || !event.target.closest) return;
  var day = event.target.closest('.chart-day[data-date]');
  if (day) { event.preventDefault(); selectReportDate(day.dataset.date); }
});

// ── Article editor draft protection ────────────────────────────────────────
// Lightweight local backup so a closed tab or a missed save does not lose
// writing. Active only on the article editor page (detected by the presence
// of #article-form). The Tiptap bundle (article-editor.js) is not touched:
// both scripts are deferred, so this file executes first and the bundle
// initializes from #article-body afterwards. The visual editor keeps its own
// copy of the text, so refreshing the textarea goes through the bundle's own
// markdown-source toggle (a same-task double click; nothing paints between
// the two clicks and the editor content is left unchanged).
(function () {
  var form = document.getElementById('article-form');
  if (!form) return;
  var FIELDS = ['title', 'slug', 'excerpt', 'body_md', 'cover_url', 'source_url', 'status', 'published_at'];
  var segments = (form.getAttribute('action') || '').split('/').filter(Boolean);
  var lastSegment = segments[segments.length - 1] || '';
  var key = 'plink-article-draft:' + (/^\d+$/.test(lastSegment) ? lastSegment : 'new');
  var bodyField = document.getElementById('article-body');
  var visual = form.querySelector('.article-editor-visual');
  var sourceToggle = form.querySelector('[data-command="source"]');
  var dirty = false;
  var submitted = false;
  var restoring = false;
  var saveTimer = null;

  function readBackup() {
    try { return JSON.parse(localStorage.getItem(key) || 'null'); } catch (e) { return null; }
  }
  function removeBackup() {
    try { localStorage.removeItem(key); } catch (e) { /* private mode */ }
  }
  function fieldValues() {
    var values = {};
    FIELDS.forEach(function (name) {
      var field = form.elements[name];
      if (field) values[name] = field.value;
    });
    return values;
  }
  function visualActive() {
    return !!(visual && !visual.hidden && bodyField && bodyField.hidden);
  }
  function editorFocused() {
    return !!(document.activeElement && visual && visual.contains(document.activeElement));
  }
  // Ask the bundle to write the visual editor's markdown into #article-body.
  function syncVisualBody() {
    if (!visualActive() || !sourceToggle) return;
    sourceToggle.click(); // visual → source: the bundle copies its markdown into the textarea
    sourceToggle.click(); // source → visual: the bundle re-reads the textarea
  }
  function persist() {
    try {
      localStorage.setItem(key, JSON.stringify({ t: Date.now(), f: fieldValues() }));
    } catch (e) { /* storage full or private mode */ }
  }
  function scheduleSave() {
    dirty = true;
    if (saveTimer) clearTimeout(saveTimer);
    saveTimer = setTimeout(function () {
      saveTimer = null;
      // Never disturb the caret mid-typing: while the visual editor holds
      // focus the textarea is snapshotted as-is (it may lag behind until the
      // next blur, submit, or unload sync).
      if (!editorFocused()) syncVisualBody();
      persist();
    }, 800);
  }
  form.addEventListener('input', function () {
    if (restoring) return;
    scheduleSave();
  });
  form.addEventListener('change', function () {
    if (restoring) return;
    // A chosen cover file cannot be persisted; the change still counts.
    scheduleSave();
  });
  window.addEventListener('beforeunload', function (event) {
    if (!dirty || submitted) return;
    syncVisualBody(); // the page is leaving; safe to refresh the textarea
    persist();
    event.preventDefault();
    event.returnValue = '';
  });
  form.addEventListener('submit', function (event) {
    submitted = true;
    if (event.submitter && event.submitter.name === 'preview') return; // preview reloads the editor; keep the backup
    // A real save navigates away. On validation errors the server re-renders
    // the submitted values into the form, so the backup is obsolete either way.
    removeBackup();
  });
  var deleteForm = document.querySelector('form[action$="/delete"]');
  if (deleteForm) deleteForm.addEventListener('submit', function () { removeBackup(); });
  // A successful save redirects back to the editor with ?saved=1; any backup
  // left over (for example from an interrupted preview) is obsolete there.
  if (/[?&]saved=1(?=&|$)/.test(window.location.search)) removeBackup();

  function restoreBackup(backup) {
    restoring = true;
    var wasVisual = visualActive();
    if (wasVisual) sourceToggle.click(); // switch to source so the textarea is the live field
    FIELDS.forEach(function (name) {
      var field = form.elements[name];
      if (!field || backup.f[name] === undefined) return;
      field.value = backup.f[name];
      field.dispatchEvent(new Event('input', { bubbles: true }));
      field.dispatchEvent(new Event('change', { bubbles: true }));
    });
    if (wasVisual) sourceToggle.click(); // back to visual: the bundle re-reads the restored markdown
    restoring = false;
    dirty = true; // the restored state is still unsaved work
  }

  var backup = readBackup();
  if (!backup || !backup.f) return;
  var current = fieldValues();
  var differs = FIELDS.some(function (name) {
    return backup.f[name] !== undefined && backup.f[name] !== current[name];
  });
  if (!differs) return;
  var when = 'an earlier session';
  try { when = new Date(backup.t).toLocaleString('en', { dateStyle: 'medium', timeStyle: 'short' }); } catch (e) { /* keep the fallback */ }
  var banner = document.createElement('div');
  banner.className = 'draft-banner';
  banner.setAttribute('role', 'status');
  var text = document.createElement('span');
  text.textContent = 'Unsaved draft from ' + when + '.';
  var restoreButton = document.createElement('button');
  restoreButton.type = 'button';
  restoreButton.className = 'btn btn-ghost';
  restoreButton.textContent = 'Restore';
  var discardButton = document.createElement('button');
  discardButton.type = 'button';
  discardButton.className = 'btn btn-ghost';
  discardButton.textContent = 'Discard';
  banner.appendChild(text);
  banner.appendChild(restoreButton);
  banner.appendChild(discardButton);
  form.parentNode.insertBefore(banner, form);
  restoreButton.addEventListener('click', function () {
    restoreBackup(backup);
    banner.remove();
  });
  discardButton.addEventListener('click', function () {
    removeBackup();
    banner.remove();
  });
})();

// ── Offer editor draft protection ──────────────────────────────────────────
// Same local backup pattern as the article editor, for the longer offer form
// (#offer-form). Every named control is kept except file inputs, which cannot
// be stored: an unfinished image upload has to be picked again by hand. A
// successful save redirects with ?saved=1 and drops the backup.
(function () {
  var form = document.getElementById('offer-form');
  if (!form) return;
  var segments = (form.getAttribute('action') || '').split('/').filter(Boolean);
  var lastSegment = segments[segments.length - 1] || '';
  var key = 'plink-offer-draft:' + (/^\d+$/.test(lastSegment) ? lastSegment : 'new');
  var SKIP = { file: 1, submit: 1, button: 1, reset: 1, hidden: 1 };
  var dirty = false;
  var submitted = false;
  var restoring = false;
  var saveTimer = null;

  // Checkboxes and radios store their checked state; everything else stores
  // its value. Disabled fields are skipped so a hidden section is not restored.
  function controls() {
    return Array.prototype.filter.call(form.elements, function (el) {
      return el.name && !el.disabled && !SKIP[el.type];
    });
  }
  function controlValue(el) {
    if (el.type === 'checkbox' || el.type === 'radio') return el.checked ? '1' : '';
    return el.value;
  }
  function readBackup() {
    try { return JSON.parse(localStorage.getItem(key) || 'null'); } catch (e) { return null; }
  }
  function removeBackup() {
    try { localStorage.removeItem(key); } catch (e) { /* private mode */ }
  }
  function persist() {
    var values = {};
    controls().forEach(function (el) { values[el.name] = controlValue(el); });
    try {
      localStorage.setItem(key, JSON.stringify({ t: Date.now(), f: values }));
    } catch (e) { /* storage full or private mode */ }
  }
  function scheduleSave() {
    dirty = true;
    if (saveTimer) clearTimeout(saveTimer);
    saveTimer = setTimeout(function () { saveTimer = null; persist(); }, 800);
  }
  form.addEventListener('input', function () {
    if (restoring) return;
    scheduleSave();
  });
  form.addEventListener('change', function () {
    if (restoring) return;
    scheduleSave(); // a chosen image file cannot be stored, but the change counts
  });
  window.addEventListener('beforeunload', function (event) {
    if (!dirty || submitted) return;
    persist();
    event.preventDefault();
    event.returnValue = '';
  });
  form.addEventListener('submit', function () {
    submitted = true;
    // A real save navigates away; on validation errors the server re-renders
    // the submitted values, so the backup is obsolete either way.
    removeBackup();
  });
  if (/[?&]saved=1(?=&|$)/.test(window.location.search)) removeBackup();

  function restoreBackup(backup) {
    restoring = true;
    controls().forEach(function (el) {
      if (!(el.name in backup.f)) return;
      if (el.type === 'checkbox' || el.type === 'radio') el.checked = backup.f[el.name] === '1';
      else el.value = backup.f[el.name];
      el.dispatchEvent(new Event('input', { bubbles: true }));
      el.dispatchEvent(new Event('change', { bubbles: true }));
    });
    restoring = false;
    dirty = true; // the restored state is still unsaved work
  }

  var backup = readBackup();
  if (!backup || !backup.f) return;
  var differs = controls().some(function (el) {
    return el.name in backup.f && controlValue(el) !== backup.f[el.name];
  });
  if (!differs) return;
  var when = 'an earlier session';
  try { when = new Date(backup.t).toLocaleString('en', { dateStyle: 'medium', timeStyle: 'short' }); } catch (e) { /* keep the fallback */ }
  var banner = document.createElement('div');
  banner.className = 'draft-banner';
  banner.setAttribute('role', 'status');
  var text = document.createElement('span');
  text.textContent = 'Unsaved draft from ' + when + '.';
  var restoreButton = document.createElement('button');
  restoreButton.type = 'button';
  restoreButton.className = 'btn btn-ghost';
  restoreButton.textContent = 'Restore';
  var discardButton = document.createElement('button');
  discardButton.type = 'button';
  discardButton.className = 'btn btn-ghost';
  discardButton.textContent = 'Discard';
  banner.appendChild(text);
  banner.appendChild(restoreButton);
  banner.appendChild(discardButton);
  form.parentNode.insertBefore(banner, form);
  restoreButton.addEventListener('click', function () {
    restoreBackup(backup);
    banner.remove();
  });
  discardButton.addEventListener('click', function () {
    removeBackup();
    banner.remove();
  });
})();
