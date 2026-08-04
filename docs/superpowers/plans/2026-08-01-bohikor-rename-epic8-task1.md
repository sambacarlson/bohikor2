# Epic 8 Task 1: Rename admin/ to bohikor/ Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rename `admin/` to `bohikor/`, give it the multi-tenant route skeleton PLAN.md's Epic 8 task 1 specifies (`platform/`, `[company]/`, `[company]/admin/`), remove the `shadcn` dependency in favor of hand-rolled Tailwind components (keeping Radix headless primitives underneath), and move today's working admin pages into their new homes — without yet building the real employee flows, platform console, or admin actions that later Epic 8 tasks deliver.

**Architecture:** All auth state lives client-side (localStorage tokens, React context) — there is no server session — so every guard/layout that needs to know who's signed in must be a Client Component using `useAuth()`/`useParams()`, never a Server Component reading `params` as a Promise. Route groups (`(protected)`) keep login pages as unguarded siblings of their guarded subtrees, exactly like the current `(auth)` vs `(main)` split, avoiding a redirect-loop where a guard sends an unauthenticated visitor to a login page that the same guard also wraps.

**Tech Stack:** Next.js 16 (App Router), React 19, TanStack Query 5, axios, Tailwind v4, Radix UI primitives (Label, Switch, DropdownMenu only), Jest + React Testing Library, Go 1.26 / Gin backend.

## Global Constraints

- Rename mechanic is `git mv admin bohikor` in place — no fresh scaffold (nothing in the repo hardcodes the `admin/` path).
- Backend must add `company_slug` to `AdminLogin` and `GET /api/admin/me` responses before any frontend redirect logic depends on it.
- Remove `shadcn` CLI, `class-variance-authority`, `tw-animate-css`, and `admin/components.json`. Keep `radix-ui` (for Label, Switch, DropdownMenu), `clsx`, `tailwind-merge`, `lucide-react`, `sonner`, `next-themes`.
- Do not change any existing product copy/branding strings (e.g. "Bohikor2 Admin") — only functional code (redirects, params, hrefs). New stub pages get neutral "coming soon" copy, no brand assertions.
- Token storage keys (`bohikor2_access_token`, `bohikor2_refresh_token`) stay unchanged — they already match `mobile/src/lib/auth.ts`.
- Every moved page's `@/...` imports need zero changes (repo uses only alias imports, confirmed via grep — no relative imports exist in any page file).

---

## Task 1: Backend — add `company_slug` to `GET /api/admin/me`

**Files:**
- Modify: `backend/internal/server/routes.go:17-19,47-76`
- Modify: `backend/internal/server/server_test.go:55-57,112-151`

**Interfaces:**
- Produces: `adminQuerier` interface now requires `GetCompanyByID(ctx context.Context, id uuid.UUID) (db.Company, error)` in addition to `GetAdminByID`. The real `db.Queries` struct passed at the call site (`server.go:117`) already implements this — no wiring change needed there.

- [ ] **Step 1: Write the failing test**

In `backend/internal/server/server_test.go`, update `testQuerier.GetCompanyByID` (line 55-57) to return a slug, and add an assertion to `TestAdminMeEndpoint_ActiveAdmin`:

```go
func (q *testQuerier) GetCompanyByID(ctx context.Context, id uuid.UUID) (db.Company, error) {
	return db.Company{ID: id, Slug: "acme", Status: db.CompanyStatusActive}, nil
}
```

Add this block right after the existing `if data["id"] != adminID.String() { ... }` check inside `TestAdminMeEndpoint_ActiveAdmin` (around line 150):

```go
	if data["company_slug"] != "acme" {
		t.Fatalf("expected company_slug acme, got %v", data["company_slug"])
	}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/server/... -run TestAdminMeEndpoint_ActiveAdmin -v`
Expected: FAIL — `data["company_slug"]` is `nil` because the handler doesn't return that field yet.

- [ ] **Step 3: Write minimal implementation**

In `backend/internal/server/routes.go`, replace the `adminQuerier` interface (lines 17-19):

```go
type adminQuerier interface {
	GetAdminByID(ctx context.Context, id uuid.UUID) (db.Admin, error)
	GetCompanyByID(ctx context.Context, id uuid.UUID) (db.Company, error)
}
```

Replace `handleAdminMe` (lines 47-76):

```go
func handleAdminMe(q adminQuerier) gin.HandlerFunc {
	return func(c *gin.Context) {
		subjectIDStr := c.GetString("subject_id")

		subjectID, err := uuid.Parse(subjectIDStr)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid subject id",
			})
			return
		}

		admin, err := q.GetAdminByID(c.Request.Context(), subjectID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "admin not found",
			})
			return
		}

		company, err := q.GetCompanyByID(c.Request.Context(), admin.CompanyID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to resolve company",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"id":           admin.ID,
				"company_id":   admin.CompanyID,
				"company_slug": company.Slug,
				"email":        admin.Email,
				"created_at":   admin.CreatedAt,
			},
		})
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/server/... -run TestAdminMeEndpoint -v`
Expected: PASS for both `TestAdminMeEndpoint_NotAdmin` and `TestAdminMeEndpoint_ActiveAdmin`.

- [ ] **Step 5: Commit**

```bash
cd /Users/carlson/space/bohikor2
git add backend/internal/server/routes.go backend/internal/server/server_test.go
git commit -m "feat(backend): add company_slug to GET /api/admin/me response"
```

---

## Task 2: Backend — add `company_slug` to `AdminLogin`

**Files:**
- Modify: `backend/internal/handler/auth.go:651-658`
- Modify: `backend/internal/handler/auth_flows_test.go:554-568`

**Interfaces:**
- Consumes: `company` variable already loaded at `auth.go:635` via `h.queries.GetCompanyByID(c.Request.Context(), admin.CompanyID)` for the suspended-company check.

- [ ] **Step 1: Write the failing test**

In `backend/internal/handler/auth_flows_test.go`, replace `TestAdminLogin_Success` (lines 554-568):

