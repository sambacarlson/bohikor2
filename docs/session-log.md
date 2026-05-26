# Session Log

## 2026-05-26 — Docker deploy fixes, EAS build, secrets audit

### What we did
- Fixed Render Docker deploy: set `dockerContext: ./backend` in `render.yaml`
- Fixed Firebase credentials temp file lifecycle in `backend/internal/firebaseapp/app.go`
- Replaced custom migration runner with `golang-migrate` in `backend/internal/database/migrate.go`
- Fixed `db/sqlc/` being gitignored (committed generated code so Docker build can find it)
- Fixed `migrations/` missing from Docker runtime stage (added `COPY --from=builder /app/migrations ./migrations/`)
- Ran EAS local Android build successfully (`build-1779550117750.aab`)
- Created Google Play Service Account and linked to EAS credentials
- Installed `golang-migrate/v4`, ran lint + tests
- Added pre-push checklist to `AGENTS.md`

### Key lessons
- Generated code (`db/sqlc/`) must be tracked in git — `.gitignore` shouldn't exclude build-time dependencies
- Multi-stage Docker builds need explicit `COPY` for runtime-accessed directories (migrations)
- Always run `docker build` locally before pushing Docker changes to Render
- Firebase Web API keys (`AIza...`) are public by design — GitHub secret scanning false positive
