# Epic 8, Task 1 — Rename `admin/` → `bohikor/` with multi-tenant route skeleton + de-shadcn

## Context

Epics 6 (multi-tenant backend) and 7 (payout reliability & float) are done and merged to `main`.
Per `PLAN.md`'s epic ordering (`6 → 7 → 8 → 9`), the next work is Epic 8: folding the employee
portal, company admin, and platform super-admin into one Next.js app, replacing the pre-pivot
`admin/` dashboard. This spec covers Epic 8's task 1 only: give the app its new name and the route
skeleton PLAN.md specifies, wire up auth/tenant context so it can support `platform_admin` and
per-company routing, and move today's working admin pages into their new homes — without yet
building the real employee flows, platform console, or admin actions that later Epic 8 tasks (2–7)
deliver.

Confirmed against the current code: `admin/` today is single-tenant, single-role (`subjectType`
hardcoded to `"admin" | null`), has zero dynamic route segments, and zero references to
`company_slug`/`platform_admin`/tenant concepts anywhere in its source — this task introduces all of
that plumbing for the first time.

Alongside the structural rename, the user asked to drop `shadcn`/its generated component wrappers in
favor of hand-written Tailwind components, allowing a full visual rebrand later, but with this task
staying functionality-focused — visual identity (palette, type, layout feel) is deferred to a
dedicated design pass using the frontend-design skill and user check-ins, not decided here.

## Decisions

**Rename mechanic:** `git mv admin bohikor`, edit in place — not a fresh scaffold. Nothing in the
repo hardcodes the `admin/` path (no CI workflows, no root Makefile/docker-compose; `admin/`'s own
tsconfig/jest/shadcn configs are all directory-relative via `@/*`). A fresh scaffold would only cost
history and re-wired tooling for no benefit.

**Backend touch required first:** `AdminLogin` (`backend/internal/handler/auth.go:618`) already
loads the `company` row (for the suspended check) but its response omits `company_slug`; same gap
in `handleAdminMe` (`backend/internal/server/routes.go:47`). Since the new login flow must redirect
to `/{slug}/admin`, and the `[company]/layout.tsx` guard needs to compare the URL slug against the
authenticated subject's company, both endpoints need `company_slug` added now — this blocks the
frontend work, so it happens first. (One session-log note claiming `AdminLogin` doesn't gate
suspended companies is stale — it already does, lines 640–643; only the slug is missing.)

**`[company]/layout.tsx` guard is UX, not the security boundary.** It reads the `company` param
(Next 16: `params` is `Promise<{ company: string }>`), and on a slug/subject mismatch redirects to
the subject's actual company rather than 404ing — reserving 404 for slugs that don't resolve to any
company. The real tenant isolation stays server-side in `RequireAdmin`/`RequireActiveUser`/
`RequirePlatformAdmin`'s existing isolation-guarantee check (`backend/internal/middleware/role.go`),
so this client guard can be soft without opening a hole.

**Auth context gets a third subject type and a stored hint, not JWT decoding.** Extend
`AuthContextType.subjectType` to `"admin" | "platform_admin" | "user" | null`. Since the client
doesn't decode JWTs today, add a lightweight `getSubjectHint()`/`setSubjectHint()` pair to
`lib/auth.ts` (written at login, read on refresh) so the loader knows which `/me` endpoint to call
before calling it. There is no `/api/platform/me` route yet (confirmed — only `/api/admin/me` and
`/api/users/me` exist), so the `platform_admin` branch is wired but its profile fetch is a stub until
the platform-console task lands.

**Token storage keys stay unchanged.** `bohikor2_access_token`/`bohikor2_refresh_token` are already
brand-scoped (confirmed identical in `mobile/src/lib/auth.ts`), not app-scoped — no rename needed.
A `platform_admin` JWT carries an empty `company_id` claim and targets companies via URL/body params
on `/api/platform/*` routes, so one token pair is enough; no per-company token scoping required.

**Guard topology fans out from one `AuthGuard` to per-subtree guards:** root `layout.tsx` stays
provider-only (no redirect — both `/platform` and `/{company}/*` share one token store);
`[company]/layout.tsx` does company-context resolution only, no role gating (so
`/{company}/login` stays reachable unauthenticated); `[company]/admin/layout.tsx` is today's
`(main)/layout.tsx` relocated (requires `subjectType === "admin"` + renders `Sidebar`); new
`platform/layout.tsx` requires `subjectType === "platform_admin"`.

**De-shadcn: remove shadcn, keep Radix headless primitives underneath.** Remove the `shadcn` CLI
package, `admin/components.json`, `class-variance-authority`, and every generated wrapper in
`admin/src/components/ui/*`. Replace them with a small set of hand-written Tailwind components
(`Button`, `Input`, `Card`, `Table`, `Badge`, `Alert`, `Label`, `Textarea`, `Skeleton`, `Separator`)
that keep the same call signatures so consuming pages need import swaps, not logic rewrites.
`Dialog`, `DropdownMenu`, `Select`, `Checkbox`, `Switch` keep `radix-ui` underneath (per user
decision — least regression risk to accessibility/keyboard/focus behavior) but drop shadcn's
generated styling for directly-applied Tailwind classes. `lucide-react` (icons) and `sonner` (toast)
stay — standalone libraries, not shadcn — but `sonner`'s shadcn-styled wrapper is replaced with a
directly-styled Tailwind version. `globals.css` drops `@import "shadcn/tailwind.css"` and
`tw-animate-css`; the CSS-variable theme (colors, radius) is simplified to a neutral baseline now,
with the real visual identity decided in a later design pass, not this task.

