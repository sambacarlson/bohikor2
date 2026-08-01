# PLAN.md — Bohikor

Salary-advance platform. Employees request an advance; it is paid instantly via Campay
mobile money. Originally a single-company mobile app; **pivoting to a multi-company web
platform** with per-employer float and hardened payment resilience.

---

## Shipped so far (Epics 1–5, single-tenant)

Condensed history — see `docs/session-log.md` for detail.

- **Epic 1 — Auth:** invitations, email OTP, user/admin accounts, event log.
- **Epic 2 — Request & payout:** `advance_requests`, Campay Withdraw API, webhook, admin requests page.
- **Epic 2.5 — Own auth:** removed Firebase. Backend is sole auth authority — HS256 JWT access
  tokens (15m) + opaque rotating refresh tokens (30d), bcrypt.
- **Epic 3 — PIN auth:** email + 5-digit PIN login, PIN rate limiting + account lock, phone
  verification via Campay Collect (USSD) instead of SMS.
- **Epic 4 — Pilot controls:** global JSONB `settings`, kill switch, request window, daily/monthly
  throttling, eligibility endpoint, OTP rate limiting.
- **Epic 5 — Hardening:** USSD persistence, post-Campay DB-failure fallbacks, webhook dedup.

**Known limits this pivot removes:** single company baked into the schema; global settings;
one Campay credential set in env; **synchronous payout with the webhook as the only async
recovery path** (a lost webhook strands a request in `pending`; a network timeout on transfer
is marked `failed` even though funds may have moved).

---

## Pivot — decisions (locked)

| Area | Decision |
| :--- | :--- |
| **Frontend** | Fold employee + admin into **one Next.js app**, rename `admin/` → `bohikor/`. Mobile app is **frozen** (kept, not primary, not deleted). |
| **Routing** | `/` landing+login · `/{company}` employee app · `/{company}/admin` company admin · `/platform` super-admin. |
| **Tenancy** | Every domain table gains `company_id`. Company derived from the **JWT claim** on authed calls; API routes stay flat. |
| **Email** | **Globally unique.** Login resolves the user's company from email, then redirects to `/{slug}`. |
| **Onboarding** | **Platform super-admin** provisions companies + their first admin and **sets each company's balance**. Company self-signup deferred. |
| **Campay** | **One platform Campay account** (global token + webhook secret). Each company has its own **float balance**; payouts draw from it. |
| **Balance** | **`company_ledger`** table (immutable entries; balance = running sum). Failed payout ⇒ reversal entry. |
| **Payout safety** | **Reconcile before deciding.** Timeouts/ambiguity ⇒ `processing`, never auto-`failed`. A reconciler polls Campay by our idempotent `external_reference`. |
| **Retries** | **No queue.** Auto (reconciler) + user-level retry (new attempt) + admin-level retry/resolve, chosen by failure nature. |
| **DB** | **Greenfield** — no data exists. Migration set is **rewritten** into a clean multi-tenant baseline; `company_id` is `NOT NULL` from the start. |

---

## Target data model

New/changed tables (full DDL lands in `docs/schema.md` during Epic 6; sketch here):

```
platform_admins(id, email UNIQUE, password_hash, created_at)          -- super-admins (global)

companies(
  id, slug UNIQUE, name, status company_status DEFAULT 'active',       -- active|suspended
  created_by UUID REFERENCES platform_admins(id), created_at, updated_at)

company_ledger(
  id, company_id FK NOT NULL,
  entry_type TEXT CHECK (entry_type IN ('topup','payout_debit','reversal','adjustment')),
  amount_xaf NUMERIC(14,2) NOT NULL,   -- signed: topup/reversal +, debit -
  advance_request_id UUID NULL,        -- set for debit/reversal
  created_by UUID NULL,                -- platform_admin for topup/adjustment
  note TEXT, created_at)
-- balance(company_id) = SUM(amount_xaf)

admins            += company_id FK NOT NULL           -- company admins, scoped
users             += company_id FK NOT NULL           -- email stays globally UNIQUE
invitations       += company_id FK NOT NULL
settings           : PK becomes (company_id, key)     -- per-company; seeded on company create
events            += company_id (nullable for platform events)
phone_verifications += company_id FK NOT NULL
advance_requests  += company_id FK NOT NULL
                   + resilience cols: attempt_count INT DEFAULT 0,
                     last_reconciled_at TIMESTAMPTZ, next_retry_at TIMESTAMPTZ,
                     needs_admin_review BOOLEAN DEFAULT FALSE
refresh_tokens     : subject_type CHECK now allows ('user','admin','platform_admin')
```

