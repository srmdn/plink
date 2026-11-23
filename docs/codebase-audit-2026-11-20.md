# Plink codebase audit — 20 November 2026

Read-only audit of the Plink checkout at commit `main` (in sync with `origin/main`
at audit time). No production system, database, or deployed instance was
touched. This document is an untracked working file; committing it is a
separate decision for the owner.

Method: full read of routing (`internal/server/server.go`), auth and CSRF
(`auth.go`, `csrf.go`), public handlers (`home.go`, `product.go`,
`redirect.go`, `offer_notice.go`, `seo.go`, `og_image.go`), admin handlers
(`ui.go`, `settings.go`), database layer (`internal/db/*`), upload pipeline
(`upload.go`), templates (`web/templates/**`), styles
(`web/js/css/storefront.css`, `admin.css`), inline scripts (`partials/head.html`),
and the design docs (`docs/ux-design.md`, `docs/analytics-backlog.md`).

## What already works well

- **Security fundamentals are solid.** CSP plus a full security-header set,
  `SameSite=Strict` cookies, constant-time password comparison, login rate
  limiting, double-submit CSRF, content-addressed upload names (hash +
  extension, no path escape), animated-image rejection, EXIF orientation,
  destination-URL validation, reserved-slug checks, and a hardened dev no-auth
  mode that rejects proxy headers.
- **Prices are integer minor units**, never floats (`product.go`).
- **Migrations are ordered and tested**; the root-level integration tests
  (24 files) give broad black-box coverage.
- **Static assets are content-versioned**; `/media` is served with immutable
  caching; OG images are generated server-side and cached.
- **The design docs exist and are honest** (`docs/ux-design.md` cites
  research, marks proposals vs. shipped behavior).

The findings below are not "rewrite everything" — they are specific,
verifiable defects and gaps, ranked by impact.

## Findings

### A. Reliability and correctness (fix first)

1. **CSRF rotation breaks multi-tab admin use.** `requireAuth` sets a fresh
   `plink_csrf` cookie on *every* authenticated GET (`auth.go:89-92`), while
   `partials/head.html:370-382` injects the token value read at page-load time
   into pre-rendered forms. Open the offer form in tab A, load any admin page
   in tab B, submit tab A → the form token no longer matches the rotated cookie
   → bare-text `403 forbidden` with no recovery UI. Long forms make this
   realistic and costly. Fix options: bind one token per session (rotate only
   on login), or accept the previous token within a grace window, and return a
   friendly "reload and retry" page instead of plain text.

2. **Articles are capped at 480 px on every screen.** `storefront.css:45`
   (`.store-header,.store-main,.store-footer { width:min(480px,100%) }`)
   binds the layout, while the intended article width at
   `storefront.css:978` (`.article-main { max-width:720px }`) is inert —
   `max-width` cannot loosen a smaller `width`. Long-form articles — the core
   content-business flow — render in a phone column on desktop, with small
   diagrams (this matches the earlier editorial note that detail text looked
   small at desktop article width). Fix: make `.article-main` override the
   width (e.g. `width:min(720px,100%)`), and consider a wider reading measure
   for `.article-body`.

3. **Sessions are in-memory only** (`auth.go:20-74`, a `map` with TTLs). Every
   restart or deploy logs the admin out mid-work; deploys have been frequent.
   Either persist sessions in SQLite (with the same TTL semantics) or accept
   and document the trade-off; a "stay signed in" option is a middle ground.

4. **Site settings are re-queried several times per render.** Handlers call
   `currentSiteSettings()` (a DB read), and the template funcs `publicProfile`
   and `branding` each call `database.GetSiteSettings()` directly
   (`server.go:108-130`), plus `/js/theme.css` is served with
   `Cache-Control: no-store` (`settings.go:386-398`), so a single public page
   triggers 3–5 identical settings queries and an uncached extra request. A
   single-process in-memory settings cache invalidated on save removes all of
   this.

### B. Product detail page (directly affects the FolioCMS listing)

5. **The detail URL is `/offers/item/{id}` with a numeric ID**
   (`server.go:229`, `product.go:148`). Not memorable, not brandable, exposes
   internal IDs, and is the canonical URL injected into SEO tags
   (`product.go:174`). Use the item's slug (e.g. `/offers/item/{slug}` or a
   `/p/{slug}` path) with a 301 from the ID form for compatibility.

6. **"Delivery or fulfillment" is hardcoded** on the detail page
   (`product-page.html:14`). For a digital good or a domain sale this heading
   is wrong; the FolioCMS escrow terms currently render under a shipping
   label. Make the heading product-kind-aware, or let the owner set a section
   label per item.

7. **The contact flow loses item context.** The detail-page CTA jumps to the
   homepage destination; the floating contact bubble opens a bare
   `mailto:mail@srmdn.com` with no subject. A buyer must explain which item
   they mean. Prefill the mailto subject with the item title, or add an
   item-aware contact action on the detail page.

8. **No structured data anywhere.** Verified: templates emit OG/Twitter meta
   only; there is no JSON-LD (`grep ld+json schema.org web/templates` →
   empty). Add `Product`/`Offer` schema on detail pages and `Article` schema
   on article pages.

9. **Notice vocabulary is too thin for real lifecycles.** Statuses are
   active/paused/ended; a "closed because an escrow opened" state cannot be
   expressed, and the notice page shows generic wording ("View active
   offers") with a hardcoded light background (`offer-notice.html:13`,
   `background:#fff`). Custom `notice_message` does render, but headings and
   status labels do not adapt. Add status-specific default wording (or a
   custom heading field) and remove the hardcoded color.

