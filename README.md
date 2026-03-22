# plink

Self-hosted and production-ready.

Self-hosted personal link shortener. No prefix, no bloat — `yourdomain.com/my-link` goes straight to the destination.

```
yourdomain.com/shopee-referral  →  https://shopee.com/...?ref=xxx
yourdomain.com/tokopedia-promo  →  https://tokopedia.com/...
```

## Features

- Clean slugs — no `s/` prefix
- Click analytics — total clicks, last 30 days chart, referrer breakdown
- Categories — organize links with filterable labels
- Instant search — filter by slug, URL, description, or category
- Simple admin UI — add, edit, delete links
- Single binary — no runtime, no Docker required
- Self-hosted — your data stays on your server

## Stack

- **Go** — standard library HTTP server
- **SQLite** — one file database (`modernc.org/sqlite`, no CGo)
- **Vanilla HTML/JS** — no npm, no build step

## Quick start

**Requires Go 1.22+** (built and tested with Go 1.26)

```bash
git clone https://github.com/srmdn/plink
cd plink
cp .env.example .env
# edit .env — set ADMIN_PASSWORD (required, no default)
go run ./cmd
# open http://localhost:8080/<ADMIN_PATH>/login  (default: http://localhost:8080/admin/login)
```

## Configuration

Copy `.env.example` to `.env` and edit:

| Variable         | Default          | Description               |
|------------------|------------------|---------------------------|
| `ADDR`           | `:8080`          | Listen address            |
| `DB_PATH`        | `plink.db`       | SQLite database file path |
| `ADMIN_PASSWORD` | (required)       | Admin password — **no default, must be set** |
| `APP_ENV`        | `development`    | Set to `production` to enable Secure cookie flag (required when running behind HTTPS) |
| `ADMIN_PATH`     | `admin`          | Admin URL path — set to something hard to guess for security by obscurity |
| `SITE_NAME`      | `plink`          | Site name shown on the public homepage |
| `SITE_DESC`      | `personal links` | Short description shown on the public homepage |

## Security

- **CSRF protection** — double-submit cookie token on all state-changing requests
- **Rate limiting** — login attempts capped at 5 per IP per 15 minutes
- **Secure cookies** — set `APP_ENV=production` to enable `Secure` flag (HTTPS only)
- **Security headers** — CSP, HSTS, X-Frame-Options, and more applied automatically
- **URL validation** — only `http`/`https` redirect targets accepted

## Build

```bash
go build -o plink ./cmd
./plink
```

## Deployment (VPS)

See [`deploy/`](deploy/) for:
- `plink.service` — systemd unit file
- `nginx.conf` — Nginx reverse proxy config

Basic setup:

```bash
# Build binary
go build -o plink ./cmd

# Copy to server
scp plink user@yourserver:/opt/plink/plink
scp .env.example user@yourserver:/opt/plink/.env
# edit /opt/plink/.env on the server

# Install and start service
sudo cp deploy/plink.service /etc/systemd/system/
sudo systemctl enable --now plink
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT
