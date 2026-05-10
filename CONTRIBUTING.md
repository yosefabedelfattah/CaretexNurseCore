# Contributing

## Workflow

- Trunk-based development. Short-lived feature branches off `main`.
- Conventional Commits: `feat:`, `fix:`, `chore:`, `refactor:`, `test:`, `docs:`, `perf:`, `build:`, `ci:`.
- Squash-merge to `main`. Required: linked issue, passing CI, code review.

## Layer rules

- **Handlers** depend on Services and DTOs. Never on repositories or GORM.
- **Services** depend on Repositories, Integrations, Models. Never on Gin/HTTP.
- **Repositories** are the only place GORM is used.
- **Models** import nothing else from `internal`.

## Testing

```bash
make test         # unit + race
go test ./internal/services/... -v
```

Aim for ≥80% coverage in `internal/services`. Repositories should be covered by
integration tests against a real PostgreSQL (use testcontainers in CI).

## Database changes

- Add a new migration pair under `migrations/` named `NNNN_description.up.sql` /
  `.down.sql` (sequentially numbered).
- Mirror the schema in the GORM models.
- Do not edit existing migrations once they have been applied to any environment.

## Security

- Never commit `.env` or any secret. Use a secret manager in production.
- All inputs must be validated server-side via DTO validation tags.
- Use parameterized queries via GORM. Avoid raw SQL unless absolutely necessary.
- Report vulnerabilities privately to the security team before disclosure.
