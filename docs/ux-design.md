# Plink UX design: links, catalog, and mini-shop

Status: **proposal for review**. Parts are implemented; see Implementation
status below.

Plink can still be used as a link shortener, a recommendation catalog, a
mini-shop with manual ordering, or a portfolio. The owner chooses the features
they need; visitors see navigation and actions that match the content of the
site.

## Implementation status

Shipped in the 10 November 2026 release:

- product detail pages with structured price, product kind, product details,
  and fulfillment notes;
- optional WhatsApp ordering per product, with click tracking kept separate
  from orders;
- unlisted standalone links;
- item type guidance in the admin guide and the English public/admin interface.

Still proposals, not behavior: configurable navigation labels and order,
structured variants, multiple images, cart, and transaction integrations.

Implemented in the current development tree, not yet released:

- the shared radius, border, and shadow token scale described under **Visual
  system**;
- system dark mode with the operator accent retained and a non-blocking
  contrast warning on save;
- a wider desktop layout for the storefront and admin while the mobile frame
  and bottom navigation keep their current behavior;
- a desktop admin shell: from 1024 px the same navigation moves into a sidebar
  so the operator does not travel to the bottom-centre of a wide screen. Below
  1024 px the pill navigation is unchanged;
- product detail pages at slug URLs with a permanent redirect from the numeric
  form, separate page-view counts, and item-aware contact and fulfillment copy;
- a media library for stored uploads, with unreferenced files cleaned up after
  48 hours;
- field-level validation errors on the offer form.

## Visual system

Status: **implemented in the current development tree, not yet released.** This
section records the token scale and the rules that go with it, so later changes
extend the system instead of adding one-off values.

Both `web/js/css/storefront.css` and `web/js/css/admin.css` use the same named
tokens. The storefront and admin read as one product because radius, border,
background, and shadow are decided together.

The rule in one sentence: the canvas is paper; a **band** runs edge to edge and
is rounded on its free corners; a **tile** sits on the canvas with a hairline
border and a radius from the scale; a **well** sits inside a tile on a deeper
tone; **floating chrome** uses the largest radius step plus blur and shadow.
There is no rounded element without a hairline, and no static tile with a
shadow.

### Radius scale

| Token | Value | Role | Examples |
| --- | --- | --- | --- |
| `--radius-xs` | 6px | micro, nested | inline code, combo options, contact-order buttons, color swatches, small meta badges |
| `--radius-sm` | 10px | controls and rows | inputs, selects, admin buttons, search and storefront buttons, table rows, thumbnails, inset image wells, product actions, notice actions, status banners, file pickers, draft banner |
| `--radius-md` | 14px | cards and panels | offer cards, storefront sections, admin headings, KPI cards, analytics panels, form sections, guide sections, login box, menus, article modal, share preview, cover images, empty states |
| `--radius-lg` | 18px | floating sheets | modals, toasts in both surfaces, contact bubble, sticky save bar |
| `--radius-xl` | 24px | floating navigation band | bottom navigation in both surfaces; nav items use `calc(var(--radius-xl) - 7px)` to account for the 7px padding |
| `--radius-pill` | 999px | pills | category chips, support buttons, offer badges, meta and media badges, status chips, progress bars |
| `--radius-circle` | 50% | circles | avatars, contact FAB, favorite button, social icons, donut dots, touch-target halos |

Band corners use `border-radius: 0 0 <step> <step>`: the thin topbar uses
`sm`, the profile hero and compact profile bar use `sm`, and the tall hero uses
`md`. The older admin `--radius: 12px` is retired to an alias of
`var(--radius-md)`, so legacy rules follow the scale instead of keeping a free
value.

### Hierarchy, borders, and shadows

The tone hierarchy is unchanged but explicit:

- Storefront: `--paper` canvas → `--surface` tile → `--soft` well.
- Admin: `--bg` canvas → `--surface` tile → `--surface2` well.
- Cross-surface pairs: `paper` ↔ `bg`, `surface` ↔ `surface`, `soft` ↔
  `surface2`.

