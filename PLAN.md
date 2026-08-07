# PLAN.md — Bohikor

Salary-advance platform. Employees request an advance; it is paid instantly via Campay
mobile money. Originally a single-company mobile app; **pivoted to a multi-company web
platform** with per-employer float and hardened payment resilience — the pivot (Epics 6–9)
is complete.

For current architecture, auth flows, and request/payout details, see `AGENTS.md` (canonical,
actively-maintained reference), `docs/brief.md` (product/domain overview), and `docs/schema.md`
(DDL). This file is now just a shipped-history record — the detailed epic-by-epic implementation
plans it used to carry have been removed now that they're done.

---

## Shipped (condensed history — see `docs/session-log.md` for detail)

- **Epic 1 — Auth:** invitations, email OTP, user/admin accounts, event log.
- **Epic 2 — Request & payout:** `advance_requests`, Campay Withdraw API, webhook, admin requests page.
- **Epic 2.5 — Own auth:** removed Firebase. Backend is sole auth authority — HS256 JWT access
  tokens (15m) + opaque rotating refresh tokens (30d), bcrypt.
- **Epic 3 — PIN auth:** email + 5-digit PIN login, PIN rate limiting + account lock, phone
  verification via Campay Collect (USSD) instead of SMS.
- **Epic 4 — Pilot controls:** global JSONB `settings`, kill switch, request window, daily/monthly
  throttling, eligibility endpoint, OTP rate limiting.
- **Epic 5 — Hardening:** USSD persistence, post-Campay DB-failure fallbacks, webhook dedup.
- **Epic 6 — Multi-tenant foundation (backend):** every domain table gained `company_id`; new
  `companies`/`platform_admins` tables; platform provisioning API; company-scoped JWT claim +
  middleware isolation guarantee.
- **Epic 7 — Payout reliability & float:** `company_ledger` (immutable, running-sum balance),
  float-gated payouts, `processing` status for ambiguous Campay outcomes (never guessed as
  `failed`), background reconciler with backoff + `needs_admin_review` escalation, user/admin
  retry and reconcile/resolve/reissue paths.
- **Epic 8 — Unified web app (`bohikor/`):** one Next.js app — employee portal, company admin,
  platform console — replacing the old single-tenant admin dashboard and mobile-only employee
  flow; renamed `admin/` → `bohikor/`, de-shadcn'd onto bare Radix + Tailwind.
- **Epic 9 — Freeze mobile, docs, deploy:** `mobile/` frozen (kept, not primary, not deleted);
  docs rewritten for the multi-tenant model; deploy verified.
- **Post-Epic-9 UI passes:** dark-default theme, adaptive modals, collapsible sidebars, employee
  dashboard desktop redesign, platform-console modal/dialog polish.

---

## Backlog — payout error-handling/observability (from 2026-08-07 audit)

Every failure point in the payout flow is already caught and logged server-side (`slog`), and
handled errors reach the client as a specific `{error, code}` body (`bohikor/`'s
`extractApiError`) and the company/platform dashboards (`events` table, `needs_admin_review`
flag). The gaps below are about how fast an operator can *notice and trace* a problem, not about
silent failures. None are urgent; none block current work. Roughly ordered by effort:

- **Panic → generic empty 500.** `server.go` uses bare `gin.Recovery()`; a handler panic returns
  an empty body (not the standard `{error, code}` shape) and logs via Gin's own writer instead of
  `slog`, so it's a second, differently-formatted log stream. Swap for a custom recovery handler
  that logs through `slog` and returns the standard error shape. Small, isolated to `server.go`.
- **No event on transport-error → `processing`.** `disburseAndResolve` (`internal/handler/advance.go`)
  already logs a Campay transport error and marks the row `processing`, but emits no `events` row
  until the reconciler eventually resolves or escalates it — so admins have no signal that a
  payout is in an uncertain state until backoff exhausts (~24 min) or the no-ref grace period (10
  min) passes. Add a `payout_transport_error`/`payout_uncertain` event at the point the row is
  marked `processing`. Touches `advance_test.go`'s mock-querier assertions too.
- **No request-ID correlation between a client-visible error and its server log line.**
  `middleware.RequestID()` sets `X-Request-ID` on the response header but never threads it into
  `slog` calls or the `Logger()` middleware. For failures before an `advance_requests` row exists
  (settings load, float check, DB errors), there's no shared key to grep logs by. Needs a
  context-aware logging helper and switching `slog.Error(...)` call sites to
  `slog.ErrorContext(ctx, ...)` across `handler/`, `service/`, `reconciler/`, `campay/` — a real
  refactor, not a patch.
- **No proactive alert on `needs_admin_review` escalation.** Today this only shows up as a badge
  on the company/platform dashboards — nothing pages or emails anyone when automated recovery
  gives up on a payout. Needs a channel decision (email is the path of least resistance since
  `emailClient` already exists; Slack would need a new webhook client), wiring into
  `reconciler.bumpAttempt`/`handleNoRef`, and a recipient config.
