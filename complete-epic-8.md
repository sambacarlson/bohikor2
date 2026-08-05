# Epic 8 — Unified web app (`bohikor/`): remaining execution plan

## Who this document is for

You are an AI coding agent picking up this repository cold. This document is self-contained: it
tells you exactly what to build, which files to touch, which backend endpoints to call (with exact
request/response shapes already verified against the Go source), which existing UI components to
reuse, and how to verify each piece of work before moving on. You should not need to guess API
contracts or invent new visual patterns — they are all specified below or point at an existing file
to copy the pattern from.

## Rules — read before doing anything

1. **Work on a dedicated branch — never commit to `main`.** Before starting Step 1, make sure
   `main` is up to date (`git checkout main && git pull`), then create and check out a new branch
   for this work, e.g. `git checkout -b epic-8-frontend`. Do all 24 steps on this one branch.
2. **Commit each step, individually, once it's done and verified.** After a step's verification
   command passes, stage only that step's files and commit with a message identifying the step,
   e.g. `git commit -m "Epic 8 step 9: employee login page"`. One commit per step — don't batch
   several steps into one commit, and don't commit a step that hasn't passed its own verification
   yet. You may `git push` this branch to the remote so a human can review it as it grows (a PR
   against `main`, opened once and updated with each push, works well) — but see rule 3.
3. **Never merge this branch into `main`, under any circumstances, for any reason.** Not
   `git merge`, not `gh pr merge`, not any other method. If you opened a PR, leave it open. Merging
   is a human decision made after reviewing the work — it is never something you decide or do.
4. **Work through the numbered steps in order.** Each step is a single, independently reviewable
   unit of work (typically one screen or one hook file, and now also one commit — see rules 1-3).
   Complete one step fully — including its verification command and its commit — then stop and wait
   to be told to continue, rather than proceeding automatically to the next step, unless the person
   running you has explicitly said to run several steps in a row.
5. **After every step**, run, from the `bohikor/` directory:
   ```bash
   npm run lint && npm run typecheck && npm run test
   ```
   All three must pass *before* you commit that step (see rule 2). If a step also touches a shared
   file used by earlier, already-built screens (e.g. `types/index.ts`, `lib/api.ts`), re-run the
   full test suite, not just tests for the new file.
6. **Do not touch `backend/` or `mobile/`.** All backend endpoints this plan needs already exist
   and are documented below with exact contracts. If you find a backend contract in this document
   doesn't match what the server actually returns, stop and report the discrepancy rather than
   changing backend code or guessing.
7. **Follow existing patterns exactly.** This codebase has an established hook pattern, page
   pattern, and test pattern (all shown below). Do not introduce a different state-management
   approach, a different HTTP client, a different component library, or a different test style. Do
   not add a UI library or dependency that isn't already in `bohikor/package.json`.
8. **Don't build ahead of the spec.** Each step lists exactly what to build. Don't add fields,
   pages, or options "while you're in there" beyond what's listed — if you think something is
   missing, note it at the end of your step instead of adding it.
9. **Mobile-first, desktop-enhanced**, per `PLAN.md`: default layout should look correct at a phone
   width (~375px); at the `md:` breakpoint (768px+) you may widen containers or move to a
   two-column layout, but don't design desktop-first.
10. **Visual style: match the existing admin screens, don't invent a new look.** Reuse the
    components in `src/components/ui/*` as-is. Don't add new colors, shadows, or a new visual
    language — the reference screens below (`[company]/admin/login/page.tsx`,
    `[company]/admin/(protected)/requests/page.tsx`) are the bar to match, not exceed.

---

## Reference patterns (read this section once, refer back as needed)

### The hook pattern (`src/hooks/*.ts`)

Every server-state read is a `useQuery`, every write is a `useMutation` that invalidates the
relevant query key on success. One file per resource. Copy this shape exactly:

```ts
// read
export function useThing(page = 1, perPage = 20) {
  return useQuery({
    queryKey: ["things", page, perPage],
    queryFn: async () => {
      const { data } = await api.get<{ data: Thing[] }>("/api/things", {
        params: { page, per_page: perPage },
      });
      return data.data;
    },
  });
}

// write
export function useCreateThing() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (input: CreateThingInput) => {
      const { data } = await api.post<{ data: Thing }>("/api/things", input);
      return data.data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["things"] });
    },
  });
}
```

Every JSON response from this backend is wrapped as `{ "data": <payload> }` on success (see
`internal/handler/handler.go JSONSuccess`) and `{ "error": string, "code"?: string }` on failure
(`JSONError`). Always type the axios call as `{ data: T }` and return `data.data`.

`src/lib/api.ts` already has the `api` axios instance with JWT-attach and refresh-on-401 wired up —
import `{ api }` from `@/lib/api` in every hook, never call axios directly.

### Two wire-shape gotchas (empirically verified against the Go backend — not guesses)

**1. Any field typed `sql.NullTime` on the Go side is NOT a plain string or `null` in the JSON.**
It serializes as a nested object: `{"Time": "2026-01-01T00:00:00Z", "Valid": true}` when set, or
`{"Time": "0001-01-01T00:00:00Z", "Valid": false}` when null — Go's `database/sql.NullTime` has no
custom JSON marshaling, unlike the `pgtype.*` types used elsewhere in this codebase. This affects:
`AdvanceRequest.last_reconciled_at`, `AdvanceRequest.next_retry_at`, and `User.terms_accepted_at`
(all called out again at their type definitions in step 1, and tracked as a backend inconsistency
in `ISSUES.md` — not something to fix here). **Practical rule: never render these three fields
directly as a date anywhere in this plan.** None of the steps below ask you to — if you're tempted
to add one, don't; leave it out instead.

**2. `amount_xaf` is a JSON number, not a string, everywhere it appears as a raw ledger/request
field** (i.e. anywhere except `balance_xaf`, which the backend explicitly converts to a real string
server-side and is safe to treat as one). This means: never call string-only methods on an
`amount_xaf` value (`.startsWith()`, `.includes()`, `.slice()`, etc.) — it will throw at runtime.
For display, template-literal interpolation works regardless of type (`` `${amount_xaf} XAF` ``).
For any numeric comparison (e.g. "is this amount negative"), wrap it first: `Number(amount_xaf) < 0`.
The TypeScript type stays `string` in step 1 for consistency with the rest of this already-shipped
codebase (which has the same, pre-existing quirk) — treat it as "typed string, behaves as number,
so only use type-agnostic operations."

### The page pattern

Client component (`"use client"` at the top), local `useState` for form fields, call a hook's
mutation in a submit handler, `toast.success`/`toast.error` from `sonner` for feedback. See
`src/app/[company]/admin/login/page.tsx` for the canonical form-page shape (email/password form,
loading state on the submit button, an `Alert` for inline errors, a toast on success/failure) and
`src/app/[company]/admin/(protected)/requests/page.tsx` for the canonical list-page shape (loading
state, empty state, a `Table`, a refresh button with a spinning icon while refetching).

Error message extraction from a failed API call — copy this exact pattern (already used in every
existing mutating page):

```ts
} catch (err: unknown) {
  const message =
    err && typeof err === "object" && "response" in err
      ? (err as { response?: { data?: { error?: string } } }).response?.data?.error || "<fallback text>"
      : "Failed to <action>";
  setError(message);
  toast.error("<short toast text>");
}
```

### The test pattern

One `__tests__/page.test.tsx` per page, next to the page file. Mock the hook module with
`jest.mock`, drive the mocked return value per test, render with `renderWithProviders` from
`@/test-utils` (wraps in `QueryClientProvider`; pass `{ withToaster: false }` if the page's own
toasts would clutter assertions). See
`src/app/[company]/admin/(protected)/requests/__tests__/page.test.tsx` for the exact shape: mock the
hook, assert heading/description render, assert loading state, assert empty state, assert data
renders in the table, assert a click handler (e.g. refresh) calls through to the mocked function.
Every new page in this plan needs an equivalent test file — write it as part of the same step, not
as a follow-up.

### Available UI primitives (`src/components/ui/*` — do not add more without asking)

`Alert`/`AlertDescription`, `Badge`, `Button`, `Card`/`CardHeader`/`CardTitle`/`CardDescription`/
`CardContent`, `DropdownMenu` (+ subparts), `Input`, `Label`, `Switch`, `Table`/`TableHeader`/
`TableBody`/`TableRow`/`TableHead`/`TableCell`, `Sonner` (toast). Icons come from `lucide-react`
(already a dependency). If a step needs something these don't cover (e.g. a modal/dialog), build it
as a plain fixed-position overlay `div` styled with Tailwind — do not pull in a new dependency for
it. `cn()` from `@/lib/utils` merges Tailwind classes (clsx + tailwind-merge), same as every
existing component.

### Auth/company context already in place