**Enums**
- `company_status`: `active | suspended`
- `request_status`: `initiated | processing | pending | success | failed`
  - add **`processing`** = Campay called, outcome unconfirmed (timeout/ambiguous). Never guessed.

**Advance request state machine**
```
initiated ──create row + reserve float (ledger debit)──▶ call Campay
   │ sync SUCCESSFUL ─▶ success
   │ sync PENDING    ─▶ pending      (reconciler + webhook confirm)
   │ sync FAILED     ─▶ failed       (post ledger reversal)
   │ timeout/network ─▶ processing   (reconciler resolves; NEVER failed here)
webhook / reconciler: pending|processing ─▶ success | failed(+reversal)
after N attempts & age ─▶ needs_admin_review = true
```

**Idempotency:** `advance_requests.id` is the Campay `external_reference`. The reconciler only
**reads** status (never re-POSTs) ⇒ no double pay. A user/admin retry creates a **new** request
(new id ⇒ new external_reference), a deliberate fresh payout.

---

## JWT & scoping

- Access-token claims add `company_id` (empty for `platform_admin`). `role` ∈ `user | admin | platform_admin`.
- `JWTAuth` middleware sets `subject_id`, `subject_type`, `company_id` in the Gin context.
- New `RequirePlatformAdmin`; `RequireAdmin`/`RequireActiveUser` additionally load the record and
  reject if `company.status = suspended`.
- **Every** sqlc query for tenant data takes a `company_id` param sourced from the claim. No handler
  trusts a company id from the request body/URL for authed reads/writes.

---

# Epic 6 — Multi-tenant foundation (backend)

**Goal:** the API is company-aware end to end; super-admin can create companies, first admins, and
set balances; settings are per-company.

1. **Rewrite migrations → clean baseline** (`000001_init.up/down.sql`): all tables above, enums,
   indexes (`idx_<table>_company_id`, `idx_company_ledger_company_id`, keep existing). Delete stale
   numbered migrations (authorized — greenfield). Update `docs/schema.md` as source of truth.
2. **sqlc queries** — add/rewrite in `db/queries/`:
   - `companies.sql` — Create, GetBySlug, GetByID, List, UpdateStatus.
   - `platform_admins.sql` — GetByEmail, GetByID, Create.
   - `company_ledger.sql` — CreateLedgerEntry, GetCompanyBalance, ListLedgerByCompany.
   - Thread `company_id` through `users.sql`, `admins.sql`, `invitations.sql`,
     `advance_requests.sql`, `phone_verifications.sql`, `events.sql`.
   - `settings.sql` — composite `(company_id, key)`; ListSettingsByCompany, UpsertSetting, seed helper.
   - `refresh_tokens.sql` — allow `platform_admin` subject_type.
   - Run `go generate ./db/...`.
3. **Config** (`internal/config`): keep single Campay creds. Add nothing per-company (platform account).
4. **Auth/JWT** (`internal/authjwt`, `internal/middleware`): add `company_id` claim; `RequirePlatformAdmin`;
   company + company-status enforcement in `RequireAdmin`/`RequireActiveUser`.
