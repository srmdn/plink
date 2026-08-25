# Contributing to plink

Thanks for your interest! plink is a personal project that's open to contributions, but please read this first.

## Status

plink is stable and production-ready. The core feature set is complete. New features are considered but the project philosophy is to stay simple and single-binary.

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
# edit .env and set ADMIN_PASSWORD (required — app will not start without it)
go run ./cmd
```

Open the configured admin login path in a local browser and sign in with the password you set in `.env`.

## Project structure

```
plink/
├── cmd/            # Entry point (go run ./cmd)
├── internal/
│   ├── config/     # .env loading
│   ├── db/         # SQLite init, migrations, queries
│   └── server/     # HTTP handlers, auth, routing
├── web/            # Templates: home, dashboard, login + partials (embedded into binary)
└── plink.go        # package plink — embed + Run()
```

## Database migrations

Schema changes go in `internal/db/db.go` as new entries in the `migrations` slice. Each migration runs once and is tracked in the `_migrations` table. Never modify existing migrations — always add a new one.

## Code style

- Standard Go formatting (`gofmt`)
- No external dependencies unless absolutely necessary
- Keep templates in `web/templates/` as plain HTML — no build step, no npm

## Reporting security issues

Please do not open a public issue for security vulnerabilities. Use the
repository's private security reporting channel instead.

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