- `src/lib/auth.ts` — token storage (`getAccessToken`/`getRefreshToken`/`setTokens`/`clearTokens`)
  and a `SubjectHint` (`"admin" | "platform_admin"`, **step 2 below extends this to include
  `"user"`**) persisted so a page reload knows which `/me`-equivalent endpoint to call.
- `src/components/providers/auth-provider.tsx` — exposes `useAuth()`: `{ admin, subjectType,
  loading, signOut, refreshSubject }`. **Step 2 extends this with a `user` field**, loaded the same
  way `admin` is: on mount, if the stored hint is `"user"`, call `GET /api/users/me` and store the
  result.
- `src/components/auth-guard.tsx` — `<AuthGuard loginHref="...">` redirects to `loginHref` if
  `!subjectType` once loading resolves; renders a spinner while loading. Already used by both the
  admin and platform protected layouts — step 7 below adds the equivalent for the employee area.
- `src/components/forbidden.tsx` — `<ForbiddenPage backHref backLabel>`, a 403 screen with
  "sign out and go back" — reused as-is for the employee area's wrong-subject-type case.

---

## Step 1 — Extend `src/types/index.ts`

**Goal:** every type the rest of this plan needs, added in one pass so later steps just `import
type { X } from "@/types"`.

**File:** `src/types/index.ts` (extend, don't replace — `User`... wait, there is no `User` type yet
for the employee; `Invitation`, `Admin`, `AdvanceRequest` already exist and need extending, not
replacing).

Add/change exactly this:

```ts
// CHANGE: RequestStatus was missing "initiated" and "processing" (Epic 7 added the latter as a
// distinct state — "Campay called, outcome unconfirmed", see AGENTS.md's payout flow section).
export type RequestStatus = "initiated" | "processing" | "pending" | "success" | "failed";

export type UserStatus = "active" | "suspended" | "locked";

// NEW — the employee record, shaped exactly like backend/internal/handler/auth.go's
// sanitizeUser() allowlist (verified against the Go source; does NOT include pin_hash or lockout
// fields, unlike the raw GET /api/users/me response today — see ISSUES.md, this is a known
// backend gap, not something to work around on the frontend).
export interface User {
  id: string;
  email: string;
  email_verified: boolean;
  full_name: string | null;
  phone_number: string | null;
  phone_verified: boolean;
  status: UserStatus;
  is_terms_accepted: boolean;
  // Deliberately `unknown`, not `string | null`: the backend's `sql.NullTime` wire shape is
  // `{ Time: string; Valid: boolean }`, never a plain string (see "wire-shape gotchas" above).
  // `unknown` forces a cast/narrow before use — don't widen this to `string | null` and don't
  // render it; use `is_terms_accepted` (a plain boolean) for any accepted/not-accepted UI instead.
  terms_accepted_at: unknown;
  terms_version: string | null;
  created_at: string;
  updated_at: string;
}

// NEW
export interface AuthResponse {
  user: User;
  company_slug?: string; // present on POST /api/auth/login only; absent elsewhere (client already knows the slug from the URL)
  access_token: string;
  refresh_token: string;
  expires_in: number;
}

// NEW
export interface RequestWindow {
  start_day: number;
  end_day: number;
  in_window: boolean;
}

// NEW
export interface EligibilityResponse {
  eligible: boolean;
  reasons: string[];
  kill_switch_active: boolean;
  request_window: RequestWindow;
  daily_requests_remaining: number;
  monthly_requests_remaining: number;
  advance_amount_xaf: string;
  phone_verified: boolean;
  terms_accepted: boolean;
}

// NEW
export interface PhoneVerificationStatus {
  phone_number: string;
  phone_verified: boolean;
  verification: {
    id: string;
    status: RequestStatus;
    created_at: string;
    ussd_code?: string;
  } | null;
}

// CHANGE — AdvanceRequest gains the Epic 7 resilience columns. All the new fields are optional
// because the employee-facing GET /api/advance-requests still returns them (they're just columns
// on the same row) but the employee UI won't display most of them; they matter for the company
// admin's requests page (step 20).
export interface AdvanceRequest {
  id: string;
  company_id?: string;
  user_id: string;
  user_email?: string;
  amount_xaf: string;
  status: RequestStatus;
  campay_payout_ref: string | null;
  failure_reason: string | null;
  payout_duration_seconds: number | null;
  attempt_count?: number;
  // Both deliberately `unknown`, not `string | null` — same sql.NullTime wire-shape gotcha as
  // User.terms_accepted_at above. Not rendered anywhere in this plan; leave them that way.
  last_reconciled_at?: unknown;
  next_retry_at?: unknown;
  needs_admin_review?: boolean;
  reissued_from_id?: string | null;
  created_at: string;
  updated_at: string;
}

// NEW — company, as returned by every /api/platform/companies* endpoint
// (backend/internal/handler/platform.go companyResponse()).
export type CompanyStatus = "active" | "suspended";

export interface Company {
  id: string;
  slug: string;
  name: string;
  status: CompanyStatus;
  balance_xaf: string;
  created_at: string;
  updated_at: string;
}

// NEW — one row from GET /api/admin/ledger's "entries" array
// (backend db.CompanyLedger struct, JSON as returned).
export type LedgerEntryType = "topup" | "payout_debit" | "reversal" | "adjustment";

export interface LedgerEntry {
  id: string;
  company_id: string;
  entry_type: LedgerEntryType;
  amount_xaf: string; // signed: topup/reversal positive, payout_debit negative
  advance_request_id: string | null;
  created_by: string | null;
  note: string | null;
  created_at: string;
}

// NEW — GET /api/admin/ledger response shape
export interface LedgerResponse {
  balance_xaf: string;
  entries: LedgerEntry[];
}

// NEW — one row from GET /api/platform/requests/needs-review
export interface RequestNeedingReview extends AdvanceRequest {
  company_slug: string;
  company_name: string;
}

// NEW — one row from GET /api/platform/requests/health
export interface CompanyRequestHealth {
  company_id: string;
  company_slug: string;
  company_name: string;
  processing_count: number;
  pending_count: number;
  needs_review_count: number;
}
```

**Verification:** `npm run typecheck` passes (nothing consumes these types yet, so this is just a
compile check on the file itself). No test file needed for a pure type addition.

---

## Step 2 — Employee auth: `lib/auth.ts`, `lib/api.ts`, `auth-provider.tsx`, `hooks/use-auth.ts`

**Goal:** the plumbing every employee-facing screen needs — token/subject-hint storage that knows
about the `user` subject type, an auth context that exposes the logged-in employee, and the auth
mutation hooks (check-invite, send-otp, verify-otp, login, create-pin, forgot-pin).

### 2a. `src/lib/auth.ts`

Change:
```ts
export type SubjectHint = "admin" | "platform_admin" | "user";
```
Nothing else in this file needs to change — `setSubjectHint("user")` will just work once the type
allows it.

### 2b. `src/lib/api.ts`

`reauthRedirectPath()` currently only distinguishes `/platform` vs `/{company}/admin`. Add the
employee case — anything under `/{company}` that isn't `/admin` should bounce to
`/{company}/login`:

```ts
function reauthRedirectPath(): string {
  const { pathname } = window.location;
  if (pathname.startsWith("/platform")) return "/platform/login";
  const [, company, section] = pathname.split("/");
  if (!company) return "/";
  if (section === "admin") return `/${company}/admin/login`;
  return `/${company}/login`;
}
```

### 2c. `src/components/providers/auth-provider.tsx`

Add a `user` field, loaded the same way `admin` is (mirror the existing `hint === "platform_admin"`
/ `admin` branches — add a third branch for `hint === "user"` that calls `GET /api/users/me`):

- `AuthContextType` gains `user: User | null`.
- `useSubjectLoader` gains a `user` state, and in `load()`, when `hint === "user"`, call
  `api.get<{ data: User }>("/api/users/me")`, set `user` + `subjectType = "user"` on success, clear
  everything and `clearTokens()` on failure (same pattern as the existing `admin` branch).
- `AuthProvider`'s context value includes `user`.

### 2d. `src/hooks/use-auth.ts` (new file)

Six mutations, one query. All hit unauthenticated `/api/auth/*` endpoints (no JWT needed — these
run before login) except where noted.