5. **Platform handler** (`internal/handler/platform.go`) under `/api/platform/*` (RequirePlatformAdmin):
   - `POST /companies` — create company (slug+name), seed default settings, return it.
   - `POST /companies/:id/admins` — create the company's first admin (email+temp password / invite).
   - `POST /companies/:id/ledger/topup` — post a `topup` ledger entry (super-admin sets balance).
   - `GET /companies`, `GET /companies/:id` — list/detail incl. computed balance.
   - `PUT /companies/:id/status` — suspend/activate a company.
6. **Auth handler** (`internal/handler/auth.go`): `Login` resolves company from email (global unique),
   verifies PIN + user active + company active, returns tokens **and** `company_slug` for redirect.
   Add `POST /api/auth/platform/login` (super-admin). Company admin login unchanged but scoped.
7. **Company scoping in existing handlers**: invitations, users, settings, advance, phone, events all
   filter/write by `company_id` from the claim.
8. **CLI** `cmd/create-platform-admin/main.go` (mirrors create-admin) to bootstrap the first super-admin.
9. **Tests:** company isolation (user A can't read company B), platform provisioning, per-company
   settings, login→company resolution, company-suspended blocks. `go test ./...` + `golangci-lint`.

---

# Epic 7 — Payout reliability & float (backend)

**Status: done**, merged to `main`.

**Goal:** no transaction can get permanently stuck; float is enforced and auditable; failures are
recoverable by the right actor.

1. **Ledger-gated payout** in `advance.go CreateRequest`:
   - New eligibility check: **company float ≥ advance amount** (reason `insufficient_employer_float`).
   - In one DB tx: create request (`initiated`) + post `payout_debit` ledger entry (reserve float).
   - Call Campay. Map outcome: SUCCESSFUL→`success`, PENDING→`pending`, FAILED→`failed`(+`reversal`),
     **timeout/transport error→`processing`** (set `next_retry_at`), **not** `failed`.
2. **Campay status lookup** (`internal/campay/client.go`): add `GetTransactionStatus(externalRef)` →
   `GET /transaction/{ref}/`. **Research step:** confirm Campay's status endpoint keys on our
   `external_reference` (vs Campay reference) and its idempotency semantics; adjust the ref we persist
   accordingly. Add unit tests (mock HTTP).
3. **Reconciler** (`internal/reconciler/`): background goroutine started in `server.New`, ticker every
   ~30s. `ListReconcilable` = requests in (`initiated` aged, `processing`, `pending`) with
   `next_retry_at <= now`. For each: poll Campay status → transition; on `failed` post ledger reversal;
   bump `attempt_count`, set `last_reconciled_at` + exponential `next_retry_at`. After max attempts &
   age ⇒ `needs_admin_review = true`. Same path reused by phone_verifications.
4. **Webhook** (`advance.go`): unchanged trigger, but transitions now go through the shared state-machine
   helper so ledger reversal + dedup are consistent with the reconciler.
5. **User-level retry** `POST /api/advance-requests/:id/retry`: allowed only when the request is
   terminal `failed` and no non-terminal request exists ⇒ creates a **new** request (fresh
   external_reference). Guarded by all normal eligibility + float checks.
6. **Admin-level actions** under `/api/admin/requests/:id/*` (company admin):
   - `POST /reconcile` — force an immediate Campay status poll.
   - `POST /resolve` — mark resolved after manual Campay-dashboard check (audited).
   - `POST /reissue` — deliberately re-initiate a payout (new external_reference) after confirming no
     funds moved. Posts fresh debit; audited.
   - Manual `adjustment` ledger entry (platform-admin only) for corrections.
7. **Eligibility endpoint**: add float + company-status reasons so mobile/web reflect them.
8. **Tests:** state-machine transitions (esp. timeout→processing→resolved), ledger debit/reversal math,
   reconciler backoff + needs-review flag, retry guards (no double active request, no double pay).

---

# Epic 8 — Unified web app (`bohikor/`)

**Goal:** one responsive Next.js app: employee portal (mobile-first, desktop-enhanced), company admin,
platform super-admin.

1. **Rename & restructure** `admin/` → `bohikor/`. Route tree:
   ```
   src/app/
     page.tsx                     landing + login (resolve company by email → redirect /{slug})
     platform/                    super-admin: companies list, create company + first admin,
                                  set/top-up balance, suspend, cross-company request health
     [company]/
       layout.tsx                 company context (validate slug vs JWT, theme, guards)
       page.tsx                   employee home: eligibility, request advance, balance-aware states
       login/ signup/ verify/ create-pin/ forgot-pin/ reset-pin/
       history/                   requests + statuses + user retry action
       account/                   phone verify, change PIN, terms
       admin/                     company admin: dashboard, users, invite, requests
                                  (+reconcile/resolve/reissue), settings, events, balance/ledger
   ```
2. **Shared data layer:** extend `lib/api.ts` (already has refresh rotation); TanStack Query hooks for
   companies, ledger/balance, requests (+retry/admin actions), eligibility, settings, users, invites,
   events. Auth provider gains `platform_admin` subject + company context.
3. **Employee flows:** port screen-for-screen from `mobile/app/**` (login, signup, verify-email,
   create-pin, forgot/reset-pin, home, history, terms, phone). Mobile-first layout; at ≥ md, widen
   (sidebar / two-column) for desktop.
4. **Company admin:** existing admin pages, now company-scoped, plus **balance/ledger view** and the
   **retry/resolve/reissue** actions + `needs_admin_review` queue.
5. **Platform console:** companies CRUD, create first admin, **set/top-up balance**, suspend/activate,
   a reconciliation/health overview.
6. **Invitation emails** link to `/{company}/signup?email=…` (slug embedded).
7. **Tests:** Jest + RTL for employee flows, admin retry actions, platform provisioning + balance,
   guards (wrong-company slug, suspended company). `npm run lint && typecheck && test`.

---

# Epic 9 — Freeze mobile, docs, deploy

1. **Freeze `mobile/`:** add a note (README/AGENTS) that it is frozen and no longer primary; keep tests
   green but no new features. Do **not** delete.
2. **Docs:** rewrite `docs/schema.md` (multi-tenant DDL, ledger, resilience columns, state machine),
   `docs/brief.md` (multi-company model, float, retries), update `AGENTS.md` (new structure, `bohikor/`,
   `/platform`, reconciler, ledger). Append a session-log entry.
3. **Deploy:** single Next.js app path-routing (`/`, `/{company}`, `/platform`); backend env unchanged
   (one Campay account); ensure reconciler goroutine is covered by graceful shutdown; `docker build`
   check per the AGENTS pre-push checklist; migrations run in CI pre-deploy.

---

## Sequencing & risks

- **Order:** Epic 6 → 7 → 8 → 9. 6 unblocks everything; 7 depends on 6's ledger/schema; 8 consumes 6+7
  APIs; 9 closes out.
- **Biggest risks:** (1) Campay status-lookup/idempotency semantics — verify early in Epic 7 before
  finalizing the `external_reference` contract. (2) Float debit/reversal correctness under concurrent
  requests — cover with tx + tests. (3) Reconciler + graceful shutdown (no double-processing on restart)
  — idempotent, read-only polling keeps this safe.
- **Open item to confirm during Epic 6:** whether company admins may *request* top-ups (vs view-only
  balance). Assumed **view-only**; super-admin is sole funder for the pilot.

## Rules (unchanged)

- **NEVER auto-commit.** Wait for explicit instruction.
- All `UPDATE` queries set `updated_at = NOW()`. Migrations numbered, no `IF [NOT] EXISTS`.
- Lint + typecheck + tests must pass for every changed workspace before a change is "done".
- Never commit secrets; per-company data always filtered by `company_id` from the JWT claim.
