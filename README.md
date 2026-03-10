# plink

Self-hosted personal link shortener. No prefix, no bloat — `yourdomain.com/my-link` goes straight to the destination.

```
yourdomain.com/shopee-referral  →  https://shopee.com/...?ref=xxx
yourdomain.com/tokopedia-promo  →  https://tokopedia.com/...
```

## Features

- Clean slugs — no `s/` prefix
- Click analytics — total clicks, last 30 days chart, referrer breakdown
- Simple admin UI — add, edit, delete links
- Single binary — no runtime, no Docker required
- Self-hosted — your data stays on your server

## Stack

- **Go** — standard library HTTP server
- **SQLite** — one file database (`modernc.org/sqlite`, no CGo)
- **Vanilla HTML/JS** — no npm, no build step

## Quick start

```bash
git clone https://github.com/srmdn/plink
cd plink
cp .env.example .env
# edit .env and set ADMIN_PASSWORD
go run ./cmd
# open http://localhost:8080/admin
```

## Configuration

Copy `.env.example` to `.env` and edit:

| Variable         | Default    | Description               |
|------------------|------------|---------------------------|
| `ADDR`           | `:8080`    | Listen address            |
| `DB_PATH`        | `plink.db` | SQLite database file path |
| `ADMIN_PASSWORD` | `admin`    | Admin password            |

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
# Copy binary
scp plink user@yourserver:/opt/plink/plink

# Copy and edit env
scp .env.example user@yourserver:/opt/plink/.env

# Install systemd service
sudo cp deploy/plink.service /etc/systemd/system/
sudo systemctl enable --now plink
```

## License

MIT
