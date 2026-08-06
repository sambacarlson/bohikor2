# Bohikor UI Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the shared design-system foundation from `docs/superpowers/specs/2026-08-05-bohikor-ui-overhaul-design.md` — re-themed color tokens (dark-default "Tinted Slate" / light "Cool Tinted"), the display typeface, a working dark/light toggle, a reusable adaptive modal, and a collapsible icon-rail sidebar — then prove it end-to-end by migrating one real screen (the employee home page's request-confirmation modal) off its old hardcoded, dark-mode-breaking markup and onto the new components.

**Architecture:** Pure `bohikor/` frontend work, token/component-layer only. Because the existing app already routes almost all color through CSS custom properties consumed via Tailwind semantic classes (`bg-card`, `text-muted-foreground`, `bg-sidebar`, etc.), re-theming `globals.css` re-skins every existing screen (employee, company admin, platform admin) simultaneously without touching their page files — this plan only touches page-level JSX where new *interactive* behavior (theme toggle, collapsible rail, adaptive dialog) needs wiring in, not for recoloring.

**Tech Stack:** Next.js 16 / React 19 / Tailwind v4 (existing), `radix-ui` (existing, used for the new `Dialog`), `next-themes` (existing dependency, currently unused — this plan wires it up), `motion` (new dependency, added in Task 8).

## Global Constraints

- Backend is untouched. No API, schema, or contract changes in this plan or any follow-on UI plan.
- Theme persistence is `localStorage` only via `next-themes` — no backend field, no new API call.
- Stay on bare `radix-ui` + Tailwind v4. Do not reintroduce a shadcn CLI scaffold or swap to a different component-kit/framework.
- No charting/data-visualization library.
- The only new npm dependency introduced across this plan is `motion` (installed in Task 8).
- `make test`-equivalent for this workspace is `npm test` (Jest) — global coverage threshold is 85% (statements/branches/functions/lines) per `bohikor/jest.config.js`(customJestConfig). Every new file needs tests that exercise it; `app/layout.tsx` is excluded from coverage instrumentation already, so tasks that only touch it lean on `npm run build` instead of a coverage-counted Jest test.
- Run `npm run lint`, `npm run typecheck`, and `npm test` from `bohikor/` before each commit in this plan; all three must pass.

---

## File Structure

New files this plan creates:
- `bohikor/src/hooks/use-media-query.ts` + `__tests__/use-media-query.test.ts` — viewport breakpoint detection, used by the adaptive `Dialog`.
- `bohikor/src/hooks/use-sidebar-collapse.ts` + `__tests__/use-sidebar-collapse.test.ts` — `localStorage`-persisted collapse state, used by both sidebars.
- `bohikor/src/components/theme-toggle.tsx` + `__tests__/theme-toggle.test.tsx` — dark/light toggle button, used by both sidebars (employee app wiring comes in the next plan).
- `bohikor/src/components/ui/dialog.tsx` + `ui/__tests__/dialog.test.tsx` — adaptive modal (bottom sheet on mobile, centered dialog on desktop), replaces ad-hoc modal markup app-wide going forward.

Existing files this plan modifies:
- `bohikor/src/app/globals.css` — token values for both themes, two new tokens, four new `@keyframes` for the dialog.
- `bohikor/src/components/ui/card.tsx` — one radius class bump.
- `bohikor/src/app/layout.tsx` — adds the display font and the `next-themes` `ThemeProvider`.
- `bohikor/src/components/providers/index.tsx` — re-exports `ThemeProvider`.
- `bohikor/src/components/sidebar.tsx` + its existing test — collapsible icon rail.
- `bohikor/src/components/platform-sidebar.tsx` + its existing test — collapsible icon rail.
- `bohikor/src/app/[company]/(protected)/page.tsx` — swaps the hardcoded `bg-white` modal for `Dialog`.

---

### Task 1: Re-theme design tokens

**Files:**
- Modify: `bohikor/src/app/globals.css`
- Modify: `bohikor/src/components/ui/card.tsx`

**Interfaces:**
- Consumes: nothing new.
- Produces: token values `--background`, `--foreground`, `--card`, `--card-elevated` (new), `--popover`, `--primary`, `--primary-foreground`, `--secondary`, `--muted`, `--accent`, `--border`, `--border-strong` (new), `--input`, `--ring`, `--sidebar*` for both `:root` (light) and `.dark`; Tailwind utility classes `bg-card-elevated` and `border-border-strong`/`bg-border-strong` become available app-wide via the `@theme inline` block. Every later task that styles a card/sidebar/border relies on these existing.

This task is CSS-value-only — no new component logic, so it's verified by a full build rather than a unit test (there is nothing here for Jest's DOM environment to meaningfully exercise; a snapshot test of raw CSS values would just restate the file).

- [ ] **Step 1: Confirm no test currently pins `card.tsx`'s radius class**

Run: `grep -rn "rounded-xl\|rounded-2xl" bohikor/src --include="*.test.tsx"`
Expected: no output (already verified during planning — this re-confirms nothing changed underneath you).

- [ ] **Step 2: Replace the `:root` block in `globals.css`**

Find the existing `:root { ... }` block (light theme values) and replace its contents with the "Cool Tinted" palette:

```css
:root {
  --background: #f4f6fa;
  --foreground: #0f1b2e;
  --card: #ffffff;
  --card-foreground: #0f1b2e;
  --card-elevated: #ffffff;
  --popover: #ffffff;
  --popover-foreground: #0f1b2e;
  --primary: #16a34a;
  --primary-foreground: #052e12;
  --secondary: #eef1f7;
  --secondary-foreground: #0f1b2e;
  --muted: #eef1f7;
  --muted-foreground: #5c6a82;
  --accent: #e7ebf3;
  --accent-foreground: #0f1b2e;
  --destructive: oklch(0.577 0.245 27.325);
  --border: #dde3ee;
  --border-strong: #c8d0e0;
  --input: #dde3ee;
  --ring: #16a34a;
  --chart-1: oklch(0.87 0 0);
  --chart-2: oklch(0.556 0 0);
  --chart-3: oklch(0.439 0 0);
  --chart-4: oklch(0.371 0 0);
  --chart-5: oklch(0.269 0 0);
  --radius: 0.625rem;
  --sidebar: #ffffff;
  --sidebar-foreground: #0f1b2e;
  --sidebar-primary: #16a34a;
  --sidebar-primary-foreground: #ffffff;
  --sidebar-accent: #eef1f7;
  --sidebar-accent-foreground: #0f1b2e;
  --sidebar-border: #dde3ee;
  --sidebar-ring: #16a34a;
}
```