All static tiles carry a `1px` hairline border from `--line` (admin:
`--border`, an alias). This includes offer cards, storefront sections, admin
headings, KPI cards, panels, table rows, chips, resource cards, empty states,
and notice panels.

Shadows are functional tokens, not decoration:

- `--shadow-hover` lifts a card slightly on hover; in the light theme it is
  derived from the ink color.
- `--shadow-float` is reserved for floating chrome such as modals, toasts, the
  contact bubble, and the sticky save bar.
- Static tiles have no shadow. Non-interactive hover effects that only faked
  elevation were removed; for example, admin KPI cards now strengthen their
  border instead of translating upward.

The half-rounded stat strip was removed. Every KPI card is a complete tile at
`md`, and the stats container is transparent with a gap instead of fusing
heading, stats, and toolbar into one full-bleed workspace.

### Dark mode

The dark palette stays owned by `/js/theme.css` (paper `#161a19`, surface
`#202623`, line `#39413d`, ink `#e9ece9`, muted `#9aa49e`) and is not
redefined in the stylesheets. The stylesheets only override the shadow tokens in
their dark blocks: ink-based shadows would become light glows on a dark surface,
so floating shadows turn solid black there. Hairlines continue to use `--line`,
and the tone hierarchy remains readable because the dark theme defines `soft`
and `surface2` as mixes of surface and paper/background.

### Scope

The mobile frame is unchanged: the storefront breakpoint stays below 768px and
the admin breakpoint below 1024px, with the same 480px storefront width and
bottom navigation. From 1024px the admin navigation becomes a sidebar, but the
pill navigation below that breakpoint and the storefront navigation at every
size stay as they are. The change is the shared token language and these
desktop rules, not a new responsive structure.

## Current condition

- The admin has **Links**, **Catalog**, **Articles**, **Analytics**, and
  **Settings**, plus a **Media** entry point from both Articles and Catalog.
- The catalog stores product, referral, service, resource, article, and project
  items. Services and projects already have their own public sections.
- Product and referral cards point to a short slug, then to the destination.
  A product with a detail page enabled goes to that page first. Project and
  article cards point directly at the destination URL.
- A WhatsApp link can be used as a destination, but price, product details,
  variant choices, and an order message builder were not available as
  dedicated features before the release above.
- Analytics records redirect clicks and product detail-page views; it does not
  measure orders, payments, or commission. Favorites are stored in the
  visitor's browser.
- Public features can be turned on and off in Settings. Unlisted standalone
  links are available.

## Decision principles

1. **Separate visitor intent.** People shopping, hiring a service, or viewing a
   portfolio can reach the matching section.
2. **Unify management without unifying every view.** One catalog item keeps a
   type and can appear in the matching public section.
3. **Explain what a button does.** Visitors know whether it opens WhatsApp, the
   provider site, product details, or a project site.
4. **Show information that fits the item type.** Physical products, digital
   products, referrals, services, and projects are not forced into the same
   fields.
5. **Keep existing behavior by default.** The owner decides whether an existing
   item uses the new flow; existing slugs and destinations do not change.

## Public navigation

Default proposal for a site with every section enabled:

```text
Home   Catalog   Services   Projects   Favorites
```

**Catalog** replaces the **Categories** label because that page holds items,
search, and filters. Important categories stay visible on the home page or at
the top of the catalog, for example **VPS**, **Digital products**, and
**Accessories**. Category names and order come from the owner's data, not from
Plink.

Services and Projects remain separate pages. The owner can disable unused
features through the existing settings. For a simple mini-shop the layout can be
**Home · Catalog**, with contact easy to find. Mixed sites can re-enable
Services and Projects.

Renaming navigation labels can be an early change. Per-install navigation names
and order need their own design and testing; do not assume that feature exists.
Existing URLs are preserved when labels change.