```go
func TestAdminLogin_Success(t *testing.T) {
	q := &flexAuthQuerier{getAdminByEmail: func(string) (db.Admin, error) {
		return db.Admin{ID: uuid.New(), CompanyID: uuid.New(), Email: "admin@acme.com", PasswordHash: "hashed:secret"}, nil
	}}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/admin/login", func(r *gin.Engine) { r.POST("/admin/login", h.AdminLogin) },
		`{"email":"admin@acme.com","password":"secret"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	// Regression: the response must never leak the bcrypt hash.
	if strings.Contains(w.Body.String(), "hashed:secret") || strings.Contains(w.Body.String(), "password_hash") {
		t.Fatalf("response leaks password hash: %s", w.Body.String())
	}

	var resp struct {
		Data struct {
			CompanySlug string `json:"company_slug"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Data.CompanySlug != "acme" {
		t.Fatalf("expected company_slug acme, got %q", resp.Data.CompanySlug)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/handler/... -run TestAdminLogin_Success -v`
Expected: FAIL — `resp.Data.CompanySlug` is empty string.

- [ ] **Step 3: Write minimal implementation**

In `backend/internal/handler/auth.go`, replace the `AdminLogin` success response (lines 651-658):

```go
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"admin":         sanitizeAdmin(admin),
			"company_slug":  company.Slug,
			"access_token":  tokens.AccessToken,
			"refresh_token": tokens.RefreshToken,
			"expires_in":    tokens.ExpiresIn,
		},
	})
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/handler/... -run TestAdminLogin -v`
Expected: PASS for `TestAdminLogin_Success`, `TestAdminLogin_SuspendedCompanyBlocks`, `TestAdminLogin_WrongPassword`, `TestAdminLogin_NotFound`, `TestAdminLogin_TokenFailed`.

- [ ] **Step 5: Run the full backend suite and commit**

Run: `cd backend && go test ./...`
Expected: PASS (no regressions elsewhere).

```bash
cd /Users/carlson/space/bohikor2
git add backend/internal/handler/auth.go backend/internal/handler/auth_flows_test.go
git commit -m "feat(backend): add company_slug to AdminLogin response"
```

---

## Task 3: Mechanical rename admin/ to bohikor/

**Files:**
- Rename: `admin/` → `bohikor/` (entire directory, via `git mv`)
- Modify: `bohikor/package.json:2`

**Interfaces:** None — this task is a pure rename, no behavior changes. All later tasks reference `bohikor/...` paths.

- [ ] **Step 1: Rename the directory**

```bash
cd /Users/carlson/space/bohikor2
git mv admin bohikor
```

- [ ] **Step 2: Update the package name**

In `bohikor/package.json`, change line 2 from `"name": "admin",` to `"name": "bohikor",`.

- [ ] **Step 3: Verify install and build still succeed unchanged**

Run: `cd bohikor && npm install && npm run build`
Expected: Build succeeds with no errors (this proves the mechanical move alone didn't break anything — all configs are directory-relative via `@/*` aliases).

- [ ] **Step 4: Update prose references in docs**

In `AGENTS.md`, update any `admin/` directory-tree mentions and the `cd admin && npm install` style instruction to `bohikor/`. In `CLAUDE.md`, update the note "`admin/` is still the pre-pivot Next.js dashboard — it has not yet been renamed/rebuilt into the unified `bohikor/` app" to reflect that the rename has happened (structure exists, full screens still pending tasks 2-7). Append one line to `docs/session-log.md` noting the rename landed.

- [ ] **Step 5: Commit**

```bash
cd /Users/carlson/space/bohikor2
git add -A -- bohikor AGENTS.md CLAUDE.md docs/session-log.md
git commit -m "refactor: rename admin/ to bohikor/ (Epic 8 task 1)"
```

---

## Task 4: De-shadcn — remove dependencies and unused components

**Files:**
- Delete: `bohikor/components.json`
- Delete: `bohikor/src/components/ui/checkbox.tsx`, `dialog.tsx`, `select.tsx`, `separator.tsx`, `skeleton.tsx`, `textarea.tsx` (confirmed unused by any page or component via grep)
- Modify: `bohikor/package.json` (remove `class-variance-authority`, `shadcn`, `tw-animate-css`)
- Modify: `bohikor/src/app/globals.css:1-3`

**Interfaces:** None — these files have zero importers anywhere in the codebase (verified: `grep -rn "from \"@/components/ui" app/ components/"` shows no matches for checkbox/dialog/select/separator/skeleton/textarea).

- [ ] **Step 1: Delete unused shadcn config and components**

```bash
cd /Users/carlson/space/bohikor2/bohikor
rm components.json
rm src/components/ui/checkbox.tsx src/components/ui/dialog.tsx src/components/ui/select.tsx src/components/ui/separator.tsx src/components/ui/skeleton.tsx src/components/ui/textarea.tsx
```

- [ ] **Step 2: Remove shadcn-only dependencies from package.json**

In `bohikor/package.json`, remove these three lines from `dependencies`:

```json
    "class-variance-authority": "^0.7.1",
    "shadcn": "^4.7.0",
    "tw-animate-css": "^1.4.0"
```

- [ ] **Step 3: Simplify globals.css**

In `bohikor/src/app/globals.css`, remove lines 2-3 so the file starts:

```css
@import "tailwindcss";

@custom-variant dark (&:is(.dark *));
```

(Everything after that — the `@theme inline` mapping and `:root`/`.dark` CSS variable blocks — is plain Tailwind v4 theming, not shadcn-specific, and stays unchanged. It's what every remaining `bg-primary`/`text-muted-foreground`/`border-border`-style class in the app resolves against.)

- [ ] **Step 4: Reinstall and verify build**

Run: `cd bohikor && npm install && npm run build`
Expected: FAILS at this point — `button.tsx`, `badge.tsx`, `alert.tsx` still import `class-variance-authority`, which no longer exists. This is expected; Task 5 fixes it. Confirm the failure is specifically a missing-module error for `class-variance-authority`, not something else.

- [ ] **Step 5: Commit**

```bash
cd /Users/carlson/space/bohikor2
git add bohikor/components.json bohikor/src/components/ui bohikor/package.json bohikor/package-lock.json bohikor/src/app/globals.css
git commit -m "chore(bohikor): remove shadcn CLI and unused ui components"
```

---

## Task 5: De-shadcn — rewrite Button, Input, Label, Card, Alert, Badge, Table

**Files:**
- Modify: `bohikor/src/components/ui/button.tsx`
- Modify: `bohikor/src/components/ui/input.tsx`
- Modify: `bohikor/src/components/ui/label.tsx`
- Modify: `bohikor/src/components/ui/card.tsx`
- Modify: `bohikor/src/components/ui/alert.tsx`
- Modify: `bohikor/src/components/ui/badge.tsx`
- Modify: `bohikor/src/components/ui/table.tsx`

**Interfaces:**
- Produces: `Button({variant?: "default"|"outline"|"ghost", size?: "default"|"sm"|"icon", ...React.ComponentProps<"button">})`, `Input(...React.ComponentProps<"input">)`, `Label(...React.ComponentProps<typeof LabelPrimitive.Root>)`, `Card({size?: "default"|"sm", ...})`/`CardHeader`/`CardTitle`/`CardDescription`/`CardContent`, `Alert({variant?: "default"|"destructive", ...})`/`AlertDescription`, `Badge({variant?: "default"|"secondary"|"destructive"|"outline", ...})`, `Table`/`TableHeader`/`TableBody`/`TableRow`/`TableHead`/`TableCell`. These exact names/props are consumed unchanged by every page moved in Task 9 — confirmed via grep that no page uses any prop/variant beyond this set (no `asChild` on Button/Badge, no `CardFooter`/`CardAction`, no `TableFooter`/`TableCaption`, no `AlertTitle`/`AlertAction`).

- [ ] **Step 1: Rewrite button.tsx**

```tsx
import * as React from "react";
import { cn } from "@/lib/utils";

const BUTTON_VARIANTS = {
  default: "bg-primary text-primary-foreground hover:bg-primary/80",
  outline:
    "border border-border bg-background hover:bg-muted hover:text-foreground",
  ghost: "hover:bg-muted hover:text-foreground",
} as const;

const BUTTON_SIZES = {
  default: "h-8 gap-1.5 px-2.5",
  sm: "h-7 gap-1 px-2.5 text-[0.8rem]",
  icon: "size-8",
} as const;

type ButtonVariant = keyof typeof BUTTON_VARIANTS;
type ButtonSize = keyof typeof BUTTON_SIZES;

function Button({
  className,
  variant = "default",
  size = "default",
  ...props
}: React.ComponentProps<"button"> & {
  variant?: ButtonVariant;
  size?: ButtonSize;
}) {
  return (
    <button
      data-slot="button"
      className={cn(
        "inline-flex shrink-0 items-center justify-center rounded-lg border border-transparent text-sm font-medium whitespace-nowrap transition-all outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:pointer-events-none disabled:opacity-50",
        BUTTON_VARIANTS[variant],
        BUTTON_SIZES[size],
        className
      )}
      {...props}
    />
  );
}

export { Button };
```

- [ ] **Step 2: Rewrite input.tsx**

```tsx
import * as React from "react";
import { cn } from "@/lib/utils";

function Input({ className, type, ...props }: React.ComponentProps<"input">) {
  return (
    <input
      type={type}
      data-slot="input"
      className={cn(
        "h-8 w-full min-w-0 rounded-lg border border-input bg-transparent px-2.5 py-1 text-base outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:pointer-events-none disabled:opacity-50 md:text-sm",
        className
      )}
      {...props}
    />
  );
}

export { Input };
```

- [ ] **Step 3: Rewrite label.tsx** (keeps Radix `Label` primitive)

```tsx
"use client";

import * as React from "react";
import { Label as LabelPrimitive } from "radix-ui";
import { cn } from "@/lib/utils";

function Label({
  className,
  ...props
}: React.ComponentProps<typeof LabelPrimitive.Root>) {
  return (
    <LabelPrimitive.Root
      data-slot="label"
      className={cn("text-sm leading-none font-medium select-none", className)}
      {...props}
    />
  );
}

export { Label };
```

- [ ] **Step 4: Rewrite card.tsx**

```tsx
import * as React from "react";
import { cn } from "@/lib/utils";

function Card({
  className,
  size = "default",
  ...props
}: React.ComponentProps<"div"> & { size?: "default" | "sm" }) {
  return (
    <div
      data-slot="card"
      className={cn(
        "flex flex-col gap-4 rounded-xl bg-card text-sm text-card-foreground ring-1 ring-foreground/10",
        size === "sm" ? "gap-3 py-3" : "py-4",
        className
      )}
      {...props}
    />
  );
}

function CardHeader({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="card-header"
      className={cn("grid auto-rows-min items-start gap-1 px-4", className)}
      {...props}
    />
  );
}

function CardTitle({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="card-title"
      className={cn("text-base leading-snug font-medium", className)}
      {...props}
    />
  );
}

function CardDescription({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="card-description"
      className={cn("text-sm text-muted-foreground", className)}
      {...props}
    />
  );
}

function CardContent({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div data-slot="card-content" className={cn("px-4", className)} {...props} />
  );
}

export { Card, CardHeader, CardTitle, CardDescription, CardContent };
```

- [ ] **Step 5: Rewrite alert.tsx**

```tsx
import * as React from "react";
import { cn } from "@/lib/utils";

const ALERT_VARIANTS = {
  default: "bg-card text-card-foreground",
  destructive: "bg-card text-destructive",
} as const;

function Alert({
  className,
  variant = "default",
  ...props
}: React.ComponentProps<"div"> & { variant?: keyof typeof ALERT_VARIANTS }) {
  return (
    <div
      data-slot="alert"
      role="alert"
      className={cn(
        "rounded-lg border px-2.5 py-2 text-sm",
        ALERT_VARIANTS[variant],
        className
      )}
      {...props}
    />
  );
}

function AlertDescription({
  className,
  ...props
}: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="alert-description"
      className={cn("text-sm text-muted-foreground", className)}
      {...props}
    />
  );
}

export { Alert, AlertDescription };
```

- [ ] **Step 6: Rewrite badge.tsx**

```tsx
import * as React from "react";
import { cn } from "@/lib/utils";

const BADGE_VARIANTS = {
  default: "bg-primary text-primary-foreground",
  secondary: "bg-secondary text-secondary-foreground",
  destructive: "bg-destructive/10 text-destructive",
  outline: "border border-border text-foreground",
} as const;

function Badge({
  className,
  variant = "default",
  ...props
}: React.ComponentProps<"span"> & { variant?: keyof typeof BADGE_VARIANTS }) {
  return (
    <span
      data-slot="badge"
      className={cn(
        "inline-flex h-5 w-fit shrink-0 items-center justify-center gap-1 rounded-full border border-transparent px-2 py-0.5 text-xs font-medium whitespace-nowrap",
        BADGE_VARIANTS[variant],
        className
      )}
      {...props}
    />
  );
}

export { Badge };
```

- [ ] **Step 7: Rewrite table.tsx**

```tsx
import * as React from "react";
import { cn } from "@/lib/utils";

function Table({ className, ...props }: React.ComponentProps<"table">) {
  return (
    <div data-slot="table-container" className="relative w-full overflow-x-auto">
      <table
        data-slot="table"
        className={cn("w-full caption-bottom text-sm", className)}
        {...props}
      />
    </div>
  );
}

function TableHeader({ className, ...props }: React.ComponentProps<"thead">) {
  return (
    <thead
      data-slot="table-header"
      className={cn("[&_tr]:border-b", className)}
      {...props}
    />
  );
}

function TableBody({ className, ...props }: React.ComponentProps<"tbody">) {
  return (
    <tbody
      data-slot="table-body"
      className={cn("[&_tr:last-child]:border-0", className)}
      {...props}
    />
  );
}

function TableRow({ className, ...props }: React.ComponentProps<"tr">) {
  return (
    <tr
      data-slot="table-row"
      className={cn("border-b transition-colors hover:bg-muted/50", className)}
      {...props}
    />
  );
}

function TableHead({ className, ...props }: React.ComponentProps<"th">) {
  return (
    <th
      data-slot="table-head"
      className={cn(
        "h-10 px-2 text-left align-middle font-medium whitespace-nowrap text-foreground",
        className
      )}
      {...props}
    />
  );
}

function TableCell({ className, ...props }: React.ComponentProps<"td">) {
  return (
    <td
      data-slot="table-cell"
      className={cn("p-2 align-middle whitespace-nowrap", className)}
      {...props}
    />
  );
}

export { Table, TableHeader, TableBody, TableRow, TableHead, TableCell };
```

- [ ] **Step 8: Verify build**

Run: `cd bohikor && npm run build`
Expected: FAILS only on `switch.tsx` and `dropdown-menu.tsx` still referencing the old elaborate shadcn class names that depended on `tw-animate-css` utilities (`animate-in`, `fade-in-0`, etc. are unresolved Tailwind classes — this doesn't fail the build, Tailwind just silently drops unknown utilities, so the build should actually succeed here). Run `npm run typecheck` too — expect PASS, since `switch.tsx`/`dropdown-menu.tsx` still compile fine (they're deferred to Task 6, not broken, just still shadcn-styled).

- [ ] **Step 9: Commit**

```bash
cd /Users/carlson/space/bohikor2
git add bohikor/src/components/ui/button.tsx bohikor/src/components/ui/input.tsx bohikor/src/components/ui/label.tsx bohikor/src/components/ui/card.tsx bohikor/src/components/ui/alert.tsx bohikor/src/components/ui/badge.tsx bohikor/src/components/ui/table.tsx
git commit -m "refactor(bohikor): replace shadcn button/input/label/card/alert/badge/table with plain Tailwind"
```

---

## Task 6: De-shadcn — rewrite Switch and DropdownMenu

**Files:**
- Modify: `bohikor/src/components/ui/switch.tsx`
- Modify: `bohikor/src/components/ui/dropdown-menu.tsx`

**Interfaces:**
- Produces: `Switch(...React.ComponentProps<typeof SwitchPrimitive.Root>)` (drops the unused `size` prop — confirmed `settings/page.tsx` never passes `size` to `Switch`). `DropdownMenu`, `DropdownMenuTrigger`, `DropdownMenuContent({align?, sideOffset?, ...})`, `DropdownMenuItem({variant?: "default"|"destructive", ...})` — confirmed these four are the only exports `users/page.tsx` imports (`DropdownMenuCheckboxItem`, `RadioGroup`, `Label`, `Separator`, `Shortcut`, `Sub*` are unused and dropped).

- [ ] **Step 1: Rewrite switch.tsx**

```tsx
"use client";

import * as React from "react";
import { Switch as SwitchPrimitive } from "radix-ui";
import { cn } from "@/lib/utils";

function Switch({
  className,
  ...props
}: React.ComponentProps<typeof SwitchPrimitive.Root>) {
  return (
    <SwitchPrimitive.Root
      data-slot="switch"
      className={cn(
        "peer relative inline-flex h-[18.4px] w-[32px] shrink-0 items-center rounded-full border border-transparent outline-none transition-colors focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 data-checked:bg-primary data-unchecked:bg-input",
        className
      )}
      {...props}
    >
      <SwitchPrimitive.Thumb
        data-slot="switch-thumb"
        className="block size-4 rounded-full bg-background transition-transform data-checked:translate-x-[calc(100%-2px)] data-unchecked:translate-x-0"
      />
    </SwitchPrimitive.Root>
  );
}

export { Switch };
```

- [ ] **Step 2: Rewrite dropdown-menu.tsx**

```tsx
"use client";

import * as React from "react";
import { DropdownMenu as DropdownMenuPrimitive } from "radix-ui";
import { cn } from "@/lib/utils";

function DropdownMenu(
  props: React.ComponentProps<typeof DropdownMenuPrimitive.Root>
) {
  return <DropdownMenuPrimitive.Root data-slot="dropdown-menu" {...props} />;
}

function DropdownMenuTrigger(
  props: React.ComponentProps<typeof DropdownMenuPrimitive.Trigger>
) {
  return (
    <DropdownMenuPrimitive.Trigger data-slot="dropdown-menu-trigger" {...props} />
  );
}

function DropdownMenuContent({
  className,
  align = "start",
  sideOffset = 4,
  ...props
}: React.ComponentProps<typeof DropdownMenuPrimitive.Content>) {
  return (
    <DropdownMenuPrimitive.Portal>
      <DropdownMenuPrimitive.Content
        data-slot="dropdown-menu-content"
        sideOffset={sideOffset}
        align={align}
        className={cn(
          "z-50 min-w-32 overflow-hidden rounded-lg bg-popover p-1 text-popover-foreground shadow-md ring-1 ring-foreground/10",
          className
        )}
        {...props}
      />
    </DropdownMenuPrimitive.Portal>
  );
}

function DropdownMenuItem({
  className,
  variant = "default",
  ...props
}: React.ComponentProps<typeof DropdownMenuPrimitive.Item> & {
  variant?: "default" | "destructive";
}) {
  return (
    <DropdownMenuPrimitive.Item
      data-slot="dropdown-menu-item"
      data-variant={variant}
      className={cn(
        "relative flex cursor-default items-center gap-1.5 rounded-md px-1.5 py-1 text-sm outline-hidden select-none focus:bg-accent focus:text-accent-foreground data-[variant=destructive]:text-destructive data-[variant=destructive]:focus:bg-destructive/10",
        className
      )}
      {...props}
    />
  );
}

export { DropdownMenu, DropdownMenuTrigger, DropdownMenuContent, DropdownMenuItem };
```

- [ ] **Step 3: Verify build and typecheck**

Run: `cd bohikor && npm run typecheck && npm run build`
Expected: PASS. `npm run test` should also still be fully green (no test imports these files directly).

- [ ] **Step 4: Commit**

```bash
cd /Users/carlson/space/bohikor2
git add bohikor/src/components/ui/switch.tsx bohikor/src/components/ui/dropdown-menu.tsx
git commit -m "refactor(bohikor): restyle switch and dropdown-menu with plain Tailwind, keep Radix"
```

---

## Task 7: Frontend — subject hint storage, Admin type, api.ts redirect fix

**Files:**
- Modify: `bohikor/src/lib/auth.ts`
- Modify: `bohikor/src/types/index.ts:33-38`
- Modify: `bohikor/src/lib/api.ts:51,73`

**Interfaces:**
- Produces: `getSubjectHint(): SubjectHint | null`, `setSubjectHint(hint: SubjectHint): void`, `type SubjectHint = "admin" | "platform_admin"` — consumed by Task 8's `AuthProvider` and Task 9/11's login pages.
- Produces: `Admin.company_slug: string` — consumed by Task 9's company-context layout and login redirect.

- [ ] **Step 1: Add subject hint storage to lib/auth.ts**

Replace the full contents of `bohikor/src/lib/auth.ts`:

```ts
"use client";

const ACCESS_TOKEN_KEY = "bohikor2_access_token";
const REFRESH_TOKEN_KEY = "bohikor2_refresh_token";
const SUBJECT_HINT_KEY = "bohikor2_subject_hint";

export type SubjectHint = "admin" | "platform_admin";

export function getAccessToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem(ACCESS_TOKEN_KEY);
}

export function getRefreshToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem(REFRESH_TOKEN_KEY);
}

export function setTokens(accessToken: string, refreshToken: string): void {
  localStorage.setItem(ACCESS_TOKEN_KEY, accessToken);
  localStorage.setItem(REFRESH_TOKEN_KEY, refreshToken);
}

export function clearTokens(): void {
  localStorage.removeItem(ACCESS_TOKEN_KEY);
  localStorage.removeItem(REFRESH_TOKEN_KEY);
  localStorage.removeItem(SUBJECT_HINT_KEY);
}

export function getSubjectHint(): SubjectHint | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem(SUBJECT_HINT_KEY) as SubjectHint | null;
}

export function setSubjectHint(hint: SubjectHint): void {
  localStorage.setItem(SUBJECT_HINT_KEY, hint);
}
```

- [ ] **Step 2: Add company_slug to the Admin type**

In `bohikor/src/types/index.ts`, replace the `Admin` interface (lines 33-38):

```ts
export interface Admin {
  id: string;
  email: string;
  password_hash: string;
  company_slug: string;
  created_at: string;
}
```

- [ ] **Step 3: Fix the stale hardcoded /login redirect in api.ts**

There is no bare `/login` route in the new tree (only `/{company}/admin/login`, `/{company}/login`, `/platform/login`). In `bohikor/src/lib/api.ts`, change both occurrences of `window.location.href = "/login";` (lines 51 and 73) to:

```ts
              window.location.href = "/";
```

- [ ] **Step 4: Verify typecheck**

Run: `cd bohikor && npm run typecheck`
Expected: FAILS — `(auth)/login/page.tsx`'s test mock and `use-admin.ts` consumers referencing `Admin` are unaffected, but nothing yet sets `company_slug` at runtime; this is just a type addition so typecheck should actually PASS at this step (no consumer reads `.company_slug` yet — that's Task 9). Confirm PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/carlson/space/bohikor2
git add bohikor/src/lib/auth.ts bohikor/src/types/index.ts bohikor/src/lib/api.ts
git commit -m "feat(bohikor): add subject hint storage, company_slug type, fix stale /login redirect"
```

---

## Task 8: Frontend — AuthProvider subjectType union + hint-based loader

**Files:**
- Modify: `bohikor/src/components/providers/auth-provider.tsx`

**Interfaces:**
- Consumes: `getSubjectHint`, `setSubjectHint` (unused directly here, read-only consumer), `getAccessToken`, `clearTokens` from `@/lib/auth` (Task 7).
- Produces: `AuthContextType.subjectType: "admin" | "platform_admin" | null` (widened from `"admin" | null`) — consumed by Task 9's `[company]/layout.tsx`, `[company]/admin/(protected)/layout.tsx`, and Task 11's `platform/(protected)/layout.tsx`.

- [ ] **Step 1: Replace auth-provider.tsx**

```tsx
"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import { api } from "@/lib/api";
import { getAccessToken, getSubjectHint, clearTokens } from "@/lib/auth";
import type { Admin } from "@/types";

type SubjectType = "admin" | "platform_admin" | null;

interface AuthContextType {
  admin: Admin | null;
  subjectType: SubjectType;
  loading: boolean;
  signOut: () => Promise<void>;
  refreshSubject: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType>({
  admin: null,
  subjectType: null,
  loading: true,
  signOut: async () => {},
  refreshSubject: async () => {},
});

export function useAuth() {
  return useContext(AuthContext);
}

function useSubjectLoader() {
  const [admin, setAdmin] = useState<Admin | null>(null);
  const [subjectType, setSubjectType] = useState<SubjectType>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    const token = getAccessToken();
    const hint = getSubjectHint();

    if (!token || !hint) {
      setAdmin(null);
      setSubjectType(null);
      setLoading(false);
      return;
    }

    if (hint === "platform_admin") {
      // No /api/platform/me endpoint yet - trust the stored hint so the
      // platform shell can render; real profile loading lands with the
      // platform-console task.
      setAdmin(null);
      setSubjectType("platform_admin");
      setLoading(false);
      return;
    }

    try {
      const { data } = await api.get<{ data: Admin }>("/api/admin/me");
      setAdmin(data.data);
      setSubjectType("admin");
    } catch {
      clearTokens();
      setAdmin(null);
      setSubjectType(null);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load(); // eslint-disable-line react-hooks/set-state-in-effect -- auth initialization must fetch and set state on mount
  }, [load]);

  return { admin, subjectType, loading, load };
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const { admin, subjectType, loading, load: refreshSubject } = useSubjectLoader();

  const signOut = useCallback(async () => {
    try {
      await api.post("/api/auth/logout");
    } catch {
      // Ignore logout API errors
    }
    clearTokens();
  }, []);

  return (
    <AuthContext.Provider value={{ admin, subjectType, loading, signOut, refreshSubject }}>
      {children}
    </AuthContext.Provider>
  );
}
```

- [ ] **Step 2: Verify typecheck**

Run: `cd bohikor && npm run typecheck`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
cd /Users/carlson/space/bohikor2
git add bohikor/src/components/providers/auth-provider.tsx
git commit -m "feat(bohikor): widen AuthProvider to support platform_admin via subject hint"
```

---

## Task 9: Route restructure — company layout, admin login/protected split, move existing pages

**Files:**
- Create: `bohikor/src/app/[company]/layout.tsx`
- Create: `bohikor/src/components/auth-guard.tsx` (rewritten, generalized)
- Move: `bohikor/src/app/(auth)/login/page.tsx` → `bohikor/src/app/[company]/admin/login/page.tsx` (+ `__tests__`)
- Move: `bohikor/src/app/(main)/layout.tsx` → `bohikor/src/app/[company]/admin/(protected)/layout.tsx`
- Move: `bohikor/src/app/(main)/page.tsx` → `bohikor/src/app/[company]/admin/(protected)/page.tsx` (+ `__tests__`)
- Move: `bohikor/src/app/(main)/invite/page.tsx` → `bohikor/src/app/[company]/admin/(protected)/invite/page.tsx` (+ `__tests__`)
- Move: `bohikor/src/app/(main)/requests/page.tsx` → `bohikor/src/app/[company]/admin/(protected)/requests/page.tsx` (+ `__tests__`)
- Move: `bohikor/src/app/(main)/settings/page.tsx` → `bohikor/src/app/[company]/admin/(protected)/settings/page.tsx`
- Move: `bohikor/src/app/(main)/users/page.tsx` → `bohikor/src/app/[company]/admin/(protected)/users/page.tsx`
- Create: `bohikor/src/app/[company]/admin/(protected)/events/page.tsx`
- Create: `bohikor/src/app/[company]/admin/(protected)/balance/page.tsx`

**Interfaces:**
- Produces: `AuthGuard({children, loginHref}: {children: React.ReactNode; loginHref: string})` — a generalized "redirect to `loginHref` if unauthenticated" wrapper, consumed here and by Task 11's `platform/(protected)/layout.tsx`.
- Consumes: `useAuth()` (admin, subjectType, loading) from Task 8, `useAdmin()` from `bohikor/src/hooks/use-admin.ts` (unchanged).

- [ ] **Step 1: Move the admin login page and its test**

```bash
cd /Users/carlson/space/bohikor2/bohikor
mkdir -p "src/app/[company]/admin"
git mv "src/app/(auth)/login" "src/app/[company]/admin/login"
```

- [ ] **Step 2: Update the moved login page for slug-aware redirects and the subject hint**

Replace `bohikor/src/app/[company]/admin/login/page.tsx`:

```tsx
"use client";

import { useState, useEffect } from "react";
import { useParams, useRouter } from "next/navigation";
import { api } from "@/lib/api";
import { setTokens, setSubjectHint } from "@/lib/auth";
import { useAuth } from "@/components/providers";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { toast } from "sonner";

export default function AdminLoginPage() {
  const router = useRouter();
  const { company } = useParams<{ company: string }>();
  const { admin, refreshSubject } = useAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (admin) {
      router.replace(`/${company}/admin`);
    }
  }, [admin, company, router]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setLoading(true);

    try {
      const { data } = await api.post("/api/auth/admin/login", { email, password });
      setTokens(data.data.access_token, data.data.refresh_token);
      setSubjectHint("admin");
      await refreshSubject();
      toast.success("Signed in successfully");
      router.push(`/${company}/admin`);
    } catch (err: unknown) {
      const message =
        err && typeof err === "object" && "response" in err
          ? (err as { response?: { data?: { error?: string } } }).response?.data?.error || "Invalid email or password"
          : "Failed to sign in";
      setError(message);
      toast.error("Sign in failed");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-muted/50 p-4">
      <Card className="w-full max-w-md">
        <CardHeader className="space-y-1">
          <CardTitle className="text-2xl font-bold">Bohikor2 Admin</CardTitle>
          <CardDescription>
            Enter your credentials to access the admin dashboard
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            {error && (
              <Alert variant="destructive">
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            )}

            <div className="space-y-2">
              <Label htmlFor="email">Email</Label>
              <Input
                id="email"
                type="email"
                placeholder="admin@bohikor2.com"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                autoComplete="email"
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="password">Password</Label>
              <Input
                id="password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                autoComplete="current-password"
              />
            </div>

            <Button type="submit" className="w-full" disabled={loading}>
              {loading ? "Signing in..." : "Sign In"}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
```

- [ ] **Step 3: Create the company-context layout**

Create `bohikor/src/app/[company]/layout.tsx`:

```tsx
"use client";

import { useEffect } from "react";
import { useParams, useRouter } from "next/navigation";
import { useAuth } from "@/components/providers";

export default function CompanyLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const { company } = useParams<{ company: string }>();
  const { admin, subjectType, loading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (loading || subjectType !== "admin" || !admin) return;
    if (admin.company_slug !== company) {
      router.replace(`/${admin.company_slug}/admin`);
    }
  }, [loading, subjectType, admin, company, router]);

  return <>{children}</>;
}
```

- [ ] **Step 4: Rewrite auth-guard.tsx as a generalized guard**

Replace `bohikor/src/components/auth-guard.tsx`:

```tsx
"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/components/providers";

export function AuthGuard({
  children,
  loginHref,
}: {
  children: React.ReactNode;
  loginHref: string;
}) {
  const { subjectType, loading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!loading && !subjectType) {
      router.push(loginHref);
    }
  }, [subjectType, loading, loginHref, router]);

  if (loading) {
    return (
      <div className="flex h-screen items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
      </div>
    );
  }

  if (!subjectType) {
    return null;
  }

  return <>{children}</>;
}
```

- [ ] **Step 5: Move the guarded pages into `[company]/admin/(protected)/`**

```bash
cd /Users/carlson/space/bohikor2/bohikor
mkdir -p "src/app/[company]/admin/(protected)"
git mv "src/app/(main)/layout.tsx" "src/app/[company]/admin/(protected)/layout.tsx"
git mv "src/app/(main)/page.tsx" "src/app/[company]/admin/(protected)/page.tsx"
git mv "src/app/(main)/__tests__" "src/app/[company]/admin/(protected)/__tests__"
git mv "src/app/(main)/invite" "src/app/[company]/admin/(protected)/invite"
git mv "src/app/(main)/requests" "src/app/[company]/admin/(protected)/requests"
git mv "src/app/(main)/settings" "src/app/[company]/admin/(protected)/settings"
git mv "src/app/(main)/users" "src/app/[company]/admin/(protected)/users"
rmdir "src/app/(main)" "src/app/(auth)"
```

- [ ] **Step 6: Rewrite the moved (protected) layout**

Replace `bohikor/src/app/[company]/admin/(protected)/layout.tsx`:

```tsx
"use client";

import { useParams } from "next/navigation";
import { AuthGuard } from "@/components/auth-guard";
import { useAdmin } from "@/hooks/use-admin";
import { ForbiddenPage } from "@/components/forbidden";
import { Sidebar } from "@/components/sidebar";

export default function CompanyAdminLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const { company } = useParams<{ company: string }>();

  return (
    <AuthGuard loginHref={`/${company}/admin/login`}>
      <AdminCheck company={company}>{children}</AdminCheck>
    </AuthGuard>
  );
}

function AdminCheck({
  children,
  company,
}: {
  children: React.ReactNode;
  company: string;
}) {
  const { data: admin, isLoading } = useAdmin();

  if (isLoading) {
    return (
      <div className="flex h-screen items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
      </div>
    );
  }

  if (!admin) {
    return (
      <ForbiddenPage
        backHref={`/${company}/admin/login`}
        backLabel="Go to Login"
      />
    );
  }

  return (
    <div className="flex h-screen">
      <Sidebar />
      <main className="flex-1 overflow-y-auto p-8">{children}</main>
    </div>
  );
}
```

- [ ] **Step 7: Create the two new admin pages**

Create `bohikor/src/app/[company]/admin/(protected)/events/page.tsx`:

```tsx
export default function EventsPage() {
  return (
    <div className="p-4">
      <p className="text-muted-foreground">Events — coming soon.</p>
    </div>
  );
}
```

Create `bohikor/src/app/[company]/admin/(protected)/balance/page.tsx`:

```tsx
export default function BalancePage() {
  return (
    <div className="p-4">
      <p className="text-muted-foreground">Balance &amp; ledger — coming soon.</p>
    </div>
  );
}
```

- [ ] **Step 8: Verify typecheck**

Run: `cd bohikor && npm run typecheck`
Expected: FAILS on `sidebar.tsx` (still uses static `href="/"` etc. — fixed in Task 10) and on the moved login test (still asserts `mockPush` called with `"/"` — fixed in Task 13). Confirm these are the *only* failures.

- [ ] **Step 9: Commit**

```bash
cd /Users/carlson/space/bohikor2
git add bohikor/src/app bohikor/src/components/auth-guard.tsx
git commit -m "refactor(bohikor): restructure routes under [company]/admin, add company-context layout"
```

---

## Task 10: Route restructure — sidebar and forbidden backHref

**Files:**
- Modify: `bohikor/src/components/sidebar.tsx`

**Interfaces:**
- Consumes: `useParams<{ company: string }>()` from `next/navigation`.
- No change to `bohikor/src/components/forbidden.tsx` — its `backHref`/`backLabel` props are already parameterized; Task 9 already passes the slug-aware value at the one call site.

- [ ] **Step 1: Rewrite sidebar.tsx with slug-prefixed hrefs and the two new nav entries**

```tsx
"use client";

import Link from "next/link";
import Image from "next/image";
import { usePathname, useParams } from "next/navigation";
import { cn } from "@/lib/utils";
import { useAuth } from "@/components/providers";
import {
  LayoutDashboard,
  Mail,
  Users,
  ArrowLeftRight,
  Activity,
  Wallet,
  Settings,
  LogOut,
} from "lucide-react";

export function Sidebar() {
  const pathname = usePathname();
  const { company } = useParams<{ company: string }>();
  const { signOut } = useAuth();

  const navItems = [
    { href: `/${company}/admin`, label: "Dashboard", icon: LayoutDashboard },
    { href: `/${company}/admin/invite`, label: "Invite", icon: Mail },
    { href: `/${company}/admin/users`, label: "Users", icon: Users },
    { href: `/${company}/admin/requests`, label: "Requests", icon: ArrowLeftRight },
    { href: `/${company}/admin/events`, label: "Events", icon: Activity },
    { href: `/${company}/admin/balance`, label: "Balance", icon: Wallet },
    { href: `/${company}/admin/settings`, label: "Settings", icon: Settings },
  ];

  return (
    <aside className="flex h-screen w-64 flex-col border-r bg-sidebar">
      <div className="flex h-16 items-center gap-2 border-b px-4">
        <Image
          src="/logo.png"
          alt="Bohikor"
          width={32}
          height={32}
          className="shrink-0"
        />
        <h1 className="text-lg font-semibold text-sidebar-foreground">
          Bohikor
        </h1>
      </div>

      <nav className="flex-1 space-y-1 px-3 py-4">
        {navItems.map((item) => {
          const Icon = item.icon;
          const isActive = pathname === item.href;

          return (
            <Link
              key={item.href}
              href={item.href}
              className={cn(
                "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
                isActive
                  ? "bg-sidebar-primary text-sidebar-primary-foreground"
                  : "text-sidebar-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
              )}
            >
              <Icon className="h-5 w-5" />
              {item.label}
            </Link>
          );
        })}
      </nav>

      <div className="border-t p-4">
        <button
          onClick={() => signOut()}
          className="flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm font-medium text-sidebar-foreground transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
        >
          <LogOut className="h-5 w-5" />
          Sign Out
        </button>
      </div>
    </aside>
  );
}
```

- [ ] **Step 2: Verify typecheck**

Run: `cd bohikor && npm run typecheck`
Expected: FAILS only on the moved login test now (Task 13 fixes it). Confirm no other errors.

- [ ] **Step 3: Commit**

```bash
cd /Users/carlson/space/bohikor2
git add bohikor/src/components/sidebar.tsx
git commit -m "feat(bohikor): slug-prefix sidebar nav links, add events/balance entries"
```

---

## Task 11: Route restructure — platform login/protected stub

**Files:**
- Create: `bohikor/src/app/platform/login/page.tsx`
- Create: `bohikor/src/app/platform/(protected)/layout.tsx`
- Create: `bohikor/src/app/platform/(protected)/page.tsx`

**Interfaces:**
- Consumes: `AuthGuard` (Task 9), `setTokens`/`setSubjectHint` (Task 7), `useAuth()` (Task 8).

- [ ] **Step 1: Create the platform login page**

```tsx
"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api";
import { setTokens, setSubjectHint } from "@/lib/auth";
import { useAuth } from "@/components/providers";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { toast } from "sonner";

export default function PlatformLoginPage() {
  const router = useRouter();
  const { subjectType, refreshSubject } = useAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (subjectType === "platform_admin") {
      router.replace("/platform");
    }
  }, [subjectType, router]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setLoading(true);

    try {
      const { data } = await api.post("/api/auth/platform/login", { email, password });
      setTokens(data.data.access_token, data.data.refresh_token);
      setSubjectHint("platform_admin");
      await refreshSubject();
      toast.success("Signed in successfully");
      router.push("/platform");
    } catch (err: unknown) {
      const message =
        err && typeof err === "object" && "response" in err
          ? (err as { response?: { data?: { error?: string } } }).response?.data?.error || "Invalid email or password"
          : "Failed to sign in";
      setError(message);
      toast.error("Sign in failed");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-muted/50 p-4">
      <Card className="w-full max-w-md">
        <CardHeader className="space-y-1">
          <CardTitle className="text-2xl font-bold">Platform Admin</CardTitle>
          <CardDescription>Sign in as a platform administrator</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            {error && (
              <Alert variant="destructive">
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            )}

            <div className="space-y-2">
              <Label htmlFor="email">Email</Label>
              <Input
                id="email"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                autoComplete="email"
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="password">Password</Label>
              <Input
                id="password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                autoComplete="current-password"
              />
            </div>

            <Button type="submit" className="w-full" disabled={loading}>
              {loading ? "Signing in..." : "Sign In"}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
```

- [ ] **Step 2: Create the platform protected layout and stub page**

Create `bohikor/src/app/platform/(protected)/layout.tsx`:

```tsx
"use client";

import { AuthGuard } from "@/components/auth-guard";

export default function PlatformLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return <AuthGuard loginHref="/platform/login">{children}</AuthGuard>;
}
```

Create `bohikor/src/app/platform/(protected)/page.tsx`:

```tsx
export default function PlatformConsolePage() {
  return (
    <div className="flex min-h-screen items-center justify-center p-4">
      <p className="text-muted-foreground">Platform console — coming soon.</p>
    </div>
  );
}
```

- [ ] **Step 3: Verify typecheck**

Run: `cd bohikor && npm run typecheck`
Expected: FAILS only on the still-unfixed moved login test (Task 13). Confirm no new errors from these three files.

- [ ] **Step 4: Commit**

```bash
cd /Users/carlson/space/bohikor2
git add "bohikor/src/app/platform"
git commit -m "feat(bohikor): add platform login and protected console shell"
```

---

## Task 12: Route restructure — remaining employee/landing stub pages

**Files:**
- Create: `bohikor/src/app/page.tsx`
- Create: `bohikor/src/app/[company]/page.tsx`
- Create: `bohikor/src/app/[company]/login/page.tsx`
- Create: `bohikor/src/app/[company]/signup/page.tsx`
- Create: `bohikor/src/app/[company]/verify/page.tsx`
- Create: `bohikor/src/app/[company]/create-pin/page.tsx`
- Create: `bohikor/src/app/[company]/forgot-pin/page.tsx`
- Create: `bohikor/src/app/[company]/reset-pin/page.tsx`
- Create: `bohikor/src/app/[company]/history/page.tsx`
- Create: `bohikor/src/app/[company]/account/page.tsx`

**Interfaces:** None — pure placeholder Server Components, no props, no state. Real logic for these lands in Epic 8 tasks 2-3.

- [ ] **Step 1: Create the root landing stub**

```tsx
export default function LandingPage() {
  return (
    <div className="flex min-h-screen items-center justify-center p-4">
      <p className="text-muted-foreground">Sign in — coming soon.</p>
    </div>
  );
}
```
Path: `bohikor/src/app/page.tsx`

- [ ] **Step 2: Create the employee home stub**

```tsx
export default function EmployeeHomePage() {
  return (
    <div className="flex min-h-screen items-center justify-center p-4">
      <p className="text-muted-foreground">Employee home — coming soon.</p>
    </div>
  );
}
```
Path: `bohikor/src/app/[company]/page.tsx`

- [ ] **Step 3: Create the employee login stub**

```tsx
export default function EmployeeLoginPage() {
  return (
    <div className="flex min-h-screen items-center justify-center p-4">
      <p className="text-muted-foreground">Employee sign in — coming soon.</p>
    </div>
  );
}
```
Path: `bohikor/src/app/[company]/login/page.tsx`

- [ ] **Step 4: Create the signup stub**

```tsx
export default function SignupPage() {
  return (
    <div className="flex min-h-screen items-center justify-center p-4">
      <p className="text-muted-foreground">Sign up — coming soon.</p>
    </div>
  );
}
```
Path: `bohikor/src/app/[company]/signup/page.tsx`

- [ ] **Step 5: Create the verify stub**

```tsx
export default function VerifyPage() {
  return (
    <div className="flex min-h-screen items-center justify-center p-4">
      <p className="text-muted-foreground">Verify — coming soon.</p>
    </div>
  );
}
```
Path: `bohikor/src/app/[company]/verify/page.tsx`

- [ ] **Step 6: Create the create-pin stub**

```tsx
export default function CreatePinPage() {
  return (
    <div className="flex min-h-screen items-center justify-center p-4">
      <p className="text-muted-foreground">Create PIN — coming soon.</p>
    </div>
  );
}
```
Path: `bohikor/src/app/[company]/create-pin/page.tsx`

- [ ] **Step 7: Create the forgot-pin stub**

```tsx
export default function ForgotPinPage() {
  return (
    <div className="flex min-h-screen items-center justify-center p-4">
      <p className="text-muted-foreground">Forgot PIN — coming soon.</p>
    </div>
  );
}
```
Path: `bohikor/src/app/[company]/forgot-pin/page.tsx`

- [ ] **Step 8: Create the reset-pin stub**

```tsx
export default function ResetPinPage() {
  return (
    <div className="flex min-h-screen items-center justify-center p-4">
      <p className="text-muted-foreground">Reset PIN — coming soon.</p>
    </div>
  );
}
```
Path: `bohikor/src/app/[company]/reset-pin/page.tsx`

- [ ] **Step 9: Create the history stub**

```tsx
export default function HistoryPage() {
  return (
    <div className="flex min-h-screen items-center justify-center p-4">
      <p className="text-muted-foreground">History — coming soon.</p>
    </div>
  );
}
```
Path: `bohikor/src/app/[company]/history/page.tsx`

- [ ] **Step 10: Create the account stub**

```tsx
export default function AccountPage() {
  return (
    <div className="flex min-h-screen items-center justify-center p-4">
      <p className="text-muted-foreground">Account — coming soon.</p>
    </div>
  );
}
```
Path: `bohikor/src/app/[company]/account/page.tsx`

- [ ] **Step 11: Verify typecheck and build**

Run: `cd bohikor && npm run typecheck && npm run build`
Expected: `typecheck` still fails only on the moved login test (Task 13 fixes it next). `build` should succeed — Next.js's route tree is now structurally complete per PLAN.md's Epic 8 task 1 target.

- [ ] **Step 12: Commit**

```bash
cd /Users/carlson/space/bohikor2
git add "bohikor/src/app/page.tsx" "bohikor/src/app/[company]"
git commit -m "feat(bohikor): add remaining employee/landing stub routes"
```

---

## Task 13: Fix moved tests

**Files:**
- Modify: `bohikor/src/app/[company]/admin/login/__tests__/page.test.tsx`

**Interfaces:** None — test-only changes, no production code.

- [ ] **Step 1: Update the moved login test's mocks and assertion**

Replace `bohikor/src/app/[company]/admin/login/__tests__/page.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import AdminLoginPage from "../page";

const mockPush = jest.fn();
const mockReplace = jest.fn();
const mockRefreshSubject = jest.fn();

jest.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, replace: mockReplace }),
  useParams: () => ({ company: "acme" }),
}));

jest.mock("@/components/providers", () => ({
  useAuth: jest.fn(),
}));

jest.mock("@/lib/api", () => ({
  api: {
    post: jest.fn(),
  },
}));

jest.mock("@/lib/auth", () => ({
  setTokens: jest.fn(),
  setSubjectHint: jest.fn(),
}));

const { useAuth } = jest.requireMock("@/components/providers");
const { api } = jest.requireMock("@/lib/api");
const { setTokens, setSubjectHint } = jest.requireMock("@/lib/auth");

describe("AdminLoginPage", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    useAuth.mockReturnValue({
      admin: null,
      subjectType: null,
      loading: false,
      signOut: jest.fn(),
      refreshSubject: mockRefreshSubject,
    });
  });

  it("renders admin login form", () => {
    render(<AdminLoginPage />);
    expect(screen.getByText("Bohikor2 Admin")).toBeInTheDocument();
    expect(
      screen.getByText(/enter your credentials/i)
    ).toBeInTheDocument();
    expect(screen.getByLabelText(/email/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/password/i)).toBeInTheDocument();
    expect(screen.getByText("Sign In")).toBeInTheDocument();
  });

  it("calls api post on form submit", async () => {
    api.post.mockResolvedValue({
      data: { data: { access_token: "at", refresh_token: "rt" } },
    });
    mockRefreshSubject.mockResolvedValue(undefined);

    const user = userEvent.setup();
    render(<AdminLoginPage />);

    await user.type(screen.getByLabelText(/email/i), "admin@example.com");
    await user.type(screen.getByLabelText(/password/i), "password123");
    await user.click(screen.getByText("Sign In"));

    expect(api.post).toHaveBeenCalledWith("/api/auth/admin/login", {
      email: "admin@example.com",
      password: "password123",
    });
  });

  it("redirects to the company admin dashboard on success", async () => {
    api.post.mockResolvedValue({
      data: { data: { access_token: "at", refresh_token: "rt" } },
    });
    mockRefreshSubject.mockResolvedValue(undefined);

    const user = userEvent.setup();
    render(<AdminLoginPage />);

    await user.type(screen.getByLabelText(/email/i), "admin@example.com");
    await user.type(screen.getByLabelText(/password/i), "password123");
    await user.click(screen.getByText("Sign In"));

    expect(setTokens).toHaveBeenCalledWith("at", "rt");
    expect(setSubjectHint).toHaveBeenCalledWith("admin");
    expect(mockPush).toHaveBeenCalledWith("/acme/admin");
  });

  it("shows error on invalid credentials", async () => {
    api.post.mockRejectedValue({
      response: { data: { error: "Invalid email or password" } },
    });

    const user = userEvent.setup();
    render(<AdminLoginPage />);

    await user.type(screen.getByLabelText(/email/i), "wrong@example.com");
    await user.type(screen.getByLabelText(/password/i), "wrong");
    await user.click(screen.getByText("Sign In"));

    expect(
      screen.getByText("Invalid email or password")
    ).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run the full frontend test suite**

Run: `cd bohikor && npm run test`
Expected: PASS — this test suite plus the moved dashboard/invite/requests suites (which don't assert paths, confirmed via grep, so they pass on import-path changes alone).

- [ ] **Step 3: Run typecheck, lint, and build**

Run: `cd bohikor && npm run typecheck && npm run lint && npm run build`
Expected: All PASS. This is the first point where every prior task's loose ends (sidebar hrefs, login test, stub types) are simultaneously resolved.

- [ ] **Step 4: Commit**

```bash
cd /Users/carlson/space/bohikor2
git add "bohikor/src/app/[company]/admin/login/__tests__/page.test.tsx"
git commit -m "test(bohikor): update moved login test for slug-aware redirect and subject hint"
```

---

## Task 14: Final verification pass

**Files:** None modified — this task only runs checks.

- [ ] **Step 1: Backend full suite**

Run: `cd backend && go test ./...`
Expected: PASS.

- [ ] **Step 2: Frontend full suite**

Run: `cd bohikor && npm run typecheck && npm run lint && npm run test && npm run build`
Expected: All PASS.

- [ ] **Step 3: Manual dev-server spot check**

Run: `cd bohikor && npm run dev` (requires `backend` running separately with a seeded company whose slug is known, e.g. `acme` — check `backend/db` seed data or create one via the platform API/DB directly if none exists).

Visit and confirm no route 500s/404s from misconfiguration (stubs rendering "coming soon" text is success; a Next.js error overlay is not):
- `http://localhost:3000/` — landing stub renders
- `http://localhost:3000/acme/admin/login` — login form renders, and successfully signing in with a seeded admin redirects to `/acme/admin` showing the dashboard with sidebar
- `http://localhost:3000/acme/history` — employee stub renders
- `http://localhost:3000/platform/login` — platform login form renders
- Visiting `/acme/admin` directly while signed out redirects to `/acme/admin/login` (not a redirect loop)

Stop the dev server when done (`Ctrl+C`).

- [ ] **Step 4: Push branch (do not merge without user review)**

```bash
cd /Users/carlson/space/bohikor2
git push -u origin epic-8-unified-web-app
```

This is the end of Epic 8 task 1. Tasks 2-7 (shared data layer extension, employee flow porting, company admin feature work, platform console, invite emails, tests, visual design pass) are separate follow-up work, not part of this plan.