**Visual design deferred but not ignored.** Task 1's stub/moved pages get clean, minimal Tailwind
styling — no bespoke branding yet. A dedicated design pass (typography, palette, visual identity)
happens next, using the frontend-design skill with AskUserQuestion checkpoints for style choices,
and the dev server run directly to check screens rather than asking the user to eyeball things.

## What moves vs. what's new

Moves as-is (behavior unchanged, only path + redirect targets + import sources updated):
`(auth)/login` → `[company]/admin/login`, `(main)/layout.tsx` → `[company]/admin/layout.tsx`,
`(main)/page.tsx` → `[company]/admin/page.tsx`, `(main)/invite` → `[company]/admin/invite`,
`(main)/requests` → `[company]/admin/requests`, `(main)/settings` → `[company]/admin/settings`,
`(main)/users` → `[company]/admin/users` (each `__tests__` folder moves with its page).

New stub routes needed to make the tree structurally complete (minimal typed components, not empty
files — task 1 doesn't implement their logic): `src/app/page.tsx` (landing+login), `platform/`
(layout + page), `[company]/page.tsx` (employee home), `[company]/{login,signup,verify,create-pin,
forgot-pin,reset-pin}/page.tsx`, `[company]/history/page.tsx`, `[company]/account/page.tsx`,
`[company]/admin/events/page.tsx`, `[company]/admin/balance/page.tsx` (the last two are genuinely
new pages per PLAN.md's tree gloss, not moves — today's `useEvents` only feeds dashboard stat cards,
and there is no balance/ledger page at all).

## Execution order

1. **Backend:** add `company_slug` to `AdminLogin`'s and `handleAdminMe`'s responses; extend their
   Go tests; `cd backend && go test ./...`.
2. **Mechanical rename:** `git mv admin bohikor`; update `package.json` name field; verify
   `npm install && npm run build` still succeeds unchanged; update prose references to `admin/` in
   `AGENTS.md`/`CLAUDE.md`/`docs/session-log.md`.
3. **De-shadcn:** remove shadcn/CVA deps and generated `ui/` components; write hand-rolled Tailwind
   replacements; simplify `globals.css`; verify build still succeeds with the new primitives wired
   into the still-untouched page tree.
4. **Route restructure + auth plumbing:** create `[company]/` tree, `git mv` each page per the table
   above, add `[company]/layout.tsx` (tenant guard), update `components/auth-provider.tsx`
   (subjectType union + hint-based loader), `lib/auth.ts` (hint read/write),
   `components/auth-guard.tsx`, `components/sidebar.tsx` (slug-prefixed hrefs + new nav entries),
   `components/forbidden.tsx` (parameterized back-link), and every hardcoded
   `router.push("/")`/`"/login"` in the moved login page. Then create the new stub routes.
5. **Fix moved tests:** update the login test's redirect-path assertion and add a `useParams` mock;
   re-run the other moved tests to confirm they pass with only import-path changes; add smoke tests
   for new stubs only if trivial.

## Critical files

- `backend/internal/handler/auth.go` (`AdminLogin`, `bindEmailPassword`)
- `backend/internal/server/routes.go` (`handleAdminMe`)
- `admin/src/components/providers/auth-provider.tsx`
- `admin/src/components/auth-guard.tsx`, `admin/src/components/sidebar.tsx`,
  `admin/src/components/forbidden.tsx`
- `admin/src/lib/auth.ts`
- `admin/src/app/(auth)/login/page.tsx` and its `__tests__`
- `admin/src/components/ui/*` (removed and replaced), `admin/components.json` (removed),
  `admin/src/app/globals.css` (simplified)

## Verification

Task 1's bar is "the rename/restructure/de-shadcn didn't regress anything," not the full Epic 8
exit criteria (real screens aren't built yet):

1. `cd backend && go test ./...` — covers the new `company_slug` fields.
2. `cd bohikor && npm run typecheck` — catches wrong `Promise<{company}>` param typing on the new
   dynamic-segment stubs, broken imports from the `git mv`, and broken imports from the de-shadcn
   swap.
3. `cd bohikor && npm run lint` — catches dead imports/unfixed redirect literals left by the move.
4. `cd bohikor && npm run build` — the real check for a forgotten `"use client"` or misused
   `useParams()` in a server component.
5. `cd bohikor && npm run test` — full green, specifically the updated login-redirect assertion and
   the other moved suites passing on import-path changes alone.
6. Manual dev-server spot check: `npm run dev`, hit `/`, `/{seed-company-slug}/admin/login`,
   `/platform`, and one employee stub (e.g. `/{slug}/history`) to catch a misplaced folder that
   typecheck/build won't catch since stubs render successfully regardless. Also visually confirm the
   moved admin pages (dashboard, invite, requests, settings, users) still render and function
   correctly with the new hand-rolled Tailwind components in place of shadcn's.

Out of scope for task 1's verification: real `platform_admin` profile loading (no `/api/platform/me`
yet), employee auth flow correctness, admin retry/reconcile actions, and any deliberate visual
design/branding — those verify in tasks 2–7 or a dedicated design pass.
