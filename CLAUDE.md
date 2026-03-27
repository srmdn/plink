# CLAUDE.md

## Project
plink: self-hosted personal link shortener. Clean slugs, click analytics, categories.
Single Go binary with SQLite and embedded HTML/JS. No frontend build step.

## Stack
- Language: Go
- Database: SQLite (`modernc.org/sqlite`, pure Go)
- Frontend: vanilla HTML/JS/CSS embedded in the binary
- Auth: session-based

## Repo visibility: PRIVATE (closed source)

## Environment: STAGING
Staging: port 8086, `staging.klikgan.com`
Production: port 8087, `klikgan.com`

Both environments run from the `main` branch. No staging branch.

## Git
- `main` is the only branch
- Deploy to staging first, verify, then deploy to production

## Conventions
- Secrets in `.env`: never committed
- `.env.example` committed with all variable names, no real values
- Build output (binary) gitignored
- No Docker, no external runtime dependencies
- Keep commits small: one logical change per commit

## Testing
Run before every commit: `go test ./...` (from repo root)
All tests must pass before committing.
Write tests for new code in the same commit.

## Writing Conventions
- No em dashes (`—`) in commit messages, docs, or any written output.
- Use a colon, semicolon, or rewrite the sentence instead.

## Security Rules
- No hardcoded credentials or tokens in source code
- SQL queries must use parameterized statements
- Review AI-generated code for security issues before committing

## Do not modify without confirming
- Database migration files
- `.env.example` (only add keys, never remove)
