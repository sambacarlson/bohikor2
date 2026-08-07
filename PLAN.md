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