```ts
import { useMutation, useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { AuthResponse } from "@/types";

// GET /api/auth/check-invite?email=<email>
// Success (200): { data: { has_invitation: true, status: "pending"|"sent"|"accepted"|"revoked"|"failed" } }
// No active invitation (404): { error: "no_invitation", ... } — treat any error response as "not invited".
export function useCheckInvite(email: string, enabled: boolean) {
  return useQuery({
    queryKey: ["check-invite", email],
    queryFn: async () => {
      const { data } = await api.get<{ data: { has_invitation: true; status: string } }>(
        "/api/auth/check-invite",
        { params: { email } }
      );
      return data.data;
    },
    enabled,
    retry: false,
  });
}

// POST /api/auth/send-email-otp  { email }  -> 200 { status: "ok" } (no data payload)
export function useSendEmailOtp() {
  return useMutation({
    mutationFn: async (email: string) => {
      await api.post("/api/auth/send-email-otp", { email });
    },
  });
}

// POST /api/auth/verify-email-otp  { email, code, purpose: "signup" | "pin_reset" }
// purpose="signup"    -> 200 { status: "ok" } (no tokens yet — proceed to create-pin)
// purpose="pin_reset" -> 200 { data: AuthResponse } (tokens issued immediately — proceed to reset-pin, already authenticated)
export function useVerifyEmailOtp() {
  return useMutation({
    mutationFn: async (input: { email: string; code: string; purpose: "signup" | "pin_reset" }) => {
      const { data } = await api.post<{ data?: AuthResponse }>("/api/auth/verify-email-otp", input);
      return data.data ?? null;
    },
  });
}

// POST /api/auth/login  { email, pin }  -> 200 { data: AuthResponse } (includes company_slug)
export function useLogin() {
  return useMutation({
    mutationFn: async (input: { email: string; pin: string }) => {
      const { data } = await api.post<{ data: AuthResponse }>("/api/auth/login", input);
      return data.data;
    },
  });
}

// POST /api/auth/create-pin  { email, pin }  -> 201 { data: AuthResponse } (no company_slug)
export function useCreatePin() {
  return useMutation({
    mutationFn: async (input: { email: string; pin: string }) => {
      const { data } = await api.post<{ data: AuthResponse }>("/api/auth/create-pin", input);
      return data.data;
    },
  });
}

// POST /api/auth/forgot-pin  { email }  -> 200 { status: "ok" }
export function useForgotPin() {
  return useMutation({
    mutationFn: async (email: string) => {
      await api.post("/api/auth/forgot-pin", { email });
    },
  });
}
```

**Error codes you'll see from these endpoints** (for building error copy in later steps — these are
the exact `code` values `JSONError` sends, read from `backend/internal/handler/auth.go`):
`invalid_pin`, `invalid_credentials`, `account_locked`, `account_suspended`, `too_many_attempts`,
`company_suspended` (login); `user_exists`, `no_invitation` (create-pin); `invalid_otp`,
`otp_permanently_blocked`, `otp_temporarily_blocked` (send/verify OTP); `no_invitation`,
`invitation_not_active` (send-email-otp); `not_found`, `account_locked` (forgot-pin).

**Verification:** `npm run typecheck && npm run lint`. No dedicated test file for this step (hooks
get exercised indirectly by the page tests in steps 8-14); `npm run test` should still pass
unchanged.

---

## Step 3 — Employee domain hooks: `use-eligibility.ts`, `use-advance-requests.ts`, `use-user.ts`

### 3a. `src/hooks/use-eligibility.ts` (new file)

```ts
// GET /api/advance-requests/eligibility -> { data: EligibilityResponse }
export function useEligibility() {
  return useQuery({
    queryKey: ["eligibility"],
    queryFn: async () => {
      const { data } = await api.get<{ data: EligibilityResponse }>("/api/advance-requests/eligibility");
      return data.data;
    },
    refetchInterval: 30_000, // matches mobile's home-screen polling cadence
  });
}
```

### 3b. `src/hooks/use-advance-requests.ts` (new file — note: distinct from the existing
`use-requests.ts`, which is the **company admin's** list of every employee's requests; this file is
the **employee's own** requests)

```ts
// GET /api/advance-requests -> { data: AdvanceRequest[] }
export function useMyAdvanceRequests() {
  return useQuery({
    queryKey: ["my-advance-requests"],
    queryFn: async () => {
      const { data } = await api.get<{ data: AdvanceRequest[] }>("/api/advance-requests");
      return data.data;
    },
    // Poll every 10s only while something is still in flight, matching mobile's history screen.
    refetchInterval: (query) => {
      const requests = query.state.data;
      const hasInFlight = requests?.some((r) => r.status === "initiated" || r.status === "processing" || r.status === "pending");
      return hasInFlight ? 10_000 : false;
    },
  });
}

// POST /api/advance-requests (no body) -> 201/202 { data: AdvanceRequest } on success;
// 4xx { error, code } on an eligibility failure (e.g. insufficient_employer_float).
export function useCreateAdvanceRequest() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      const { data } = await api.post<{ data: AdvanceRequest }>("/api/advance-requests");
      return data.data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["my-advance-requests"] });
      queryClient.invalidateQueries({ queryKey: ["eligibility"] });
    },
  });
}

// POST /api/advance-requests/:id/retry -> same response shape as create; only legal on a
// terminally "failed" request.
export function useRetryAdvanceRequest() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => {
      const { data } = await api.post<{ data: AdvanceRequest }>(`/api/advance-requests/${id}/retry`);
      return data.data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["my-advance-requests"] });
    },
  });
}
```

### 3c. `src/hooks/use-user.ts` (new file — the employee's own profile + account actions)

```ts
// PUT /api/users/terms  { version: "v1" }  -> { data: User }
export function useAcceptTerms() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      const { data } = await api.put<{ data: User }>("/api/users/terms", { version: "v1" });
      return data.data;
    },
  });
}

// PUT /api/users/me/pin  { current_pin, new_pin }  -> { data: { message: string } }
export function useChangePin() {
  return useMutation({
    mutationFn: async (input: { current_pin: string; new_pin: string }) => {
      const { data } = await api.put<{ data: { message: string } }>("/api/users/me/pin", input);
      return data.data;
    },
  });
}

// POST /api/users/phone  { phone_number }  -> { data: PhoneVerificationStatus }
export function useAddPhone() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (phone_number: string) => {
      const { data } = await api.post<{ data: PhoneVerificationStatus }>("/api/users/phone", { phone_number });
      return data.data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["phone-verification"] });
    },
  });
}

// GET /api/users/phone-verification -> { data: PhoneVerificationStatus }
export function usePhoneVerificationStatus() {
  return useQuery({
    queryKey: ["phone-verification"],
    queryFn: async () => {
      const { data } = await api.get<{ data: PhoneVerificationStatus }>("/api/users/phone-verification");
      return data.data;
    },
    refetchInterval: (query) => {
      const status = query.state.data?.verification?.status;
      return status === "initiated" || status === "pending" ? 5_000 : false;
    },
  });
}
```

After adding these, `useAuth()`'s `refreshSubject()` (already exists — re-fetches `/api/users/me`)
is what you call after terms-accept/phone-add/etc. to sync the `user` object in context. Don't add
a separate "refresh user" hook — reuse `refreshSubject` from `useAuth()`.

**Verification:** `npm run typecheck && npm run lint && npm run test` (should still pass — nothing
consumes these yet).

---

## Step 4 — `src/hooks/use-ledger.ts` (company admin)

```ts
// GET /api/admin/ledger?page=&per_page= -> { data: LedgerResponse }
export function useLedger(page = 1, perPage = 50) {
  return useQuery({
    queryKey: ["ledger", page, perPage],
    queryFn: async () => {
      const { data } = await api.get<{ data: LedgerResponse }>("/api/admin/ledger", {
        params: { page, per_page: perPage },
      });
      return data.data;
    },
  });
}
```

**Verification:** `npm run typecheck && npm run lint && npm run test`.

---

## Step 5 — Admin request-action hooks: extend `src/hooks/use-requests.ts`

