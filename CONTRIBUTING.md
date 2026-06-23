# Contributing to Koffy

Thanks for helping improve Koffy.

## Development setup

1. Fork and clone the repository.
2. Copy `.env.example` to `.env` and configure a local Casdoor application.
3. Start dependencies and services with `docker compose -f docker-compose.local.yml up --build -d`.
4. Run `go test ./...` and `cd web && npm ci && npm run build` before opening a pull request.

## Pull requests

- Keep changes focused and follow existing patterns.
- Add or update tests for behavior changes.
- Update documentation and `.env.example` when configuration changes.
- Never commit credentials, personal data, `.env`, certificates, private keys, database dumps, or generated build artifacts.
- Describe user-visible changes in `CHANGELOG.md` under an Unreleased section.

## Database changes

The public v0.1.0 baseline uses `migrations/001_init.sql` for the complete schema and `migrations/002_seed_local.sql` only for local demo data. Until a release after v0.1.0 requires an upgrade path, keep the baseline schema internally consistent and test it against an empty MySQL 8.4 database.

## Conduct

Participation is governed by [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).