Favorites stays optional and must explain that its contents are stored in that
browser. The floating contact button is optional, has a clear name, and must not
cover the bottom navigation, content, or the main action on small screens. When
the contact feature is disabled, the owner can still provide contact information
through the profile or available pages.

## Public page sketches

Mini-shop example with fictional data:

```text
HOME
[Shop logo]
Selected gear and guides
[Search products or services          ] [Search]

Categories: [All] [Digital] [Accessories] [VPS]

Featured
[Image]                   [Image]
Photography guide         Camera bag
Rp79,000                  Rp240,000
[View details]            [View details]

Questions? [Contact us]
-------------------------------------------------
Home                      Catalog
```

```text
DIGITAL PRODUCT DETAIL
‹ Back to catalog
[Guide image]
Photography guide
Rp79,000
PDF · delivered after payment is confirmed

Summary of contents and how the product is delivered
[Order via WhatsApp]
Orders and payment are confirmed in chat.
```

```text
PHYSICAL PRODUCT DETAIL
‹ Back to catalog
[Camera bag image]
Camera bag
Rp240,000
Color: black
Shipping cost confirmed at checkout

[Order via WhatsApp]
```

These examples are illustrations only. If a price is not filled in, do not show
`Rp0`; use a label such as **Ask for price** when the owner chooses it. Do not
state that stock is available or give a shipping estimate without data from the
owner. Provide fulfillment information and a return policy that fit the product
type.

## Visitor flow per scenario

| Scenario | Proposed flow | Notes |
| --- | --- | --- |
| Physical product | Catalog → product detail → Order via WhatsApp | Show the price when available, relevant variants, and shipping cost or how it is confirmed. |
| Digital product | Catalog → product detail → Order via WhatsApp | Explain format, contents, and how and when delivery happens after payment is confirmed. |
| Referral | Catalog → View offer → provider site | Show the provider identity and an appropriate affiliate disclosure. Do not equate a click with a purchase. |
| Service | Services → service detail or Contact → contact channel | Explain the service, price range when available, and the next step. |
| Project | Projects → project detail or project site | Separate a portfolio piece from a product that can be ordered. |

For WhatsApp, the opening message can include the product name, the detail URL,
and available choices. Visitors still review and send the message themselves.
Example: "Hi, I am interested in the camera bag. Is it still available?" Do not
include personal data or claim that an order was placed automatically.

Card buttons must match the action: **View details** for a product page,
**View offer** for a referral, and **View project** for a portfolio piece. The
floating contact is a general help path; the message button on a product carries
the context of the selected product. They do not replace each other.

## Admin design

Keep the five main menus, with descriptions on empty pages and through short
help text:

| Menu | Question it answers |
| --- | --- |
| **Links** | Which short URL did I share, and how many times was it clicked? |
| **Catalog** | Which cards, products, services, and projects do I show? |
| **Articles** | What long-form writing is published, drafted, or backed up? |
| **Analytics** | Which links, providers, or channels get clicks? |
| **Settings** | How are the site identity, contact, and public sections configured? |

Example relationship shown in the admin: **GreenCloud** is one catalog item;
`greencloud-blog` and `greencloud-instagram` are connected links that separate
click sources. A standalone link does not need an item.

### Add item

```text
Add item

What do you want to add?
[Product]        [Referral]
[Service]        [Project]
[Article]        [Resource / link]
```

Choosing a type opens a form with the main fields first:

- **Product:** name, image, description, physical/digital kind, optional price,
  and how to order. Variant and fulfillment settings follow the product kind.
- **Referral:** name, provider, image, description, and destination URL.
- **Service:** service name, description, optional price, and how to get in
  touch.
- **Project:** title, image, description, and project URL when available.
- **Article/resource:** name, description, and destination that fit the content
  type.

