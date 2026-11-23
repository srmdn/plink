# Analytics backlog

This note records advanced options deliberately deferred from the current
analytics dashboard release. The core feature set stays focused on link
management, provider ranking, and reading performance by slug.

## Status

- [x] **Contextual trend** — the Trend column is hidden when the period filter
  is `all`, because there is no meaningful comparison period. Released in
  v1.3.0.
- [x] **Product detail-page views** — each product detail page visit is
  recorded separately from slug clicks and WhatsApp button clicks, and the
  product form shows an all-time count. Implemented after v1.5.0; not yet
  released. This is a visit count, not a view-through or conversion measure.
- [ ] **Bulk metadata edit** — change provider, channel, campaign, or category
  for several links at once.
- [ ] **Promotion strategy status** — add manual `push`, `observe`, and `pause`
  states so the ranking can become a promotion worklist directly.
- [ ] **Conversion and revenue tracking** — connect clicks with conversion,
  commission, and revenue so ranking is not based on traffic alone.

Contextual trend shipped in v1.3.0. Product detail-page views are implemented
after v1.5.0 and are not yet released. The three remaining items are not done
and need evaluation after the dashboard has been used with production data.