Add three mutations to the existing file (don't touch the existing `useRequests` query):

```ts
// POST /api/admin/requests/:id/reconcile -> { data: AdvanceRequest }
// 409 { code: "nothing_to_poll" } if the request has no campay_payout_ref to poll.
export function useReconcileRequest() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => {
      const { data } = await api.post<{ data: AdvanceRequest }>(`/api/admin/requests/${id}/reconcile`);
      return data.data;
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["requests"] }),
  });
}

// POST /api/admin/requests/:id/resolve  { note: string }  -> { data: AdvanceRequest }
// 409 { code: "not_flagged_for_review" } if it's not currently flagged.
export function useResolveRequest() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (input: { id: string; note: string }) => {
      const { data } = await api.post<{ data: AdvanceRequest }>(`/api/admin/requests/${input.id}/resolve`, {
        note: input.note,
      });
      return data.data;
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["requests"] }),
  });
}

// POST /api/admin/requests/:id/reissue (no body) -> 201/202 { data: AdvanceRequest } on success.
// Only legal on a terminally "failed" request. Possible failures: 403 insufficient_employer_float,
// 409 already_reissued, 502 transfer_failed (still creates a row, just failed again — refetch).
export function useReissueRequest() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => {
      const { data } = await api.post<{ data: AdvanceRequest }>(`/api/admin/requests/${id}/reissue`);
      return data.data;
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["requests"] }),
  });
}
```

**Verification:** `npm run typecheck && npm run lint && npm run test`.

---

## Step 6 — Platform hooks: `use-companies.ts`, `use-platform-requests.ts`

### 6a. `src/hooks/use-companies.ts` (new file)

```ts
// GET /api/platform/companies -> { data: Company[] }
export function useCompanies() {
  return useQuery({
    queryKey: ["companies"],
    queryFn: async () => {
      const { data } = await api.get<{ data: Company[] }>("/api/platform/companies");
      return data.data;
    },
  });
}

// GET /api/platform/companies/:id -> { data: Company }
export function useCompany(id: string, enabled = true) {
  return useQuery({
    queryKey: ["company", id],
    queryFn: async () => {
      const { data } = await api.get<{ data: Company }>(`/api/platform/companies/${id}`);
      return data.data;
    },
    enabled,
  });
}

// POST /api/platform/companies  { slug, name }  -> 201 { data: Company }
// slug must match /^[a-z0-9]+(-[a-z0-9]+)*$/ (lowercase words separated by single hyphens) — this
// is enforced server-side (400 invalid_slug) and should also be validated client-side before submit.
export function useCreateCompany() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (input: { slug: string; name: string }) => {
      const { data } = await api.post<{ data: Company }>("/api/platform/companies", input);
      return data.data;
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["companies"] }),
  });
}

// PUT /api/platform/companies/:id/status  { status: "active" | "suspended" }  -> { data: Company }
export function useUpdateCompanyStatus() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (input: { id: string; status: "active" | "suspended" }) => {
      const { data } = await api.put<{ data: Company }>(`/api/platform/companies/${input.id}/status`, {
        status: input.status,
      });
      return data.data;
    },
    onSuccess: (_data, vars) => {
      queryClient.invalidateQueries({ queryKey: ["companies"] });
      queryClient.invalidateQueries({ queryKey: ["company", vars.id] });
    },
  });
}

// POST /api/platform/companies/:id/admins  { email, password }  -> 201 { data: { id, company_id, email, created_at } }
export function useCreateCompanyAdmin() {
  return useMutation({
    mutationFn: async (input: { companyId: string; email: string; password: string }) => {
      const { data } = await api.post<{ data: { id: string; company_id: string; email: string; created_at: string } }>(
        `/api/platform/companies/${input.companyId}/admins`,
        { email: input.email, password: input.password }
      );
      return data.data;
    },
  });
}

// POST /api/platform/companies/:id/ledger/topup  { amount_xaf: string, note?: string }
//   -> 201 { data: { entry: LedgerEntry, balance_xaf: string } }
export function useTopUpCompany() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (input: { companyId: string; amount_xaf: string; note?: string }) => {
      const { data } = await api.post<{ data: { entry: LedgerEntry; balance_xaf: string } }>(
        `/api/platform/companies/${input.companyId}/ledger/topup`,
        { amount_xaf: input.amount_xaf, note: input.note }
      );
      return data.data;
    },
    onSuccess: (_data, vars) => {
      queryClient.invalidateQueries({ queryKey: ["companies"] });
      queryClient.invalidateQueries({ queryKey: ["company", vars.companyId] });
    },
  });
}

// POST /api/platform/companies/:id/ledger/adjustment  { amount_xaf: string (signed, non-zero), note: string (required) }
//   -> same response shape as topup.
export function useAdjustCompanyLedger() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (input: { companyId: string; amount_xaf: string; note: string }) => {
      const { data } = await api.post<{ data: { entry: LedgerEntry; balance_xaf: string } }>(
        `/api/platform/companies/${input.companyId}/ledger/adjustment`,
        { amount_xaf: input.amount_xaf, note: input.note }
      );
      return data.data;
    },
    onSuccess: (_data, vars) => {
      queryClient.invalidateQueries({ queryKey: ["companies"] });
      queryClient.invalidateQueries({ queryKey: ["company", vars.companyId] });
    },
  });
}
```

### 6b. `src/hooks/use-platform-requests.ts` (new file)

```ts
// GET /api/platform/requests/needs-review -> { data: RequestNeedingReview[] }
export function useRequestsNeedingReview() {
  return useQuery({
    queryKey: ["platform-requests-needs-review"],
    queryFn: async () => {
      const { data } = await api.get<{ data: RequestNeedingReview[] }>("/api/platform/requests/needs-review");
      return data.data;
    },
  });
}

// GET /api/platform/requests/health -> { data: CompanyRequestHealth[] }
export function useRequestsHealth() {
  return useQuery({
    queryKey: ["platform-requests-health"],
    queryFn: async () => {
      const { data } = await api.get<{ data: CompanyRequestHealth[] }>("/api/platform/requests/health");
      return data.data;
    },
  });
}
```

**Verification:** `npm run typecheck && npm run lint && npm run test`.

---

## Step 7 — Restructure the employee area into a `(protected)` route group

**Why:** the admin and platform areas both use a `(protected)` route group with a shared layout
that wraps `<AuthGuard>` (see `src/app/[company]/admin/(protected)/layout.tsx` and
`src/app/platform/(protected)/layout.tsx`). The employee area's skeleton put `page.tsx` (home),
`history/`, and `account/` directly under `[company]/` alongside the pre-auth screens
(login/signup/etc.), with no such guard. Before building those three screens, restructure to match
the established pattern — a route group changes nothing about the URL (still `/{company}`,
`/{company}/history`, `/{company}/account`), it only changes the file layout.

**Steps:** the `(protected)` directory name contains parentheses, which are easy to mis-escape in
a shell command (`git mv` with a badly-escaped path just fails with a confusing "no such file"
error). Avoid the shell entirely for this — use your file read/write/delete tools directly, which
don't need any path escaping:
1. Read `src/app/[company]/page.tsx`, write its exact same content to
   `src/app/[company]/(protected)/page.tsx`, then delete the original
   `src/app/[company]/page.tsx`. (This is a plain content-preserving move — don't change anything
   in the file while doing this.)
2. Do the same for every file under `src/app/[company]/history/` → the equivalent path under
   `src/app/[company]/(protected)/history/` (there's just the one `page.tsx` today).
3. Do the same for every file under `src/app/[company]/account/` → the equivalent path under
   `src/app/[company]/(protected)/account/`.
4. Create `src/app/[company]/(protected)/layout.tsx`:

```tsx
"use client";

import { useParams } from "next/navigation";
import { AuthGuard } from "@/components/auth-guard";
import { useAuth } from "@/components/providers";
import { ForbiddenPage } from "@/components/forbidden";

export default function EmployeeProtectedLayout({ children }: { children: React.ReactNode }) {
  const { company } = useParams<{ company: string }>();
  return (
    <AuthGuard loginHref={`/${company}/login`}>
      <UserCheck company={company}>{children}</UserCheck>
    </AuthGuard>
  );
}

function UserCheck({ children, company }: { children: React.ReactNode; company: string }) {
  const { user, subjectType } = useAuth();

  if (subjectType !== "user" || !user) {
    return <ForbiddenPage backHref={`/${company}/login`} backLabel="Go to Login" />;
  }

  return <>{children}</>;
}
```

Note this deliberately does **not** check `user.company_slug` against the URL `company` param the
way `[company]/layout.tsx` does for admins — the `User` type has no `company_slug` field (the
employee JWT's `company_id` claim is what the backend actually enforces on every request; the
frontend doesn't need to duplicate that check because a user literally cannot fetch another
company's data no matter what URL they're on — see AGENTS.md's "isolation guarantee"). Don't add a
slug-mismatch check here.

**Verification:** `npm run typecheck && npm run lint && npm run test`. The existing
`src/app/[company]/__tests__/layout.test.tsx` and `stub-pages.test.tsx` import from the old paths
(`../[company]/page`, `../[company]/history/page`, `../[company]/account/page`) — update their
import paths to `../[company]/(protected)/page`, etc. `stub-pages.test.tsx` will keep passing
against the still-stub content for now; step 24 removes the stub assertions once the real pages
exist.

---

## Step 8 — Landing page (`src/app/page.tsx`)

**Goal:** replace the "Sign in — coming soon" stub. A single email input; on submit, resolve which
company the email belongs to and redirect into that company's login page. This page does **not**
perform the actual login (no PIN field) — it only routes the visitor, per PLAN.md: "Login resolves
the user's company from email, then redirects to `/{slug}`."

**There is no dedicated backend endpoint for "resolve company by email" alone.** Reuse
`GET /api/auth/check-invite?email=` — it doesn't return a company slug either. **Use this flow
instead**, which matches what the endpoints actually support: this page has two buttons/links, not
a smart auto-detect — "I'm an employee" and "I'm a company admin" — because the backend has no
email→company-slug lookup that works before knowing which role you are. Given that constraint,
build this page as:

- A short "Bohikor" heading/logo area.
- Two clearly separated actions: an email input + "Continue" button for employees, which submits to
  `GET /api/auth/check-invite?email=` — on 200 (`has_invitation: true`), you still don't have a
  slug, so this path cannot fully resolve without one. **Do not build a fake resolution flow.**
  Instead: a short helper line under the input — "Don't know your company's link? Ask your
  employer for your sign-in URL" — and the primary call to action is a note directing both
  employees and admins to their company-specific URL (`bohikor.app/{company}/login` or
  `/{company}/admin/login`), which is what they'll actually have received via the invitation email
  (step 6 of the original PLAN — invitation emails link straight to `/{company}/signup?email=...`,
  bypassing this page entirely for new signups) or already know from bookmarking.
- Keep this page intentionally minimal: a heading, one paragraph of explanatory copy, and a link to
  `/platform/login` in small text at the bottom (for the platform super-admin). Do not build
  speculative company-resolution UI the backend can't actually back.

This is a deliberate scope-down from the original PLAN.md route-tree bullet ("resolve company by
email → redirect /{slug}") — flag this explicitly back to the reviewer as a call worth confirming,
rather than building a non-functional lookup. Use this exact copy unless told otherwise:

```
Bohikor
Salary advances, made simple.

Your employer gives you a personal sign-in link when you're invited — check your
invitation email, or ask your admin if you can't find it.

[small text, bottom]  Platform administrator? Sign in →  (links to /platform/login)
```

**Files:** `src/app/page.tsx` (replace stub), `src/app/__tests__/stub-pages.test.tsx` (remove the
`LandingPage` row from the `stubs` array and its now-unused import — this page no longer renders
"Sign in — coming soon.").

**Test:** `src/app/__tests__/page.test.tsx` (new) — renders the heading and the platform-login
link (`getByRole("link", { name: /platform administrator/i })` has the right `href`).

**Verification:** `npm run typecheck && npm run lint && npm run test`.

---

## Step 9 — `src/app/[company]/login/page.tsx`

**Goal:** email + 5-digit PIN → `useLogin()` → store tokens, `setSubjectHint("user")`, redirect to
`/{company}` (home). Mirror `[company]/admin/login/page.tsx`'s structure exactly (Card, form,
Alert-on-error, loading button state, toast).

**Fields:**
- Email (`type="email"`, required, `autoComplete="email"`).
- PIN (`type="password"`, `inputMode="numeric"`, `maxLength={5}`, required). Client-side validation:
  exactly 5 digits (`/^\d{5}$/`) — show an inline validation message rather than submitting if not.

**Flow:**
```ts
const result = await login.mutateAsync({ email, pin });
setTokens(result.access_token, result.refresh_token);
setSubjectHint("user");
await refreshSubject();
toast.success("Signed in successfully");
router.push(`/${company}`);
```

**Error copy by code** (from `useLogin`'s possible error codes, step 2): `invalid_credentials` →
"Invalid email or PIN"; `account_locked` → show the server's message verbatim (it already includes
context); `account_suspended` → "Your account has been suspended."; `too_many_attempts` → show the
server's message verbatim (includes the wait time); `company_suspended` → "Your company account is
suspended. Please contact support."; anything else / no `code` → "Failed to sign in."

**Also add**, below the form: a "Forgot your PIN?" link → `/{company}/forgot-pin`, and "New here?"
→ `/{company}/signup` (both small text links under the Card, matching the visual weight of similar
secondary links elsewhere in the codebase).

If already authenticated as a `user` on mount (`useAuth()`'s `user` is set), redirect straight to
`/{company}` — mirror the `useEffect` pattern in `admin/login/page.tsx`.

**Test:** mirror `admin/login/__tests__/page.test.tsx` — heading renders, submits the form and
calls the mutation with the right payload, shows the loading state, shows an error Alert on
failure, redirects on success (assert `router.push` called with `/{company}`).

**Verification:** `npm run typecheck && npm run lint && npm run test`.

---

## Step 10 — `src/app/[company]/signup/page.tsx`

**Goal:** port `mobile/app/(auth)/signup.tsx`. Single email input, invitation-gated.

**Pre-fill from the invitation link — this is the entire point of this step, don't skip it.** The
backend's invitation email (`internal/email/email.go SendInvitation`, already shipped) links to
exactly `/{company}/signup?email={encoded email}`. On mount, read `email` from the URL via
`useSearchParams().get("email")` and pre-fill the email input's initial state with it (decoded —
`useSearchParams` already URL-decodes). Leave the field editable (don't make it read-only) in case
the invited person wants to correct a typo'd email or use a different one, but a returning visitor
who clicks the link should never have to retype it. If there's no `email` param (someone navigated
here directly), the field just starts empty as normal.

**Flow:**
1. On submit, call `GET /api/auth/check-invite?email=` (via `useCheckInvite`, `enabled: false` by
   default, trigger with `refetch()` on submit — or just call `api.get` directly in the submit
   handler if that's simpler; either is fine, but don't fire the query on every keystroke).
2. If the request 404s (no active invitation) → inline error: "No invitation found for this email.
   Contact your manager." (matches the backend's `no_invitation` message).
3. If it succeeds and `status === "accepted"` → the user already completed signup previously.
   Redirect to `/{company}/login` with a toast: "You already have an account — please sign in."
4. If it succeeds and `status` is `"pending"` or `"sent"` → call `useSendEmailOtp()` with the email,
   then navigate to `/{company}/verify?email={encodeURIComponent(email)}&purpose=signup`.
5. If it succeeds and `status` is `"revoked"` or `"failed"` — this shouldn't actually happen since
   `check-invite` only returns success for `pending|sent|accepted` (see the SQL in step-context
   above), but handle defensively with the same "No invitation found" message as a fallback.

**Fields:** just email, `type="email"`, required, client-side format check
(`/^[^\s@]+@[^\s@]+\.[^\s@]+$/`) before submitting.

**Copy:** heading "Create your account", description "Enter the email address your employer
invited you with.", button "Continue".

**Test:** mock `useCheckInvite`/`useSendEmailOtp`, assert each of the four branches above navigates
or errors correctly; assert that rendering the page with `?email=someone%40example.com` in the URL
pre-fills the input with `someone@example.com`.

**Verification:** `npm run typecheck && npm run lint && npm run test`.

---

## Step 11 — `src/app/[company]/verify/page.tsx`

**Goal:** port `mobile/app/(auth)/verify-email.tsx`. 6-digit OTP input. Reads `email` and `purpose`
from the URL query string (`useSearchParams`).

**Fields:** single OTP code input, `inputMode="numeric"`, `maxLength={6}`, required, exactly 6
digits client-side.

**Flow:**
```ts
const result = await verifyOtp.mutateAsync({ email, code, purpose }); // purpose from query string
if (purpose === "signup") {
  // result is null (verify-otp for signup doesn't return tokens) — proceed to create-pin
  router.push(`/${company}/create-pin?email=${encodeURIComponent(email)}`);
} else {
  // purpose === "pin_reset" — result is AuthResponse, already authenticated
  setTokens(result!.access_token, result!.refresh_token);
  setSubjectHint("user");
  await refreshSubject();
  router.push(`/${company}/reset-pin`);
}
```

**Resend cooldown:** a "Resend code" button/link, disabled for 60 seconds after the page loads (and
again after each resend), showing "Resend code in {n}s" — a simple `useState` countdown with
`setInterval`, matching mobile's behavior. Resend calls `useSendEmailOtp()` again with the same
email (only meaningful for `purpose=signup`; for `purpose=pin_reset` the equivalent resend is
`useForgotPin()`, called with the same email — branch on `purpose` for which mutation to call on
resend).

**Error copy:** `invalid_otp` → "Invalid OTP code"; `otp_permanently_blocked` → server message
verbatim; `otp_temporarily_blocked` → server message verbatim (includes wait time).

**Test:** assert the two post-verify branches route correctly with mocked `useVerifyEmailOtp`
returning `null` vs an `AuthResponse`; assert the resend button is disabled immediately after
mount and re-enables after the countdown (use `jest.useFakeTimers()`).

**Verification:** `npm run typecheck && npm run lint && npm run test`.

---

## Step 12 — `src/app/[company]/create-pin/page.tsx`

**Goal:** port `mobile/app/(auth)/create-pin.tsx`. Reads `email` from the URL query string.

**Fields:** PIN + Confirm PIN, both `inputMode="numeric"`, `maxLength={5}`. Client validation:
both exactly 5 digits, and equal to each other (inline error "PINs do not match" if not, checked
on submit).

**Flow:**
```ts
const result = await createPin.mutateAsync({ email, pin });
setTokens(result.access_token, result.refresh_token);
setSubjectHint("user");
await refreshSubject();
toast.success("Account created");
router.push(`/${company}`); // straight to home — no separate "welcome" screen
```

**Error copy:** `user_exists` → "An account with this email already exists. Please log in
instead." (with a link to `/{company}/login`); `no_invitation` → "No invitation found for this
email. Contact your manager."; `invalid_pin` → "PIN must be 5 digits."

**Test:** mismatched PINs shows inline error without calling the mutation; matching PINs calls
`useCreatePin` with the right payload; success path stores tokens and redirects to `/{company}`.

**Verification:** `npm run typecheck && npm run lint && npm run test`.

---

## Step 13 — `src/app/[company]/forgot-pin/page.tsx`

**Goal:** port `mobile/app/(auth)/forgot-pin.tsx`. Single email input.

**Flow:** submit calls `useForgotPin()` with the email; on success (regardless of whether the
email exists — but the backend does 404 on unknown email, see `code: "not_found"`), navigate to
`/{company}/verify?email={encoded}&purpose=pin_reset`. On `not_found`, show inline error "No
account found with this email" instead of navigating. On `account_locked`, show the server message
verbatim.

**Test:** success navigates with the right query string; `not_found` shows the error and does not
navigate.

**Verification:** `npm run typecheck && npm run lint && npm run test`.

---

## Step 14 — `src/app/[company]/reset-pin/page.tsx`

**Goal:** port `mobile/app/(auth)/reset-pin.tsx`. **Requires an active session** — the user arrived
here already authenticated (tokens were set in step 11's `pin_reset` branch). This page is
therefore NOT under the `(protected)` group (it's a step in the auth flow, reachable only via the
verify redirect, but still needs `useAuth()`'s `user` to be loaded) — wrap its content in a simple
inline check: if `loading` show a spinner, if `!user` redirect to `/{company}/login` (a lighter,
local version of `AuthGuard` — don't move this page into `(protected)`, since semantically it's
still part of the auth flow, not the main app).

**Fields:** New PIN + Confirm PIN, same validation as create-pin (5 digits, must match). **No
`current_pin` field** — identity was already proven via the email OTP.

**Flow:**
```ts
// PUT /api/users/me/pin/reset  { new_pin }  -> { data: { message: string } }
await resetPin.mutateAsync(newPin); // add this one-off call inline or as a small useResetPin hook in use-user.ts (step 3c) if you prefer consistency — either is fine, but pick one and don't duplicate the endpoint call in two places
await refreshSubject();
toast.success("PIN reset");
router.push(`/${company}`);
```
(If you add `useResetPin`, put it in `src/hooks/use-user.ts` next to `useChangePin` — same file,
same pattern: `PUT /api/users/me/pin/reset` `{ new_pin }` → `{ data: { message } }`.)

**Test:** mismatched PINs blocks submit; success calls the endpoint and redirects.

**Verification:** `npm run typecheck && npm run lint && npm run test`.

---

## Step 15 — `src/app/[company]/(protected)/page.tsx` (employee home)

**Goal:** port `mobile/app/(app)/home.tsx`. This is the most business-logic-heavy screen — read it
carefully.

**Data:** `useAuth()`'s `user`, plus `useEligibility()` (polls every 30s, already built in step 3).

**Derived state:**
```ts
const displayName = user.full_name || user.email;
const profileIncomplete = !user.phone_verified || !user.phone_number;
const advanceAmount = eligibility?.advance_amount_xaf
  ? (eligibility.advance_amount_xaf.includes("XAF") ? eligibility.advance_amount_xaf : `${eligibility.advance_amount_xaf} XAF`)
  : "10,000 XAF";
```

**"Request Advance" button gating** (`handleRequestAdvance`):
1. `if (!user.is_terms_accepted)` → `router.push(`/${company}/account?section=terms`)` (see step 16
   — terms live in the account page's terms section, not a standalone route, per the existing
   skeleton).
2. `else if (profileIncomplete)` → `router.push(`/${company}/account?section=phone`)`.
3. `else` → open a confirm modal (local `useState<boolean>`).

The button is always clickable (routes to fix a blocker rather than being disabled), but its visual
emphasis (e.g. `variant="default"` vs a muted style) should reflect
`user.is_terms_accepted && !profileIncomplete`.

**Confirm modal:** plain fixed-overlay `div` (no dialog primitive exists in this codebase — see
"Available UI primitives" above) showing the amount and this exact disclosure text: "This amount
plus any applicable charges will be deducted from your upcoming salary payment. You can only have
one active advance request at a time." Cancel / Confirm buttons. Confirm calls
`useCreateAdvanceRequest().mutateAsync()`; on success, close the modal and
`router.push(`/${company}/history`)`; on error, keep the modal open and show the extracted error
message (use the standard error-extraction pattern from the Reference section) inside the modal.

**Eligibility card:** loading spinner while `eligibilityLoading`; once loaded, show: the advance
amount, an Eligible/Not-eligible `Badge` (`variant="default"` / `"destructive"`),
`daily_requests_remaining` / `monthly_requests_remaining`, the request window as "Available days
{start_day}–{end_day} of the month", a red `Alert` "Advances are temporarily disabled" if
`kill_switch_active`, and — if `!eligible` — a bulleted list of `reasons`. A manual refresh icon
button next to the card header (same `RefreshCw` + `animate-spin` pattern as every other page).

**Gating banners** (mutually exclusive, shown above the eligibility card):
- If `!user.is_terms_accepted`: an `Alert` "You haven't accepted the terms yet" with an "Accept
  Terms" button → `/{company}/account?section=terms`.
- Else if `profileIncomplete`: an `Alert` with message "Add a phone number to request an advance."
  if no phone at all, or "Verify your phone number to request an advance." if a phone exists but
  isn't verified; button "Add Phone Number" / "Verify Phone" → `/{company}/account?section=phone`.

**"Your Information" card:** email (with a ✓ or — depending on `email_verified`), phone (✓ only if
both present and verified — otherwise show "Not verified" or "Not set"), `full_name` if present,
`status`, terms accepted yes/no.

**Header:** a simple top bar with "Bohikor" + the user's display name, and a `DropdownMenu`
(already available) with "Account" (→ `/{company}/account`) and "Sign Out" (`signOut()` then
`router.push(`/${company}/login`)`).

**Navigation:** a card/row linking to `/{company}/history` ("View Transaction History").

**Test:** cover the three button-gating branches (terms not accepted → navigates to account/terms;
profile incomplete → navigates to account/phone; both satisfied → opens modal); confirm-modal
success calls the mutation and navigates to history; eligibility loading/loaded/error states
render; the kill-switch banner renders when `kill_switch_active`.

**Verification:** `npm run typecheck && npm run lint && npm run test`.

---

## Step 16 — `src/app/[company]/(protected)/history/page.tsx`

**Goal:** port `mobile/app/(app)/history.tsx`. Read-only list, no inputs.

**Data:** `useMyAdvanceRequests()` (already polls every 10s while anything's in flight, per step
3b).

**States:** loading spinner; error state ("Failed to load history" + Retry button calling
`refetch()`); empty state ("No advance requests yet.").

**Each row/card:** amount (`{amount_xaf} XAF`), a status `Badge` color-coded — `initiated`/
`processing` → `secondary`, `pending` → `outline`, `success` → `default`, `failed` →
`destructive` (reuse the `statusVariant` map pattern already in
`admin/(protected)/requests/page.tsx`, extended with `processing`); formatted `created_at`
(`toLocaleDateString` + time, `en-US`); `campay_payout_ref` if present (monospace, small text);
`failure_reason` if present, in red/destructive text.

**Retry button:** for a row with `status === "failed"`, show a "Retry" button that calls
`useRetryAdvanceRequest().mutateAsync(id)`; on success, `toast.success` and let the list refetch
naturally (invalidation already wired in the hook); on error, `toast.error` with the extracted
message.

**Header:** back arrow (`router.back()`) + page title "History" + a manual refresh button (same
pattern as every other list page).

**Test:** loading/error/empty states; renders a list of mixed-status requests with correct badge
variants; clicking Retry on a failed row calls the mutation with the right id; Retry button is not
shown for non-failed rows.

**Verification:** `npm run typecheck && npm run lint && npm run test`.

---

## Step 17 — `src/app/[company]/(protected)/account/page.tsx`

**Goal:** consolidate `mobile/app/(app)/settings.tsx` + `settings/phone.tsx` +
`settings/change-pin.tsx` + `terms.tsx` into one page with three sections, since the existing
skeleton only stubbed a single `account/page.tsx` (no sub-routes) — this matches PLAN.md's route
tree, which lists "phone verify, change PIN, terms" as sub-bullets of the single `account/` entry,
not as separate routes.

Always render all three sections stacked vertically (mobile-first: one column, full width; at
`md:` you may lay them out as side-by-side cards) — don't build a tab component or conditionally
hide sections, three stacked `Card`s is enough, matching this codebase's plain style. To support
step 15's deep links (`?section=terms`, `?section=phone`), give each section's outer `Card` an
`id` matching its name (`id="phone"`, `id="pin"`, `id="terms"`), read `?section=` via
`useSearchParams`, and in a `useEffect` on mount, if present, call
`document.getElementById(section)?.scrollIntoView({ behavior: "smooth" })`. This is a convenience,
not a requirement for the page to function — if `section` is absent or doesn't match an id, do
nothing (the page still renders correctly, just not scrolled).

### Section 1: Phone verification (port `settings/phone.tsx`)

**Fields:** country code (text input, default `"+237"`, auto-prefix `+` if the user deletes it) +
phone number (`inputMode="tel"`). Combine as `fullPhone = countryCode + phoneNumber`. Validate
`fullPhone` against `/^\+[1-9]\d{6,14}$/` before submit.

**Flow:** `usePhoneVerificationStatus()` (already polls every 5s while pending, step 3c). If no
phone on file yet (`!verification?.phone_number`) and not just submitted, show the form; submit
calls `useAddPhone()`; on success, switch to the status view.

**Status view:** phone number, Verified yes/no, and — if `verification.ussd_code` is present — a
highlighted instructional box: "Dial the USSD code on your phone" + the code in monospace +
"Enter your mobile money PIN when prompted. Verification will complete automatically." Status text:
`success` → "Verified" (green), `failed` → "Failed" (red), `pending` → "Processing…" (yellow),
anything else → "Initiated" (yellow).

**Retry logic** (`canRetry`): `false` if no verification or already `phone_verified`; `true`
immediately if `status === "failed"`; `true` if `status` is `initiated`/`pending` AND at least 60
seconds have passed since `verification.created_at`; `false` otherwise. A "Try Again" button
(shown only when `canRetry`) resets back to the form.

When `verification.phone_verified` flips to `true`, call `refreshSubject()` once (guard with a
`useRef` so it only fires once, not every poll tick) so `useAuth()`'s `user.phone_verified` stays
in sync.

### Section 2: Change PIN (port `settings/change-pin.tsx` — **but actually implement it**, don't
port the mobile "Under Maintenance" placeholder)

Mobile's version is a static "under maintenance" screen despite the backend endpoint
(`PUT /api/users/me/pin`) and hook (`useChangePin`, step 3c) already existing and working — this
was flagged in the mobile-screen research as a real feature gap, not an intentional decision. Build
the actual form here:

**Fields:** Current PIN, New PIN, Confirm New PIN — all 5-digit numeric, same validation as
create-pin (new PINs match each other; none may be empty).

**Flow:** submit calls `useChangePin().mutateAsync({ current_pin, new_pin })`; success → clear the
form, `toast.success("PIN changed")`. Error: extract server message (likely `invalid_pin` if the
current PIN is wrong — check the response `error` text and show it inline) with fallback "Failed to
change PIN."

### Section 3: Terms (port `terms.tsx`)

Hardcoded terms text (five short numbered points): (1) this is a one-time salary advance of the
amount shown on your home screen; (2) the amount will be deducted from your next salary payment;
(3) you may only have one active advance request at a time; (4) payout is made via mobile money to
your verified phone number; (5) by accepting, you authorize the deduction described above.

If `user.is_terms_accepted` is already `true`, show a static "Terms Already Accepted" state — do
not show the checkbox/accept form again, matching mobile's one-time-accept behavior. Don't try to
also show *when* it was accepted: `user.terms_accepted_at` is one of the `unknown`-typed fields
from step 1 (see "wire-shape gotchas") precisely because it can't be safely rendered as a date —
skip it entirely rather than working around it.

Otherwise: a checkbox "I have read and accept the terms and conditions", an "Accept" button
disabled until checked and not pending. Submit calls `useAcceptTerms()` (already hardcodes
`version: "v1"`, step 3c); on success, `refreshSubject()` then `toast.success("Terms accepted")`.
On error, static text "Failed to accept terms. Please try again." (not derived from the server
message, matching mobile).

### Also on this page: Sign out

A "Sign Out" button/row (can live at the top or bottom of the page, your call) — `signOut()` then
`router.push(`/${company}/login`)`. No confirmation dialog, matching mobile.

**Test:** one test file covering all three sections is fine given they're one page — at minimum:
phone form validation + submit + status-view rendering + retry-eligibility logic; change-PIN
mismatch validation + submit; terms checkbox-gated submit + the "already accepted" branch;
`?section=` deep-linking scrolls to/highlights the right section (or at minimum, doesn't crash and
renders all three regardless of the param).

**Verification:** `npm run typecheck && npm run lint && npm run test`.

---

## Step 18 — Company admin: `[company]/admin/(protected)/balance/page.tsx`

**Goal:** replace the "Balance & ledger — coming soon" stub. Read-only (per PLAN.md's locked
decision: company admins view balance, only the platform super-admin funds it).

**Data:** `useLedger()` (step 4).

**Layout:** a prominent balance card at the top (`{balance_xaf} XAF` large text), then a `Table` of
`entries` below it — columns: Type (`Badge`, one color per `entry_type`: `topup`/`reversal` green,
`payout_debit` neutral/gray, `adjustment` blue), Amount (`{amount_xaf} XAF`, red text if
`Number(entry.amount_xaf) < 0` — **do not use `.startsWith("-")` or any other string method on
`amount_xaf`, it is a JSON number at runtime despite its `string` TypeScript type; see "wire-shape
gotchas" in the Reference section**), Note (`—` if null), Date (`created_at` formatted).
Loading/empty states matching every other list page (`"Loading ledger..."` / `"No ledger entries
yet"`). A refresh button, same pattern as `requests/page.tsx`.

Add a "Balance" nav item check: `src/components/sidebar.tsx` **already has** a `/{company}/admin/balance`
entry (line 31 in the current file) — no sidebar change needed for this step.

**Test:** mirror `requests/__tests__/page.test.tsx`'s structure — loading/empty/populated states,
balance renders, entry type badges render with the right variant, refresh button works. Include a
mocked entry with a negative `amount_xaf` (e.g. `-5000` as a **number**, not the string `"-5000"`
— your mock data must match the real wire shape) and assert the page renders without throwing and
shows it in red — this is the regression test for the `.startsWith` bug called out above.

**Verification:** `npm run typecheck && npm run lint && npm run test`.

---

## Step 19 — Company admin: `[company]/admin/(protected)/events/page.tsx`

**Goal:** replace the "Events — coming soon" stub with a dedicated events log (the dashboard's
`page.tsx` already shows a "Recent Events" widget using `useEvents()` — this is the equivalent
full, dedicated list page; the hook already exists at `src/hooks/use-events.ts`, don't touch it).

Reuse the exact `EVENT_LABELS`/`EVENT_COLORS` maps and row rendering already implemented inline in
`admin/(protected)/page.tsx` (dashboard) — copy that rendering logic into this page rather than
inventing a new event-row look, but show the full list (`useEvents()`'s full result, not
`.slice(0, 20)`), with the same refresh button pattern.

**Test:** mirror the existing pattern — loading/empty/populated, event label/color mapping renders
correctly, unmapped event types fall back gracefully (label = raw `event_type`, color = gray).

**Verification:** `npm run typecheck && npm run lint && npm run test`.

---

## Step 20 — Company admin: requests page gets reconcile/resolve/reissue actions

**Goal:** extend the existing (already-real, not a stub) `[company]/admin/(protected)/requests/page.tsx`
with the Epic 7 admin recovery actions, using the hooks from step 5. Don't rewrite the page —
add to it.

**Add a "Needs Review" column/indicator:** if `request.needs_admin_review`, show a small warning
`Badge` ("Needs Review", destructive/yellow variant) next to the status badge.

**Add an Actions column** with buttons shown conditionally per row:
- **Reconcile**: shown when `status` is `"processing"` or `"pending"`. Calls
  `useReconcileRequest().mutateAsync(id)`. On the `nothing_to_poll` error (409), toast:
  "This request has no payout reference to check yet." On success, toast "Reconciled — status:
  {new status}."
- **Resolve**: shown when `needs_admin_review` is `true`. Opens a small inline note input (plain
  overlay or an inline expanding row — your call, keep it simple) requiring a non-empty note before
  the "Confirm Resolve" button is enabled. Calls `useResolveRequest().mutateAsync({ id, note })`.
  On success, toast "Marked as resolved."
- **Reissue**: shown when `status === "failed"` AND `reissued_from_id` is not already set on some
  *other* row pointing at this one (you don't have that reverse lookup client-side — simplest
  correct approach: just show the button whenever `status === "failed"`, and handle the
  `already_reissued` 409 error with a clear toast: "This request has already been reissued." —
  don't try to pre-compute reissue-eligibility client-side). Calls
  `useReissueRequest().mutateAsync(id)`. On success (201/202), toast "Reissued — new request
  created."; on the `insufficient_employer_float` 403, toast that message verbatim; on
  `transfer_failed` (502), toast: "Reissue failed — the new attempt also failed. Check the request
  list for details."

Each action button should show a per-row loading state (disable just that row's buttons while its
mutation is pending — don't disable the whole table).

**Test:** for each action, mock the corresponding hook and assert: the button only renders under
the right status/flag condition, clicking it calls the mutation with the right argument, and a
representative error path (e.g. `nothing_to_poll` for reconcile, empty-note-disables-confirm for
resolve, `already_reissued` for reissue) is handled without crashing.

**Verification:** `npm run typecheck && npm run lint && npm run test`.

---

## Step 21 — Platform console: companies list + create company

**Goal:** replace the "Platform console — coming soon" stub in
`src/app/platform/(protected)/page.tsx`. This step builds the default view: a list of companies
with a "Create Company" flow. Steps 22-23 add more to this same file/page (company detail actions,
health overview) — don't restructure into sub-routes; keep it one page with local view state (see
the Reference section's note on why: no existing platform sub-route precedent to follow, and
PLAN.md doesn't call for one).

**Layout:** page header "Platform Console" + a "Create Company" button (top right, opens a form —
inline card or overlay, your call). Below it, a `Table` of companies from `useCompanies()`:
columns Name, Slug, Status (`Badge`, `default` for active / `destructive` for suspended), Balance
(`{balance_xaf} XAF`), Created. Each row is clickable/has a "Manage" button that sets a local
`useState<string | null>` (`selectedCompanyId`) — step 22 renders the detail panel when this is
set.

**Create Company form fields:** Name (text, required), Slug (text, required; helper text: "lowercase
letters, numbers, and hyphens only"; client-side validate against
`/^[a-z0-9]+(-[a-z0-9]+)*$/` before submit, matching the server's `slugPattern`). Optionally
auto-derive the slug from the name as the admin types (lowercase, spaces→hyphens, strip
non-matching characters) but still let them edit it — this is a nice-to-have, not required; keep it
simple if you skip it. On submit, `useCreateCompany().mutateAsync({ slug, name })`; on success,
`toast.success` and close the form; on `create_failed` (409, likely a duplicate slug), inline
error: "That slug is already taken." On `invalid_slug` (400), inline error matching the helper
text.

**Test:** companies list renders with correct status badges; create-company form validates slug
format client-side before submit; submit calls the mutation with the right payload; duplicate-slug
error renders inline.

**Verification:** `npm run typecheck && npm run lint && npm run test`.

---

## Step 22 — Platform console: company detail (top-up, adjust, suspend, create first admin)

**Goal:** when `selectedCompanyId` (from step 21) is set, render a detail panel (inline expansion
below the table, or a simple overlay — match whatever step 21 chose) using `useCompany(id)`.

**Detail panel contents:**
- Name, slug, status, balance (larger/prominent text).
- **Suspend/Activate button**: shows "Suspend" if `status === "active"`, "Activate" if
  `"suspended"`. Calls `useUpdateCompanyStatus()`. A confirm step before suspending (a plain
  `window.confirm`-style inline "Are you sure?" toggle is fine here — this is a consequential
  action) is recommended but keep it simple; at minimum, don't let a single misclick suspend a
  company — require two clicks (e.g. button turns into "Confirm Suspend?" on first click).
- **Top Up form**: Amount (XAF, numeric string, must be positive — client-validate `> 0` before
  submit) + optional Note. Calls `useTopUpCompany()`. On success, show the new balance and
  `toast.success`.
- **Adjustment form**: Amount (XAF, signed — can be negative for a correction; client-validate
  non-zero) + required Note (server requires it, 400 otherwise — enforce client-side too). Calls
  `useAdjustCompanyLedger()`. This is a manual correction tool — label it clearly, e.g. a small
  "Manual Adjustment (corrections only)" heading so it isn't confused with Top Up.
- **Create Admin form** (only meaningfully useful for a fresh company with no admin yet, but no
  harm in always showing it — the backend allows creating additional admins too): Email + Password
  fields. Calls `useCreateCompanyAdmin()`. On success, show the created admin's email in a success
  message ("Admin {email} created — share the password with them securely.") since the password
  isn't retrievable again. On `create_failed` (409, duplicate email), inline error: "That email
  already has an admin account."

**Test:** each of the four actions (suspend/activate, top-up, adjustment, create-admin) — form
validation blocks an invalid submit, valid submit calls the right mutation with the right payload,
success shows the expected feedback.

**Verification:** `npm run typecheck && npm run lint && npm run test`.

---

## Step 23 — Platform console: needs-review + health overview

**Goal:** add a third section to the same platform console page — a "Reconciliation Health"
overview using the two hooks from step 6b.

**Health table** (`useRequestsHealth()`): one row per company — Company, Processing count, Pending
count, Needs-Review count (highlight this column, e.g. a destructive-colored `Badge`, when > 0 —
this is the number that should draw the platform admin's eye). Sort is already handled
server-side (needs-review-heaviest companies first) — don't re-sort client-side.

**Needs-review queue** (`useRequestsNeedingReview()`): a table of the actual flagged request rows —
columns Company (slug/name), User email, Amount, Status, Created, a link/button to jump to that
company's admin requests page (`/{company_slug}/admin/requests`) so the platform admin can hand off
to the company admin's reconcile/resolve/reissue actions from step 20 — **the platform console
itself does not duplicate those actions**; it's read-only visibility, per the endpoint's own doc
comment ("Platform-admin visibility... the reconciler's escalation path today only surfaces
per-company... so there was no cross-tenant view"). Don't add reconcile/resolve/reissue buttons
here.

Empty states for both: "All companies healthy — no requests need review." / "Nothing flagged for
review." (positive framing, since empty is the good state here, unlike other list pages).

**Test:** health table renders with the needs-review column highlighted when non-zero; needs-review
queue renders company/user/status columns; the "jump to company" link has the correct
`/{company_slug}/admin/requests` href; both empty states render their positive-framing copy.

**Verification:** `npm run typecheck && npm run lint && npm run test`.

---

## Step 24 — Close-out pass

1. Delete the now-fully-replaced entries from `src/app/__tests__/stub-pages.test.tsx` — every page
   in the `stubs` array that this plan replaced with real content (`EmployeeHomePage`,
   `EmployeeLoginPage`, `SignupPage`, `VerifyPage`, `CreatePinPage`, `ForgotPinPage`,
   `ResetPinPage`, `HistoryPage`, `AccountPage`, `EventsPage`, `BalancePage`,
   `PlatformConsolePage`, and `LandingPage` from step 8) should be removed from the `stubs` array
   and its imports. If the array ends up empty, delete the whole file (`describe.each([])` on an
   empty array is a no-op test file, not worth keeping) — but double check every one of those pages
   really was replaced with real content in an earlier step before removing its stub assertion.
2. Run the full verification suite one more time from `bohikor/`:
   ```bash
   npm run lint && npm run typecheck && npm run test
   ```
3. Grep the diff for anything that still says "coming soon" — there shouldn't be any left in
   `src/app/`.
4. Do **not** touch `AGENTS.md`, `CLAUDE.md`, or `PLAN.md` — documentation updates for Epic 8
   happen once, at the end of the epic (Epic 9's docs task), not per-frontend-step.
5. Commit this step like every other (rule 2), push the branch (rule 2) if you haven't been
   pushing incrementally, and stop — **do not merge into `main`** (rule 3). Report which steps you
   completed and the final lint/typecheck/test output; leave the branch/PR for a human to review
   and merge.

---

## Known scope decisions made while writing this plan (flag if you disagree)

- **Landing page (step 8) does not implement automatic email→company redirect**, because no
  backend endpoint supports it (`check-invite` doesn't return a slug). This is a deliberate
  scope-down from `PLAN.md`'s route-tree bullet — confirm with a human before treating it as
  "wrong" and trying to build a workaround.
- **Employee "account" is one page with three sections**, not three routes, because that's what the
  existing skeleton stubbed and what `PLAN.md`'s route tree implies (sub-bullets under one `account/`
  entry, not separate top-level routes).
- **Platform console is one page with local view state**, not multiple routes, to match its
  existing single-file skeleton and avoid introducing a routing pattern (dynamic nested platform
  routes) with no precedent elsewhere in the codebase.
- **Change-PIN is actually implemented** (step 17) rather than porting mobile's "Under Maintenance"
  placeholder, since the backend endpoint and hook both already work — the maintenance placeholder
  was mobile-specific debt, not a deliberate product decision.

## Verification pass, 2026-08-05

This document was checked against the actual backend source (including two empirical Go
marshaling tests, not just reading code) after the two new endpoints and the invitation-email
change landed on `main`. Four corrections were made as a result — all already applied above, listed
here so a partial reader of this doc knows they exist: (1) `AdvanceRequest.last_reconciled_at`/
`next_retry_at` and `User.terms_accepted_at` retyped from `string | null` to `unknown`, since they
actually wire-serialize as `{Time, Valid}` objects (step 1, and the new "wire-shape gotchas"
subsection in the Reference section); (2) step 18's ledger-page instructions fixed from
`.startsWith("-")` to `Number(...) < 0` for detecting negative amounts, since `amount_xaf`
wire-serializes as a number despite its `string` TypeScript type; (3) step 10 (signup) gained the
`?email=` pre-fill it was missing — without it, the invitation email's whole point (a working
deep link) was silently defeated; (4) step 7's file-move instructions switched from shell `git mv`
with escaped parentheses to plain read/write/delete, since shell-escaping `(protected)` is an easy
way for any agent to fail confusingly. None of the underlying backend inconsistencies these
corrections work around block anything in this plan — they're tracked in `ISSUES.md` instead.