- [ ] **Step 3: Replace the `.dark` block in `globals.css`**

Replace the existing `.dark { ... }` block with the "Tinted Slate" palette:

```css
.dark {
  --background: #0b1220;
  --foreground: #f1f5f9;
  --card: #111a2d;
  --card-foreground: #f1f5f9;
  --card-elevated: #151f36;
  --popover: #151f36;
  --popover-foreground: #f1f5f9;
  --primary: #22c55e;
  --primary-foreground: #052e12;
  --secondary: #1a2540;
  --secondary-foreground: #e2e8f0;
  --muted: #16213a;
  --muted-foreground: #8b98ac;
  --accent: #1a2540;
  --accent-foreground: #f1f5f9;
  --destructive: oklch(0.704 0.191 22.216);
  --border: #1e2a45;
  --border-strong: #22304a;
  --input: #1e2a45;
  --ring: #22c55e;
  --chart-1: oklch(0.87 0 0);
  --chart-2: oklch(0.556 0 0);
  --chart-3: oklch(0.439 0 0);
  --chart-4: oklch(0.371 0 0);
  --chart-5: oklch(0.269 0 0);
  --sidebar: #0d1526;
  --sidebar-foreground: #f1f5f9;
  --sidebar-primary: #22c55e;
  --sidebar-primary-foreground: #052e12;
  --sidebar-accent: #1a2540;
  --sidebar-accent-foreground: #f1f5f9;
  --sidebar-border: #1e2a45;
  --sidebar-ring: #22c55e;
}
```

- [ ] **Step 4: Register the two new tokens in the `@theme inline` block**

In `globals.css`, inside the existing `@theme inline { ... }` block, add these two lines next to the other `--color-card*`/`--color-border` entries:

```css
  --color-card-elevated: var(--card-elevated);
  --color-border-strong: var(--border-strong);
```

- [ ] **Step 5: Add the dialog keyframes**

