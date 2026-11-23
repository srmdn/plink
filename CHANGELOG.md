# Changelog

All notable changes to Plink are documented in this file.

## v1.6.0

### Added

- Admin sessions persist in SQLite, so restarts and deploys no longer sign the
  operator out. Only token hashes are stored, and the session lifetime remains
  24 hours.
- Product detail pages record page views, shown as an all-time count in the
  product form and kept separate from slug clicks and WhatsApp button clicks.
- Product detail pages use the item slug at `/offers/item/<slug>`. The previous
  numeric URL answers with a permanent redirect to the slug, including the
  WhatsApp order endpoint.
- Product detail pages and native articles include JSON-LD structured data: a
  Product block with an optional price offer, and an Article block.
- An admin media library at `/<admin>/media` lists stored uploads with size,
  modification time, reference status, and a copy-URL action.
- A secondary contact action on product detail pages uses the first configured
  contact channel and prefills the item title in the mail subject or WhatsApp
  message.
- The storefront and admin follow the system dark-mode preference while
  keeping the operator accent color.
- The article editor keeps a local backup of unsaved drafts and offers to
  restore or discard it after a reload.
- The offer editor keeps the same local backup, so a long item form survives an
  accidental reload.
- Uploaded files that are no longer referenced are removed once they are older
  than 48 hours; the sweep runs daily.
- The media library is reachable from Catalog as well as Articles, next to the
  existing add actions.
- The admin link list pages 25 links at a time instead of rendering the whole
  catalog, and keeps the current page after toggling or deleting a link.

### Changed

- The CSRF token is bound to the admin session and stays valid across admin
  tabs instead of rotating on every page load.
- Product detail headings adapt to the product kind: digital products show
  "Delivery or access" instead of "Delivery or fulfillment".
- Offer form validation marks the fields that need correction in addition to
  the form-level message.
- Custom theme colors below a 4.5:1 WCAG contrast ratio produce a non-blocking
  warning after saving.
- The storefront widens to 720 px and the admin content area to 1180 px on
  desktop; the mobile frame and bottom navigation keep their current behavior.
- From 1024 px the admin navigation becomes a sidebar, so links, articles,
  analytics, and settings no longer require travelling to the bottom-centre of
  a wide screen. Below 1024 px the pill navigation is unchanged.
- Article dates read as "Nov 20, 2026" on the public article page and in the
  admin articles table instead of a raw timestamp.
- Catalog visibility uses one pair of terms, Visible and Hidden, in the filter,
  the counter, and each row.
- A product detail page without an image shows the typographic title mark
  instead of the Plink logo.
- Cards without an image use a typographic mark derived from the item title
  instead of a generic placeholder.
- The storefront and admin share one radius, border, and shadow scale: bands,
  tiles, and wells use named tokens, static tiles carry hairline borders, and
  shadows are limited to hover and floating chrome.
- Projects and Articles open with one lead card and list the remaining items as
  compact rows. The section eyebrow, the card mark, and the "view all" link use
  ink, leaving the accent for action labels.
- From 1024 px the admin Articles and Catalog lists read as column tables with
  a header row; below that they keep the single-column cards.
- An item saved without a button label uses an action that matches its type:
  View product, View service, Open link, Read article, or View project. Older
  items keep the label they already have.
- Inline admin scripts moved to a versioned `admin.js` asset, and the inline
  analytics chart handler moved to delegated listeners. The
  Content-Security-Policy now allows scripts only from the site itself or with
  the per-response nonce, with no `unsafe-inline` for scripts.

### Fixed

- Forms in other admin tabs no longer fail with a bare 403 after another tab
  loads an admin page.
- The offer notice page uses theme variables instead of a hardcoded white
  background.
- The navigation shadow is a named token and turns black in dark mode instead
  of glowing around the pill.

### Upgrade notes

- Schema v24 adds the `sessions` and `product_views` tables. Back up the
  database and uploads, and verify the backup, before upgrading.
- Sessions created before the upgrade are not carried over; sign in once after
  upgrading. Later restarts and deploys keep the session.
- Older binaries only understand the numeric product detail URL; slug URLs do
  not resolve until an updated binary is running. Consider this before rolling
  back a release that has published slug URLs.

## v1.5.0

### Added

- Write native articles in Plink with a visual editor or Markdown, private
  drafts, previews, covers, and published pages at `/a/<slug>`. Published
  native articles also appear alongside catalog articles.
- Added a project website and an English analytics screenshot using sample data.
- Local development can opt out of admin login on a loopback-only listener;
  normal deployments keep authentication and CSRF protection.
- Standalone links can be hidden from public listings while remaining active
  and accessible to anyone with the URL.
- Product cards can opt into a detail page with a structured optional price,
  product type, additional description, and delivery information. Existing
  products and short links keep their current redirect behavior.
- Product detail pages can offer WhatsApp ordering with a prefilled, editable
  message and a separate click count; existing short-link destinations remain
  unchanged.

### Changed

- The project is now licensed under AGPL-3.0-only; previously released MIT
  versions remain available under their original terms. Third-party licenses
  continue to apply.
- Translated the storefront and admin interface to English and updated the
  README and in-app guide.
- Updated `modernc.org/sqlite` from 1.60.1 to 1.62.0 and
  `modernc.org/libc` from 1.77.1 to 1.80.1.
- The catalog dashboard shows visibility separately from item status, offers a
  visibility filter, and clarifies that its counts follow the current filters.
- Catalog search includes channel slugs and retains filters after changing an
  item's visibility. The editor explains when a hidden slug becomes public.
- The public navigation calls the item list Catalog, while the admin explains
  the difference between short links and catalog cards. The item editor shows
  type-specific guidance and groups program settings for new items.

