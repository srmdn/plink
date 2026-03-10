# Contributing to plink

Thanks for your interest! plink is a personal project that's open to contributions, but please read this first.

## Status

**plink is currently in active development and not production-ready.** The API, database schema, and configuration format may change without notice until a stable release is tagged.

## What kind of contributions are welcome?

- Bug reports and bug fixes
- Documentation improvements
- Security issues (see below)
- Feature suggestions via issues — discuss before building

## What to avoid

- Large refactors without prior discussion
- Adding external dependencies (npm, new Go modules) without a strong reason
- Features that conflict with the project's philosophy: simple, single binary, no bloat

## Getting started

```bash
git clone https://github.com/srmdn/plink
cd plink
cp .env.example .env
go run ./cmd
```

The admin UI is at `http://localhost:8080/admin`. Default password is `admin`.

## Project structure

```
plink/
├── cmd/            # Entry point (go run ./cmd)
├── internal/
│   ├── config/     # .env loading
│   ├── db/         # SQLite init, migrations, queries
│   └── server/     # HTTP handlers, auth, routing
├── web/            # Admin UI (single HTML file + favicon)
├── deploy/         # systemd + nginx examples
└── plink.go        # package plink — embed + Run()
```

## Database migrations

Schema changes go in `internal/db/db.go` as new entries in the `migrations` slice. Each migration runs once and is tracked in the `_migrations` table. Never modify existing migrations — always add a new one.

## Code style

- Standard Go formatting (`gofmt`)
- No external dependencies unless absolutely necessary
- Keep the admin UI as a single `web/admin.html` file — no build step, no npm

## Reporting security issues

Please do not open a public issue for security vulnerabilities. Email `mail@saidwp.com` instead.

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