Append this block to `globals.css`, after the existing `@layer base { ... }` block (these are consumed by Task 7's `Dialog` component):

```css
@keyframes dialog-overlay-in {
  from { opacity: 0; }
  to { opacity: 1; }
}
@keyframes dialog-overlay-out {
  from { opacity: 1; }
  to { opacity: 0; }
}
@keyframes dialog-scale-in {
  from { opacity: 0; transform: translate(-50%, -50%) scale(0.96); }
  to { opacity: 1; transform: translate(-50%, -50%) scale(1); }
}
@keyframes dialog-scale-out {
  from { opacity: 1; transform: translate(-50%, -50%) scale(1); }
  to { opacity: 0; transform: translate(-50%, -50%) scale(0.96); }
}
@keyframes dialog-sheet-in {
  from { opacity: 0; transform: translateY(16px); }
  to { opacity: 1; transform: translateY(0); }
}
@keyframes dialog-sheet-out {
  from { opacity: 1; transform: translateY(0); }
  to { opacity: 0; transform: translateY(16px); }
}
```

- [ ] **Step 6: Bump the card radius**

In `bohikor/src/components/ui/card.tsx`, change the `rounded-xl` class to `rounded-2xl` in the root card className string (line ~13: `"flex flex-col gap-4 rounded-xl bg-card ..."` → `"flex flex-col gap-4 rounded-2xl bg-card ..."`).

- [ ] **Step 7: Verify the build compiles**

Run: `cd bohikor && npm run build`
Expected: build succeeds with no Tailwind/PostCSS/TypeScript errors.

- [ ] **Step 8: Verify no existing tests broke**

Run: `cd bohikor && npm test`
Expected: all existing suites still pass (token/radius changes don't touch class names or text that tests assert on).

- [ ] **Step 9: Commit**

```bash
git add bohikor/src/app/globals.css bohikor/src/components/ui/card.tsx
git commit -m "style: re-theme tokens to Tinted Slate (dark) / Cool Tinted (light)"
```

---

### Task 2: Add Bricolage Grotesque display font

**Files:**
- Modify: `bohikor/src/app/layout.tsx`
- Modify: `bohikor/src/app/globals.css`

**Interfaces:**
- Consumes: nothing new.
- Produces: a `font-heading` Tailwind utility class that renders Bricolage Grotesque, available to every future task/plan that styles headings and hero numbers (not yet applied to any JSX in this plan — that's per-screen work for the next plan).

- [ ] **Step 1: Import and configure the font in `layout.tsx`**

In `bohikor/src/app/layout.tsx`, add the import alongside the existing `Geist`/`Geist_Mono` import:

```tsx
import { Geist, Geist_Mono, Bricolage_Grotesque } from "next/font/google";
```

Add the font instance next to `geistSans`/`geistMono`:

```tsx
const bricolageGrotesque = Bricolage_Grotesque({
  variable: "--font-bricolage",
  subsets: ["latin"],
  weight: ["600", "700"],
});
```

- [ ] **Step 2: Add the font variable to the `<html>` className**

Change:

```tsx
className={`${geistSans.variable} ${geistMono.variable} h-full antialiased`}
```

to:

```tsx
className={`${geistSans.variable} ${geistMono.variable} ${bricolageGrotesque.variable} h-full antialiased`}
```

- [ ] **Step 3: Repoint the `--font-heading` token**

In `bohikor/src/app/globals.css`, inside `@theme inline`, change:

```css
  --font-heading: var(--font-sans);
```

to:

```css
  --font-heading: var(--font-bricolage);
```

- [ ] **Step 4: Verify the build compiles and loads the font**

Run: `cd bohikor && npm run build`
Expected: build succeeds. `next/font/google` fails the build at compile time if the font name/weight combination is invalid, so a successful build confirms `Bricolage_Grotesque` with weights `600`/`700` is a valid Google Fonts request.

(`layout.tsx` is excluded from Jest coverage instrumentation via `jest.config.js`'s `collectCoverageFrom`, and font loading is a Next.js build-time concern that jsdom can't meaningfully exercise — the build check above is the real verification for this task.)

- [ ] **Step 5: Commit**

```bash
git add bohikor/src/app/layout.tsx bohikor/src/app/globals.css
git commit -m "style: add Bricolage Grotesque as the display/heading font"
```

---

### Task 3: Wire the dark-default theme provider

**Files:**
- Modify: `bohikor/src/components/providers/index.tsx`
- Modify: `bohikor/src/app/layout.tsx`
- Test: `bohikor/src/components/providers/__tests__/index.test.tsx` (new)

**Interfaces:**
- Consumes: `next-themes` (already in `package.json`).
- Produces: `ThemeProvider` re-exported from `@/components/providers`, wrapping the whole app with `attribute="class"`, `defaultTheme="dark"`, `enableSystem={false}` — so `<html>` gets a `dark` or `light` class matching `globals.css`'s existing `.dark` selector. Task 4 (`ThemeToggle`) and every later theme-aware component rely on this being mounted at the root.

- [ ] **Step 1: Write the failing test**

Create `bohikor/src/components/providers/__tests__/index.test.tsx`:

```tsx
import { ThemeProvider } from "../index";

describe("providers barrel", () => {
  it("re-exports ThemeProvider from next-themes", () => {
    expect(typeof ThemeProvider).toBe("function");
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd bohikor && npx jest src/components/providers/__tests__/index.test.tsx`
Expected: FAIL — `ThemeProvider` is not exported from `../index`.

- [ ] **Step 3: Add the re-export**

In `bohikor/src/components/providers/index.tsx`, add:

```tsx
export { ThemeProvider } from "next-themes";
```

so the file reads:

```tsx
export { ReactQueryProvider } from "./react-query-provider";
export { AuthProvider, useAuth } from "./auth-provider";
export { ThemeProvider } from "next-themes";
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd bohikor && npx jest src/components/providers/__tests__/index.test.tsx`
Expected: PASS

- [ ] **Step 5: Wire `ThemeProvider` into the root layout**

In `bohikor/src/app/layout.tsx`, update the import:

```tsx
import { AuthProvider, ReactQueryProvider, ThemeProvider } from "@/components/providers";
```

Add `suppressHydrationWarning` to the `<html>` tag (required by `next-themes` since it sets the theme class client-side before hydration finishes):

```tsx
<html
  lang="en"
  suppressHydrationWarning
  className={`${geistSans.variable} ${geistMono.variable} ${bricolageGrotesque.variable} h-full antialiased`}
>
```

Wrap the existing provider tree with `ThemeProvider`:

```tsx
<body className="min-h-full flex flex-col">
  <ThemeProvider attribute="class" defaultTheme="dark" enableSystem={false} disableTransitionOnChange>
    <AuthProvider>
      <ReactQueryProvider>
        {children}
        <Toaster />
      </ReactQueryProvider>
    </AuthProvider>
  </ThemeProvider>
</body>
```

- [ ] **Step 6: Verify the build compiles**

Run: `cd bohikor && npm run build`
Expected: build succeeds.

- [ ] **Step 7: Run the full test suite**

Run: `cd bohikor && npm test`
Expected: all suites pass, including the new provider test.

- [ ] **Step 8: Commit**

```bash
git add bohikor/src/components/providers/index.tsx bohikor/src/components/providers/__tests__/index.test.tsx bohikor/src/app/layout.tsx
git commit -m "feat: wire next-themes ThemeProvider, dark by default"
```

---

### Task 4: Build the `ThemeToggle` component

**Files:**
- Create: `bohikor/src/components/theme-toggle.tsx`
- Test: `bohikor/src/components/__tests__/theme-toggle.test.tsx`

**Interfaces:**
- Consumes: `useTheme` from `next-themes` (available since Task 3).
- Produces: `ThemeToggle({ className?: string })` — a button component. Task 8 and Task 9 render `<ThemeToggle />` inside the sidebars; the next plan renders it in the employee app header.

- [ ] **Step 1: Write the failing test**

Create `bohikor/src/components/__tests__/theme-toggle.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ThemeToggle } from "../theme-toggle";

const mockSetTheme = jest.fn();
let mockResolvedTheme = "dark";

jest.mock("next-themes", () => ({
  useTheme: () => ({ resolvedTheme: mockResolvedTheme, setTheme: mockSetTheme }),
}));

describe("ThemeToggle", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockResolvedTheme = "dark";
  });

  it("shows a 'switch to light' control when the resolved theme is dark", async () => {
    render(<ThemeToggle />);
    expect(await screen.findByRole("button", { name: "Switch to light mode" })).toBeInTheDocument();
  });

  it("shows a 'switch to dark' control when the resolved theme is light", async () => {
    mockResolvedTheme = "light";
    render(<ThemeToggle />);
    expect(await screen.findByRole("button", { name: "Switch to dark mode" })).toBeInTheDocument();
  });

  it("calls setTheme with the opposite theme when clicked", async () => {
    const user = userEvent.setup();
    render(<ThemeToggle />);

    const button = await screen.findByRole("button", { name: "Switch to light mode" });
    await user.click(button);

    expect(mockSetTheme).toHaveBeenCalledWith("light");
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd bohikor && npx jest src/components/__tests__/theme-toggle.test.tsx`
Expected: FAIL — cannot find module `../theme-toggle`.

- [ ] **Step 3: Write the implementation**

Create `bohikor/src/components/theme-toggle.tsx`:

```tsx
"use client";

import { useEffect, useState } from "react";
import { Moon, Sun } from "lucide-react";
import { useTheme } from "next-themes";
import { cn } from "@/lib/utils";

export function ThemeToggle({ className }: { className?: string }) {
  const { resolvedTheme, setTheme } = useTheme();
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  const isDark = mounted ? resolvedTheme === "dark" : true;

  return (
    <button
      type="button"
      onClick={() => setTheme(isDark ? "light" : "dark")}
      aria-label={isDark ? "Switch to light mode" : "Switch to dark mode"}
      disabled={!mounted}
      className={cn(
        "flex h-9 w-9 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground disabled:opacity-0",
        className
      )}
    >
      {isDark ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
    </button>
  );
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd bohikor && npx jest src/components/__tests__/theme-toggle.test.tsx`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add bohikor/src/components/theme-toggle.tsx bohikor/src/components/__tests__/theme-toggle.test.tsx
git commit -m "feat: add ThemeToggle component"
```

---

### Task 5: Build the `useMediaQuery` hook

**Files:**
- Create: `bohikor/src/hooks/use-media-query.ts`
- Test: `bohikor/src/hooks/__tests__/use-media-query.test.ts`

**Interfaces:**
- Consumes: `window.matchMedia` (already globally mocked in `bohikor/jest.setup.ts` for other tests; this task's tests override that mock per-case).
- Produces: `useMediaQuery(query: string): boolean`. Task 7's `Dialog` calls `useMediaQuery("(min-width: 1024px)")` to decide bottom-sheet vs. centered presentation.

- [ ] **Step 1: Write the failing test**

Create `bohikor/src/hooks/__tests__/use-media-query.test.ts`:

```ts
import { renderHook, act } from "@testing-library/react";
import { useMediaQuery } from "../use-media-query";

function mockMatchMedia(initialMatches: boolean) {
  const listeners: Array<(event: MediaQueryListEvent) => void> = [];
  const mql = {
    matches: initialMatches,
    media: "",
    addEventListener: (
      _: string,
      listener: (event: MediaQueryListEvent) => void
    ) => {
      listeners.push(listener);
    },
    removeEventListener: jest.fn(),
  };
  window.matchMedia = jest.fn().mockReturnValue(mql);
  return {
    fireChange: (matches: boolean) => {
      listeners.forEach((listener) => listener({ matches } as MediaQueryListEvent));
    },
  };
}

describe("useMediaQuery", () => {
  it("returns true when the query currently matches", () => {
    mockMatchMedia(true);
    const { result } = renderHook(() => useMediaQuery("(min-width: 1024px)"));
    expect(result.current).toBe(true);
  });

  it("returns false when the query does not match", () => {
    mockMatchMedia(false);
    const { result } = renderHook(() => useMediaQuery("(min-width: 1024px)"));
    expect(result.current).toBe(false);
  });

  it("updates when the media query change event fires", () => {
    const { fireChange } = mockMatchMedia(false);
    const { result } = renderHook(() => useMediaQuery("(min-width: 1024px)"));
    expect(result.current).toBe(false);

    act(() => {
      fireChange(true);
    });

    expect(result.current).toBe(true);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd bohikor && npx jest src/hooks/__tests__/use-media-query.test.ts`
Expected: FAIL — cannot find module `../use-media-query`.

- [ ] **Step 3: Write the implementation**

Create `bohikor/src/hooks/use-media-query.ts`:

```ts
import { useEffect, useState } from "react";

export function useMediaQuery(query: string): boolean {
  const [matches, setMatches] = useState(false);

  useEffect(() => {
    const mediaQueryList = window.matchMedia(query);
    setMatches(mediaQueryList.matches);

    const listener = (event: MediaQueryListEvent) => setMatches(event.matches);
    mediaQueryList.addEventListener("change", listener);
    return () => mediaQueryList.removeEventListener("change", listener);
  }, [query]);

  return matches;
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd bohikor && npx jest src/hooks/__tests__/use-media-query.test.ts`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add bohikor/src/hooks/use-media-query.ts bohikor/src/hooks/__tests__/use-media-query.test.ts
git commit -m "feat: add useMediaQuery hook"
```

---

### Task 6: Build the `useSidebarCollapse` hook

**Files:**
- Create: `bohikor/src/hooks/use-sidebar-collapse.ts`
- Test: `bohikor/src/hooks/__tests__/use-sidebar-collapse.test.ts`

**Interfaces:**
- Consumes: `window.localStorage` (available by default in `jest-environment-jsdom`, no mock needed).
- Produces: `useSidebarCollapse(): { collapsed: boolean; toggle: () => void; hydrated: boolean }`. Task 8 and Task 9 both consume `collapsed`/`toggle`.

- [ ] **Step 1: Write the failing test**

Create `bohikor/src/hooks/__tests__/use-sidebar-collapse.test.ts`:

```ts
import { renderHook, act } from "@testing-library/react";
import { useSidebarCollapse } from "../use-sidebar-collapse";

describe("useSidebarCollapse", () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  it("defaults to expanded when nothing is stored", () => {
    const { result } = renderHook(() => useSidebarCollapse());
    expect(result.current.collapsed).toBe(false);
  });

  it("reads a previously stored collapsed state on mount", () => {
    window.localStorage.setItem("bohikor:sidebar-collapsed", "true");
    const { result } = renderHook(() => useSidebarCollapse());
    expect(result.current.collapsed).toBe(true);
  });

  it("toggling flips the state and persists it to localStorage", () => {
    const { result } = renderHook(() => useSidebarCollapse());

    act(() => {
      result.current.toggle();
    });

    expect(result.current.collapsed).toBe(true);
    expect(window.localStorage.getItem("bohikor:sidebar-collapsed")).toBe("true");

    act(() => {
      result.current.toggle();
    });

    expect(result.current.collapsed).toBe(false);
    expect(window.localStorage.getItem("bohikor:sidebar-collapsed")).toBe("false");
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd bohikor && npx jest src/hooks/__tests__/use-sidebar-collapse.test.ts`
Expected: FAIL — cannot find module `../use-sidebar-collapse`.

- [ ] **Step 3: Write the implementation**

Create `bohikor/src/hooks/use-sidebar-collapse.ts`:

```ts
import { useCallback, useEffect, useState } from "react";

const STORAGE_KEY = "bohikor:sidebar-collapsed";

export function useSidebarCollapse() {
  const [collapsed, setCollapsed] = useState(false);
  const [hydrated, setHydrated] = useState(false);

  useEffect(() => {
    const stored = window.localStorage.getItem(STORAGE_KEY);
    setCollapsed(stored === "true");
    setHydrated(true);
  }, []);

  const toggle = useCallback(() => {
    setCollapsed((prev) => {
      const next = !prev;
      window.localStorage.setItem(STORAGE_KEY, String(next));
      return next;
    });
  }, []);

  return { collapsed: hydrated ? collapsed : false, toggle, hydrated };
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd bohikor && npx jest src/hooks/__tests__/use-sidebar-collapse.test.ts`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add bohikor/src/hooks/use-sidebar-collapse.ts bohikor/src/hooks/__tests__/use-sidebar-collapse.test.ts
git commit -m "feat: add useSidebarCollapse hook"
```

---

### Task 7: Build the adaptive `Dialog` component

**Files:**
- Create: `bohikor/src/components/ui/dialog.tsx`
- Test: `bohikor/src/components/ui/__tests__/dialog.test.tsx`

**Interfaces:**
- Consumes: `useMediaQuery` (Task 5), the `radix-ui` package's `Dialog` namespace (already a dependency, same package `dropdown-menu.tsx` draws its primitive from), the `dialog-*` keyframes (Task 1).
- Produces: `Dialog`, `DialogTrigger`, `DialogClose`, `DialogContent`, `DialogHeader`, `DialogTitle`, `DialogDescription`, `DialogFooter` from `@/components/ui/dialog`. Task 10 consumes `Dialog`, `DialogContent`, `DialogHeader`, `DialogTitle`, `DialogDescription`, `DialogFooter`.

Presentation is CSS-driven off Radix's own `data-state="open"|"closed"` attribute (Radix keeps the content mounted until its exit animation/transition finishes, so no `forceMount`/external animation library is needed here for correctness) — bottom sheet under the `lg` breakpoint, centered dialog at `lg` and up, matching the "adaptive modal" behavior validated in the design spec.

- [ ] **Step 1: Write the failing test**

Create `bohikor/src/components/ui/__tests__/dialog.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "../dialog";

jest.mock("@/hooks/use-media-query", () => ({
  useMediaQuery: jest.fn(),
}));

const { useMediaQuery } = jest.requireMock("@/hooks/use-media-query");

function renderDialog() {
  render(
    <Dialog defaultOpen>
      <DialogContent>
        <DialogTitle>Reissue payout?</DialogTitle>
        <DialogDescription>Re-debits float and re-sends via Campay.</DialogDescription>
        <DialogFooter>
          <button>Confirm</button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

describe("DialogContent", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("renders bottom-sheet styling and a drag handle when isDesktop is false", () => {
    useMediaQuery.mockReturnValue(false);
    renderDialog();

    const content = screen.getByRole("dialog");
    expect(content.className).toContain("rounded-t-2xl");
    expect(content.className).toContain("bottom-0");
    expect(screen.getByTestId("dialog-sheet-handle")).toBeInTheDocument();
  });

  it("renders centered-dialog styling and no drag handle when isDesktop is true", () => {
    useMediaQuery.mockReturnValue(true);
    renderDialog();

    const content = screen.getByRole("dialog");
    expect(content.className).toContain("top-1/2");
    expect(content.className).not.toContain("rounded-t-2xl");
    expect(screen.queryByTestId("dialog-sheet-handle")).not.toBeInTheDocument();
  });

  it("renders title, description, and footer content", () => {
    useMediaQuery.mockReturnValue(true);
    renderDialog();

    expect(screen.getByText("Reissue payout?")).toBeInTheDocument();
    expect(screen.getByText("Re-debits float and re-sends via Campay.")).toBeInTheDocument();
    expect(screen.getByText("Confirm")).toBeInTheDocument();
  });

  it("closes when the close button is clicked", async () => {
    useMediaQuery.mockReturnValue(true);
    const user = userEvent.setup();
    renderDialog();

    expect(screen.getByRole("dialog")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Close" }));

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd bohikor && npx jest src/components/ui/__tests__/dialog.test.tsx`
Expected: FAIL — cannot find module `../dialog`.

- [ ] **Step 3: Write the implementation**

Create `bohikor/src/components/ui/dialog.tsx`:

```tsx
"use client";

import * as React from "react";
import { Dialog as DialogPrimitive } from "radix-ui";
import { X } from "lucide-react";
import { cn } from "@/lib/utils";
import { useMediaQuery } from "@/hooks/use-media-query";

const Dialog = DialogPrimitive.Root;
const DialogTrigger = DialogPrimitive.Trigger;
const DialogClose = DialogPrimitive.Close;

function DialogOverlay({
  className,
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Overlay>) {
  return (
    <DialogPrimitive.Overlay
      data-slot="dialog-overlay"
      className={cn(
        "fixed inset-0 z-50 bg-black/60 data-[state=open]:animate-[dialog-overlay-in_0.2s_ease-out] data-[state=closed]:animate-[dialog-overlay-out_0.2s_ease-in]",
        className
      )}
      {...props}
    />
  );
}

function DialogContent({
  className,
  children,
  showClose = true,
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Content> & { showClose?: boolean }) {
  const isDesktop = useMediaQuery("(min-width: 1024px)");

  return (
    <DialogPrimitive.Portal>
      <DialogOverlay />
      <DialogPrimitive.Content
        data-slot="dialog-content"
        className={cn(
          "fixed z-50 border border-border bg-card text-card-foreground shadow-lg outline-none",
          isDesktop
            ? "top-1/2 left-1/2 w-full max-w-md -translate-x-1/2 -translate-y-1/2 rounded-2xl p-6 data-[state=open]:animate-[dialog-scale-in_0.2s_ease-out] data-[state=closed]:animate-[dialog-scale-out_0.15s_ease-in]"
            : "inset-x-0 bottom-0 rounded-t-2xl p-6 pb-8 data-[state=open]:animate-[dialog-sheet-in_0.3s_ease-out] data-[state=closed]:animate-[dialog-sheet-out_0.2s_ease-in]",
          className
        )}
        {...props}
      >
        {!isDesktop && (
          <div
            data-testid="dialog-sheet-handle"
            aria-hidden="true"
            className="mx-auto mb-4 h-1.5 w-9 rounded-full bg-border-strong"
          />
        )}
        {children}
        {showClose && (
          <DialogPrimitive.Close
            aria-label="Close"
            className="absolute top-4 right-4 rounded-lg p-1 text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
          >
            <X className="h-4 w-4" />
          </DialogPrimitive.Close>
        )}
      </DialogPrimitive.Content>
    </DialogPrimitive.Portal>
  );
}

function DialogHeader({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div data-slot="dialog-header" className={cn("mb-4 space-y-1.5", className)} {...props} />
  );
}

function DialogTitle({
  className,
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Title>) {
  return (
    <DialogPrimitive.Title
      data-slot="dialog-title"
      className={cn("text-base font-semibold text-foreground", className)}
      {...props}
    />
  );
}

function DialogDescription({
  className,
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Description>) {
  return (
    <DialogPrimitive.Description
      data-slot="dialog-description"
      className={cn("text-sm text-muted-foreground", className)}
      {...props}
    />
  );
}

function DialogFooter({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="dialog-footer"
      className={cn("mt-6 flex justify-end gap-3", className)}
      {...props}
    />
  );
}

export {
  Dialog,
  DialogTrigger,
  DialogClose,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
};
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd bohikor && npx jest src/components/ui/__tests__/dialog.test.tsx`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add bohikor/src/components/ui/dialog.tsx bohikor/src/components/ui/__tests__/dialog.test.tsx
git commit -m "feat: add adaptive Dialog (bottom sheet on mobile, centered on desktop)"
```

---

### Task 8: Collapsible icon-rail `Sidebar` (company admin)

**Files:**
- Modify: `bohikor/src/components/sidebar.tsx`
- Modify: `bohikor/src/components/__tests__/sidebar.test.tsx`

**Interfaces:**
- Consumes: `useSidebarCollapse` (Task 6), `ThemeToggle` (Task 4), `motion` from `motion/react` (new dependency, installed in Step 1).
- Produces: `Sidebar()` — same export name/shape as before (no props), so every existing page that renders `<Sidebar />` keeps working unchanged.

- [ ] **Step 1: Install the `motion` dependency**

Run: `cd bohikor && npm install motion`
Expected: `package.json`/`package-lock.json` gain a `motion` entry.

- [ ] **Step 2: Update the failing test**

Replace `bohikor/src/components/__tests__/sidebar.test.tsx` with:

```tsx
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Sidebar } from "../sidebar";

const mockSignOut = jest.fn();
const mockPush = jest.fn();
const mockToggle = jest.fn();
let mockCollapsed = false;

jest.mock("next/navigation", () => ({
  usePathname: jest.fn(),
  useParams: () => ({ company: "acme" }),
  useRouter: () => ({ push: mockPush }),
}));

jest.mock("@/components/providers", () => ({
  useAuth: () => ({ signOut: mockSignOut }),
}));

jest.mock("@/hooks/use-sidebar-collapse", () => ({
  useSidebarCollapse: () => ({ collapsed: mockCollapsed, toggle: mockToggle }),
}));

jest.mock("next-themes", () => ({
  useTheme: () => ({ resolvedTheme: "dark", setTheme: jest.fn() }),
}));

const { usePathname } = jest.requireMock("next/navigation");

describe("Sidebar", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockCollapsed = false;
    usePathname.mockReturnValue("/acme/admin");
  });

  it("renders all 7 nav links with slug-prefixed hrefs when expanded", () => {
    render(<Sidebar />);

    const expected: [string, string][] = [
      ["Dashboard", "/acme/admin"],
      ["Invite", "/acme/admin/invite"],
      ["Users", "/acme/admin/users"],
      ["Requests", "/acme/admin/requests"],
      ["Events", "/acme/admin/events"],
      ["Balance", "/acme/admin/balance"],
      ["Settings", "/acme/admin/settings"],
    ];

    for (const [label, href] of expected) {
      const link = screen.getByRole("link", { name: label });
      expect(link).toHaveAttribute("href", href);
    }
  });

  it("applies the emerald active-state classes to the current route's link", () => {
    usePathname.mockReturnValue("/acme/admin/users");
    render(<Sidebar />);

    const usersLink = screen.getByRole("link", { name: "Users" });
    const dashboardLink = screen.getByRole("link", { name: "Dashboard" });

    expect(usersLink.className).toContain("bg-emerald-500/10");
    expect(dashboardLink.className).not.toContain("bg-emerald-500/10");
  });

  it("calls signOut and redirects to the company login page when sign out is clicked", async () => {
    const user = userEvent.setup();
    render(<Sidebar />);

    await user.click(screen.getByRole("button", { name: "Sign Out" }));

    expect(mockSignOut).toHaveBeenCalled();
    expect(mockPush).toHaveBeenCalledWith("/acme/admin/login");
  });

  it("hides nav labels and the brand name, and adds a title tooltip, when collapsed", () => {
    mockCollapsed = true;
    render(<Sidebar />);

    expect(screen.queryByText("Dashboard")).not.toBeInTheDocument();
    expect(screen.queryByText("Bohikor")).not.toBeInTheDocument();

    const dashboardLink = screen.getByRole("link", { name: "Dashboard" });
    expect(dashboardLink).toHaveAttribute("title", "Dashboard");
  });

  it("calls toggle when the collapse/expand control is clicked", async () => {
    const user = userEvent.setup();
    render(<Sidebar />);

    await user.click(screen.getByRole("button", { name: "Collapse sidebar" }));

    expect(mockToggle).toHaveBeenCalled();
  });
});
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd bohikor && npx jest src/components/__tests__/sidebar.test.tsx`
Expected: FAIL — the current `Sidebar` has no collapse behavior, no `aria-label`s on the nav links, and the active-state class is still `bg-sidebar-primary`, not `bg-emerald-500/10`.

- [ ] **Step 4: Rewrite the implementation**

Replace `bohikor/src/components/sidebar.tsx` with:

```tsx
"use client";

import Link from "next/link";
import Image from "next/image";
import { motion } from "motion/react";
import { usePathname, useParams, useRouter } from "next/navigation";
import { cn } from "@/lib/utils";
import { useAuth } from "@/components/providers";
import { useSidebarCollapse } from "@/hooks/use-sidebar-collapse";
import { ThemeToggle } from "@/components/theme-toggle";
import {
  LayoutDashboard,
  Mail,
  Users,
  ArrowLeftRight,
  Activity,
  Wallet,
  Settings,
  LogOut,
  PanelLeftClose,
  PanelLeftOpen,
} from "lucide-react";

export function Sidebar() {
  const pathname = usePathname();
  const router = useRouter();
  const { company } = useParams<{ company: string }>();
  const { signOut } = useAuth();
  const { collapsed, toggle } = useSidebarCollapse();

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
    <motion.aside
      animate={{ width: collapsed ? 56 : 256 }}
      transition={{ duration: 0.2, ease: "easeInOut" }}
      className="flex h-screen shrink-0 flex-col overflow-hidden border-r border-sidebar-border bg-sidebar"
    >
      <div className="flex h-16 items-center gap-2 border-b border-sidebar-border px-4">
        <Image src="/logo.png" alt="Bohikor" width={28} height={28} className="shrink-0" />
        {!collapsed && (
          <h1 className="truncate text-lg font-semibold text-sidebar-foreground">Bohikor</h1>
        )}
      </div>

      <nav className="flex-1 space-y-1 px-3 py-4">
        {navItems.map((item) => {
          const Icon = item.icon;
          const isActive = pathname === item.href;

          return (
            <Link
              key={item.href}
              href={item.href}
              aria-label={item.label}
              title={collapsed ? item.label : undefined}
              className={cn(
                "flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors",
                collapsed && "justify-center px-0",
                isActive
                  ? "bg-emerald-500/10 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-400"
                  : "text-sidebar-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
              )}
            >
              <Icon className="h-5 w-5 shrink-0" />
              {!collapsed && <span className="truncate">{item.label}</span>}
            </Link>
          );
        })}
      </nav>

      <div className="space-y-1 border-t border-sidebar-border p-3">
        <ThemeToggle className={collapsed ? "mx-auto" : ""} />
        <button
          onClick={async () => {
            await signOut();
            router.push(`/${company}/admin/login`);
          }}
          title={collapsed ? "Sign Out" : undefined}
          className={cn(
            "flex w-full items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium text-sidebar-foreground transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
            collapsed && "justify-center px-0"
          )}
        >
          <LogOut className="h-5 w-5 shrink-0" />
          {!collapsed && "Sign Out"}
        </button>
        <button
          onClick={toggle}
          aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
          title={collapsed ? "Expand sidebar" : "Collapse sidebar"}
          className={cn(
            "flex w-full items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium text-muted-foreground transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
            collapsed && "justify-center px-0"
          )}
        >
          {collapsed ? (
            <PanelLeftOpen className="h-5 w-5 shrink-0" />
          ) : (
            <>
              <PanelLeftClose className="h-5 w-5 shrink-0" />
              <span>Collapse</span>
            </>
          )}
        </button>
      </div>
    </motion.aside>
  );
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd bohikor && npx jest src/components/__tests__/sidebar.test.tsx`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add bohikor/src/components/sidebar.tsx bohikor/src/components/__tests__/sidebar.test.tsx bohikor/package.json bohikor/package-lock.json
git commit -m "feat: collapsible icon-rail Sidebar with theme toggle"
```

---

### Task 9: Collapsible icon-rail `PlatformSidebar`

**Files:**
- Modify: `bohikor/src/components/platform-sidebar.tsx`
- Create: `bohikor/src/components/__tests__/platform-sidebar.test.tsx` (no test file exists for this component yet — confirmed via `ls bohikor/src/components/__tests__/`, which currently contains only `auth-guard.test.tsx`, `forbidden.test.tsx`, `sidebar.test.tsx`)

**Interfaces:**
- Consumes: `useSidebarCollapse` (Task 6), `ThemeToggle` (Task 4), `motion` (already installed in Task 8).
- Produces: `PlatformSidebar()` — same export name/shape as before.

- [ ] **Step 1: Write the failing test**

Create `bohikor/src/components/__tests__/platform-sidebar.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { PlatformSidebar } from "../platform-sidebar";

const mockSignOut = jest.fn();
const mockPush = jest.fn();
const mockToggle = jest.fn();
let mockCollapsed = false;

jest.mock("next/navigation", () => ({
  usePathname: jest.fn(),
  useRouter: () => ({ push: mockPush }),
}));

jest.mock("@/components/providers", () => ({
  useAuth: () => ({ signOut: mockSignOut }),
}));

jest.mock("@/hooks/use-sidebar-collapse", () => ({
  useSidebarCollapse: () => ({ collapsed: mockCollapsed, toggle: mockToggle }),
}));

jest.mock("next-themes", () => ({
  useTheme: () => ({ resolvedTheme: "dark", setTheme: jest.fn() }),
}));

const { usePathname } = jest.requireMock("next/navigation");

describe("PlatformSidebar", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockCollapsed = false;
    usePathname.mockReturnValue("/platform");
  });

  it("renders the Dashboard nav link", () => {
    render(<PlatformSidebar />);
    expect(screen.getByRole("link", { name: "Dashboard" })).toHaveAttribute("href", "/platform");
  });

  it("applies the emerald active-state classes on the current route", () => {
    render(<PlatformSidebar />);
    expect(screen.getByRole("link", { name: "Dashboard" }).className).toContain(
      "bg-emerald-500/10"
    );
  });

  it("calls signOut and redirects to the platform login page when sign out is clicked", async () => {
    const user = userEvent.setup();
    render(<PlatformSidebar />);

    await user.click(screen.getByRole("button", { name: "Sign Out" }));

    expect(mockSignOut).toHaveBeenCalled();
    expect(mockPush).toHaveBeenCalledWith("/platform/login");
  });

  it("hides labels and shows a title tooltip when collapsed", () => {
    mockCollapsed = true;
    render(<PlatformSidebar />);

    expect(screen.queryByText("Bohikor")).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Dashboard" })).toHaveAttribute("title", "Dashboard");
  });

  it("calls toggle when the collapse/expand control is clicked", async () => {
    const user = userEvent.setup();
    render(<PlatformSidebar />);

    await user.click(screen.getByRole("button", { name: "Collapse sidebar" }));

    expect(mockToggle).toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd bohikor && npx jest src/components/__tests__/platform-sidebar.test.tsx`
Expected: FAIL against the current implementation (no collapse behavior, no `aria-label`, old active-state class).

- [ ] **Step 3: Rewrite the implementation**

Replace `bohikor/src/components/platform-sidebar.tsx` with:

```tsx
"use client";

import Link from "next/link";
import Image from "next/image";
import { motion } from "motion/react";
import { usePathname, useRouter } from "next/navigation";
import { cn } from "@/lib/utils";
import { useAuth } from "@/components/providers";
import { useSidebarCollapse } from "@/hooks/use-sidebar-collapse";
import { ThemeToggle } from "@/components/theme-toggle";
import { LayoutDashboard, LogOut, PanelLeftClose, PanelLeftOpen } from "lucide-react";

export function PlatformSidebar() {
  const pathname = usePathname();
  const router = useRouter();
  const { signOut } = useAuth();
  const { collapsed, toggle } = useSidebarCollapse();

  const navItems = [{ href: "/platform", label: "Dashboard", icon: LayoutDashboard }];

  return (
    <motion.aside
      animate={{ width: collapsed ? 56 : 256 }}
      transition={{ duration: 0.2, ease: "easeInOut" }}
      className="flex h-screen shrink-0 flex-col overflow-hidden border-r border-sidebar-border bg-sidebar"
    >
      <div className="flex h-16 items-center gap-2 border-b border-sidebar-border px-4">
        <Image src="/logo.png" alt="Bohikor" width={28} height={28} className="shrink-0" />
        {!collapsed && (
          <div className="truncate">
            <h1 className="text-lg font-semibold text-sidebar-foreground">Bohikor</h1>
            <p className="text-xs text-muted-foreground">Platform</p>
          </div>
        )}
      </div>

      <nav className="flex-1 space-y-1 px-3 py-4">
        {navItems.map((item) => {
          const Icon = item.icon;
          const isActive = pathname === item.href;

          return (
            <Link
              key={item.href}
              href={item.href}
              aria-label={item.label}
              title={collapsed ? item.label : undefined}
              className={cn(
                "flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors",
                collapsed && "justify-center px-0",
                isActive
                  ? "bg-emerald-500/10 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-400"
                  : "text-sidebar-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
              )}
            >
              <Icon className="h-5 w-5 shrink-0" />
              {!collapsed && <span className="truncate">{item.label}</span>}
            </Link>
          );
        })}
      </nav>

      <div className="space-y-1 border-t border-sidebar-border p-3">
        <ThemeToggle className={collapsed ? "mx-auto" : ""} />
        <button
          onClick={async () => {
            await signOut();
            router.push("/platform/login");
          }}
          title={collapsed ? "Sign Out" : undefined}
          className={cn(
            "flex w-full items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium text-sidebar-foreground transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
            collapsed && "justify-center px-0"
          )}
        >
          <LogOut className="h-5 w-5 shrink-0" />
          {!collapsed && "Sign Out"}
        </button>
        <button
          onClick={toggle}
          aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
          title={collapsed ? "Expand sidebar" : "Collapse sidebar"}
          className={cn(
            "flex w-full items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium text-muted-foreground transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
            collapsed && "justify-center px-0"
          )}
        >
          {collapsed ? (
            <PanelLeftOpen className="h-5 w-5 shrink-0" />
          ) : (
            <>
              <PanelLeftClose className="h-5 w-5 shrink-0" />
              <span>Collapse</span>
            </>
          )}
        </button>
      </div>
    </motion.aside>
  );
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd bohikor && npx jest src/components/__tests__/platform-sidebar.test.tsx`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add bohikor/src/components/platform-sidebar.tsx bohikor/src/components/__tests__/platform-sidebar.test.tsx
git commit -m "feat: collapsible icon-rail PlatformSidebar with theme toggle"
```

---

### Task 10: Migrate the employee home page's modal to `Dialog`

**Files:**
- Modify: `bohikor/src/app/[company]/(protected)/page.tsx`

**Interfaces:**
- Consumes: `Dialog`, `DialogContent`, `DialogHeader`, `DialogTitle`, `DialogDescription`, `DialogFooter` (Task 7).
- Produces: nothing new for later tasks — this is the foundation plan's proof-of-integration and a real bug fix (the old modal was hardcoded `bg-white`, which rendered a white box in dark mode).

The existing test file (`bohikor/src/app/[company]/(protected)/__tests__/page.test.tsx`) asserts on text content and button names only ("Confirm Advance Request", the description paragraph, the "Confirm"/"Cancel" buttons) — none of that text changes, so no test edits are needed. Those tests are this task's regression net.

- [ ] **Step 1: Confirm the current tests pass before touching the page**

Run: `cd bohikor && npx jest "src/app/\[company\]/\(protected\)/__tests__/page.test.tsx"`
Expected: PASS (baseline, before the refactor).

- [ ] **Step 2: Update the imports**

In `bohikor/src/app/[company]/(protected)/page.tsx`, remove `X` from the `lucide-react` import (no longer used directly — `Dialog`'s built-in close button owns it now):

```tsx
import { Check, LogOut, MoreVertical, RefreshCw, User } from "lucide-react";
```

Add the dialog import:

```tsx
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
```

- [ ] **Step 3: Replace the hardcoded modal markup**

Find the block starting at `{modalVisible && (` (the manual `fixed inset-0` overlay with the hardcoded `bg-white` panel) and replace the entire block with:

```tsx
<Dialog open={modalVisible} onOpenChange={setModalVisible}>
  <DialogContent>
    <DialogHeader>
      <DialogTitle>Confirm Advance Request</DialogTitle>
    </DialogHeader>

    <DialogDescription>
      You are about to request a salary advance of{" "}
      <span className="font-semibold text-foreground">{advanceAmount}</span>.
      This amount plus any applicable charges will be deducted from your
      upcoming salary payment. You can only have one active advance request
      at a time.
    </DialogDescription>

    {modalError && <p className="mt-3 text-sm text-destructive">{modalError}</p>}

    <DialogFooter>
      <Button
        variant="outline"
        onClick={() => setModalVisible(false)}
        disabled={createRequest.isPending}
      >
        Cancel
      </Button>
      <Button onClick={handleConfirmRequest} disabled={createRequest.isPending}>
        {createRequest.isPending ? "Requesting..." : "Confirm"}
      </Button>
    </DialogFooter>
  </DialogContent>
</Dialog>
```

- [ ] **Step 4: Run the page's test suite to confirm it still passes unchanged**

Run: `cd bohikor && npx jest "src/app/\[company\]/\(protected\)/__tests__/page.test.tsx"`
Expected: PASS — same assertions as the Step 1 baseline, now exercising the new `Dialog`-based markup.

- [ ] **Step 5: Run lint and typecheck**

Run: `cd bohikor && npm run lint && npm run typecheck`
Expected: both pass (confirms the removed `X` import isn't flagged as unused elsewhere and no type errors from the new JSX).

- [ ] **Step 6: Run the full test suite and build**

Run: `cd bohikor && npm test && npm run build`
Expected: all suites pass; production build succeeds.

- [ ] **Step 7: Commit**

```bash
git add "bohikor/src/app/[company]/(protected)/page.tsx"
git commit -m "fix: migrate employee home request modal to adaptive Dialog

Replaces hardcoded bg-white modal markup (which rendered incorrectly
in dark mode) with the new adaptive Dialog component."
```

---

## After this plan

This plan intentionally stops at the shared foundation plus one proof-of-integration screen. The design spec's own phasing (Employee → Company Admin → Platform Admin) continues as separate follow-on plans:

- **Next:** apply `font-heading`/tabular-nums, motion (entrance stagger, CTA glow), and the desktop two-column/icon-rail layout to the remaining employee screens (`(protected)/history`, `(protected)/account`, `login`, `signup`, `verify`, `create-pin`, `forgot-pin`, `reset-pin`) — the surface prioritized first per the design spec.
- **Then:** company admin screens (`[company]/admin/**`) — restyled tables, modal-based row actions for `reconcile`/`resolve`/`reissue` (replacing the current dropdown), balance/ledger view polish.
- **Then:** platform admin screens (`platform/**`) — same treatment, lowest urgency.

Each should get its own plan via `superpowers:writing-plans` once this one is merged and verified.