**Advanced** holds priority, dates, program status, channel, and campaign where
relevant. Existing values must stay visible and saved while the form is edited;
hiding the advanced section must not delete them. A product without an image,
price, or variants can still be created if the owner chooses that. Validation
must point out the fields to fix and keep the input.

### Readable statuses

| Label | Meaning |
| --- | --- |
| **Card hidden** | The item card does not appear in the catalog; link and program status still apply. |
| **Unlisted** | An active standalone link does not appear in the public list, but can be opened by URL. |
| **Link paused** | The short link returns 404. |
| **Program paused/ended** | Connected slugs follow the notice or the fallback destination rule. |

When an unlisted link is used as a card link, explain that the slug can appear
publicly if the card is shown. When changing program status, explain that every
connected slug may be affected. Do not use the word "private" for a link that
anyone can still open with its URL.

## Stages and boundaries

### Stage 1: clarity without changing existing flows

- Rename the public **Categories** label to **Catalog**; keep URLs.
- Test access to important categories from the home page and the catalog.
- Tidy the Links/Catalog help, the item type picker, and the status explanations.
- Reorganize the form gradually without deleting values or changing existing
  item button actions. Test phone, desktop, and keyboard.

### Stage 2: WhatsApp mini-shop

- Design and implement price, physical/digital kind, product details, and how
  to order; store price as a structured value with a currency, not ambiguous
  numeric text. An empty price differs from a price of zero.
- Add optional product details and a WhatsApp link builder with an opening
  message. Normalize the number to the international format; URL-encode the
  message text.
- Define stock, variants, shipping, and delivery as information the owner fills
  in; do not promise automatic calculation or fulfillment.
- Existing items keep their published destination and behavior until the owner
  chooses the new flow. The detail page must have its own URL that does not take
  over or break short slugs already shared.
- Keep **WhatsApp click** metrics separate from orders and payments. There is no
  payment system, order management, or automatic inventory at this stage.

### Stage 3: evaluate based on use

- Evaluate whether the owner needs navigation label/order settings, structured
  variants, multiple images, a cart, or transaction integrations.
- Do not add that complexity before the simple shop scenario has been tested.

Changes that need database columns or new routes require their own migration
design, backward compatibility, backup, and rollback testing before release.
This design does not grant permission to deploy or to change production data.

## Usability test plan

Use fictional data on a clean local installation. Test each scenario with at
least one person unfamiliar with Plink; early results are qualitative, not
statistical proof. Observe without pointing at menus.

1. **New owner:** create a short link, then explain the difference between
   active, unlisted, and paused links.
2. **Shop owner:** create a digital and a physical product, fill in prices, and
   set up WhatsApp ordering without assuming automatic payment.
3. **Shop visitor:** find a product from a category, understand the price and
   how it is delivered, then open a WhatsApp message with the right product.
4. **Affiliate owner:** create one item and two channel slugs; find per-channel
   analytics without treating clicks as purchases.
5. **Portfolio visitor:** find a project and tell it apart from a service.
6. **Accessibility:** use a keyboard and a screen reader; check control names,
   focus, contrast, touch size, and the bottom tabs and contact button on a
   phone.

Record successes, confusing steps, and recurring problems. Also check existing
items, existing slugs, disabled feature choices, narrow screens, long text,
empty images, and items without a price before approving implementation.

## Research basis and limits

- [Baymard: category navigation on mobile devices](https://baymard.com/research-articles/main-navigation-product-categories)
  shows the importance of easy-to-find categories; the research also notes that
  a single "Shop" entry point can work for a small catalog.
- [Baymard: shipping cost information on product pages](https://baymard.com/research-articles/show-shipping-costs-on-product-pages)
  shows why shipping information helps purchase decisions.
- [WhatsApp: click to chat](https://faq.whatsapp.com/5913398998672934)
  explains the international number format and URL-encoded opening messages.

That research supports design hypotheses; it is not proof that this design is
already usable by Plink users. Direct validation is still required.