### Fixed

- Saving a catalog item no longer overwrites the description, category, or
  provider on its attached links.
- Period analytics include clicks recorded during the current second.
- Click-share percentages match between the chart, its tooltips, and rankings.

### Upgrade notes

- This release adds database migrations through schema v22 for unlisted links,
  product pages, WhatsApp ordering, and native articles. Back up the database
  and uploads before upgrading and verify the backup.
- Older versions do not understand some newer settings and content. In
  particular, unlisted links may reappear in public listings after a rollback.
  See the README for rollback precautions.

## v1.4.2

### Changed

- Updated `modernc.org/sqlite` from 1.50.1 to 1.60.1, with
  `modernc.org/libc` 1.72.3 → 1.77.1 and `modernc.org/memory` 1.11.0 → 1.12.1.
  This includes WAL shared-memory fault handling, VFS/page-cache safety fixes,
  and faster multi-row parameter binding. No database schema change.

## v1.4.1

### Security

- Rebuilt with Go 1.27.3 to address GO-2026-6809 in `crypto/x509`
  (distrust compromised intermediate certificates).

## v1.4.0

### Security

- Upgraded `golang.org/x/image` to remove reachable decoder vulnerabilities.
- Admin and API responses now use `Cache-Control: no-store`.
- Bounded stored referrer and user-agent values, and added HTTP server timeouts
  and a 64 KiB header cap.
- Proxy headers are trusted only from loopback, and short admin passwords now
  produce a startup warning.
- Removed the admin path from `robots.txt` and ignored the default `uploads/`
  directory in Git.

### Changed

- Updated the vendored htmx runtime from 2.0.4 to 4.0.0, including the renamed
  events, error-response swap behavior, and pending-state handling.
- Moved the base admin styles into the versioned stylesheet, cutting the admin
  document size and removing dead dark-theme rules.
- Improved mobile ergonomics: larger touch targets, section-specific empty
  states, and an edge fade on the featured scroller.
- Added sticky save actions to Settings and the offer editor, and replaced
  native file inputs with a styled picker.
- The floating contact button now hides while scrolling down and returns on
  scroll up; the guide table of contents moved into the guide header.
- Clarified admin counts and table labels, and styled color inputs as swatches.
- The public affiliate fallback now uses the formal register.

### Fixed

- Create-link validation errors stay in create mode instead of switching to an
  edit form.
- Direct visits to analytics fragment endpoints redirect to the dashboard.
- The guide TOC no longer overlaps cards, and offer notice pages now load the
  contact-button script.

## v1.3.0

### Added

- Redesigned public storefront with a biolink-style profile header, mobile
  navigation, and configurable branding, theme palettes, and public origin.
- Offers storefront with grouped tracking links, program lifecycle states
  (active, paused, ended), end-of-program notices, and separate notice
  analytics.
- Blog ("Bacaan") article catalog type, project listing, and favorites for the
  public catalog.
- Configurable floating contact button with reorderable contact channels.
- Compact sticky profile bar when the biolink hero is scrolled out.
- Catalog image uploads, plus logo, favicon, and share image uploads in
  Settings, with a brand mark fallback for cards without an image.
- SEO settings with server-rendered metadata and generated social share cards.
- Catalog item previews generated on existing short links, served to social
  crawlers without changing visitor redirects, with a Settings toggle.
- Public feature toggles in Settings to show or hide storefront sections and
  pages, with preset modes.
- Configurable visitor analytics through `ANALYTICS_SCRIPT_URL` and
  `ANALYTICS_WEBSITE_ID`; optional Cloudflare Web Analytics support through
  `CLOUDFLARE_INSIGHTS`.
- Optional upload directory override through `UPLOADS_DIR`.
- Floating table of contents on the in-app guide.
- Content-versioned static asset URLs that bypass stale edge caches after a
  deploy.

### Changed

- Redesigned admin dashboard with clearer analytics rankings and separated link
  analytics panels.
- Catalog item links open in a new tab, and the public header shows a dashboard
  link only to signed-in admins.
- Contact-channel links are excluded from public resource listings while still
  redirecting and counting clicks.
- Public copy uses a clear, formal register with deployment-neutral defaults
  that remain editable in Settings.

### Fixed

- Featured offers no longer appear in the homepage grid.
- Article and project cards are excluded from the favorites page, and the
  Proyek navigation entry has its own icon.
- Feature checkboxes stay locked to the selected preset mode.
- Native date input icons are readable in the light admin theme.
- Replaced native datalist dropdowns with a reliable combobox.
- Corrected the CSP directive separator.
- Improved admin responsive behavior on smaller screens.

### Removed

- Removed orphaned analytics modal scripts, dead markup, and unused dark
  analytics styles.

## v1.2.0

### Added

- Configurable analytics timezone through `APP_TIMEZONE`, with `UTC` as the
  default.
- Date-filtered dashboard reporting and a daily click CSV export.
- Grouped referrer sources with expandable raw referrer details.
- Date-aware traffic source reports for individual links and the dashboard
  overview.
- Clickable days in per-link analytics charts to set the dashboard report date.

### Changed

- Daily analytics charts, date filters, exports, and last-click displays use
  the configured reporting timezone.
- Clarified the export menu so link catalog exports are separate from daily
  click reports.
- Dashboard filters now persist in the main dashboard URL.

## v1.1.0

### Added

- A curated public homepage with recommended links and an expandable catalog.
- Public resource search across slugs, descriptions, and categories.
- Dashboard summary metrics for total links and clicks.
- A compact dashboard table for faster link management on desktop.
- An open-source footer and refreshed favicon.

### Changed

- Improved category filtering so filtered results open at the resource section.
- Improved the mobile layout for public links and dashboard controls.