### C. Storefront design

10. **Desktop is a 480 px phone frame everywhere** — single-column grids,
    fixed bottom navigation, tiny mobile patterns. This is a deliberate
    mobile-first choice, but at ≥768 px the storefront could use a two/three
    column catalog grid, a header nav instead of the bottom bar, and the
    wider article measure from finding 2. The bottom nav is also present on
    article pages, where it competes with reading.

11. **No dark mode.** `:root { color-scheme:light }` and the notice template
    hardcode white; the operator can pick custom accent/background colors but
    there is no `prefers-color-scheme` support and no contrast validation on
    save (user-chosen colors can fail WCAG contrast silently).

12. **Cards without images fall back to a generic mark.** For a US$300
    listing this weakens perceived quality (already noted in the listing
    review). Platform-side: consider a typographic auto-mark built from the
    item title as the default placeholder.

### D. Analytics

13. **Internal analytics are slug-click-centric only.** Recorded events:
    redirect clicks, notice views, WhatsApp clicks. Catalog impressions,
    detail-page visits (explicitly uncounted), and article views are all
    invisible. The optional external script hook
    (`ANALYTICS_SCRIPT_URL`, Cloudflare Insights allow-listing in the CSP)
    exists but is disabled by default, so today the funnel for a public
    listing is blind between "card seen" and "contact clicked". Options:
    enable the external, privacy-friendly script on the public storefront;
    or record a `detail_views` event per product page the same way
    `RecordNoticeView` works — either closes the gap that the FolioCMS
    listing review flagged as its top "belum dicek" item.

14. **Analytics dashboards recompute everything per request** over all links
    (`ui.go`, `queries.go`). Fine at current scale; revisit with caching if
    link count or traffic grows. The `ogCache` map is unbounded
    (`og_image.go:185-188`); cap it.

### E. Code organization and admin hygiene

15. **`ui.go` is a 2,011-line file** holding dashboard building, analytics
    aggregation, server-side SVG chart string builders, CRUD handlers, and
    formatting helpers. Split by responsibility (dashboard, analytics, charts,
    link CRUD). The server-side SVG builders in Go strings are the least
    maintainable part — isolate them in one file with tests, or move chart
    rendering client-side (the storefront JS is tiny; there is room).

16. **~380 lines of inline JavaScript live in `partials/head.html`**, and
    `offer-notice.html` inlines a `<style>` block. This forces
    `script-src 'unsafe-inline'` and `style-src 'unsafe-inline'` in the CSP
    (`server.go:38-51`). Extract to versioned assets (the `asset` helper
    already exists), then tighten the CSP; inline styles can take nonces
    until fully moved.

17. **Admin is also a phone frame on desktop**: a fixed 448 px bottom nav
    with 10 px labels (`admin.css:1111-1146`) and narrow centered forms. For
    the operator's heaviest workflows (long article bodies, ten-section offer
    forms, analytics tables) a desktop layout — top or side nav, wide forms,
    multi-column tables — would materially improve productivity. This is the
    class of issue behind the earlier "bottom nav split into two rows"
    report.

18. **Offer form errors are form-level.** Validation failures re-render with
    input preserved (verified in `handleUpdateOffer`, `ui.go:1908-1916`) —
    good — but there is no field-level error marking, which the project's
    own UX doc calls for (`docs/ux-design.md`, "Validation must point out the
    fields to fix").

19. **Article editor gaps (known, confirmed in code):** no autosave; body
    media uploads are stored before the draft is saved
    (`upload.go:175-195`), so canceled drafts leave orphan files with no
    cleanup job; there is no media library view to see or reuse uploaded
    images (content-hash dedupe already exists server-side).

20. **Small hygiene items.** CSS variable naming debt (`--green` holds the
    red accent, `--map` unused; `theme.css` duplicates accent into both
    `--bg` and `--paper`); local runtime artifacts (`plink.db`,
    `plink-pre-production-sync-20260814.db`) sit inside the repo directory
    (untracked, but the pre-production sync copy belongs outside the
    checkout); favorites are localStorage-only and the Favorites nav item
    shows even for first-time visitors with an empty list.

## Priority ranking

**P0 — reliability/correctness (small, high value):**
1 (CSRF multi-tab), 2 (article width), 3 (sessions decision), 4 (settings cache).

**P1 — revenue and listing UX (affects the FolioCMS listing directly):**
13 (page-view visibility), 6 (fulfillment heading), 7 (item-aware contact),
9 (notice wording), 5 (slug URL), 8 (JSON-LD).

**P2 — experience:**
10 (storefront desktop layout), 17 (admin desktop layout), 11 (dark mode +
contrast validation), 18 (field-level errors), 19 (autosave + upload GC),
16 (CSP tightening), 12 (default card mark).

**P3 — hygiene:**
15 (ui.go split), 14 (analytics caching, ogCache cap), 20 (naming, repo
hygiene, favorites), media library.

## Boundaries

- Nothing in this audit changes production. Every code change needs the
  local-first workflow (verify with `go test ./...` and a local build), and
  deploy needs explicit authorization per action, including the backup
  checkpoint the deploy procedure requires.
- Findings 6, 7, 9, and 13 also surfaced independently in the FolioCMS
  listing review (20 November, `content-business/analytics/`); they are the
  overlap between codebase health and the active listing workflow.
