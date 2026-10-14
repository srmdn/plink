# Changelog

All notable changes to Plink are documented in this file.

## Unreleased

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
