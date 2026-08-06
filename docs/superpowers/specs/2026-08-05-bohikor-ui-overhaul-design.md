# Bohikor UI/UX Overhaul — Design

**Date:** 2026-08-05
**Scope:** `bohikor/` (Next.js app) only. **Backend is untouched** — no API, schema, or contract
changes. This is a re-skin plus targeted structural layout changes (sidebar, modal presentation,
responsive grid), not a rewrite of data/logic. Existing TanStack Query hooks, auth flow, routing
structure, and the Jest+RTL suite (270 passing tests as of this writing) carry over unchanged.

**Why:** the current app (Epic 8, shipped) is functionally complete across all three surfaces
(employee, company admin, platform admin) but visually plain — default shadcn-ish grayscale
palette, no dark mode, static non-collapsible sidebar, dropdown menus for consequential actions,
no motion. Goal: "modern and delightful, not too much," ready to present as a real market product.

---

## 1. Visual language

**Direction:** premium fintech minimal (Stripe/Mercury/Wise-adjacent) — restrained color, generous
whitespace, one confident accent hue, gradients and shadows used sparingly and with intent, quiet
high-polish motion.

**Dark mode is default.** Light mode is a genuine mirror of the same theme family, not a separate
palette — both were validated against the same card/button/badge mockup before being approved.

### Dark theme — "Tinted Slate"

| Token | Value | Usage |
|---|---|---|
| `--background` | `#0b1220` | page background — blue-tinted dark navy, not pure charcoal/black |
| `--card` | `#111a2d` | standard card surface |
| `--card-elevated` | `#151f36` | hero/emphasis card surface (e.g. balance card), often with a gradient overlay: `linear-gradient(160deg, #151f36, #111a2d)` |
| `--border` | `#1e2a45` | standard card border |
| `--border-strong` | `#22304a` | emphasis card border, dividers in sidebar/header |
| `--foreground` | `#f1f5f9` / `#f8fafc` | primary text |
| `--muted-foreground` | `#8b98ac` | secondary/label text |
| `--muted-foreground-dim` | `#5c6a82` | tertiary text, table headers |

### Light theme — "Cool Tinted"

| Token | Value | Usage |
|---|---|---|
| `--background` | `#f4f6fa` | page background — faint navy-tinted off-white, not pure white |
| `--card` | `#ffffff` | card surface, pops against the tinted page bg |
| `--border` | `#dde3ee` | card border |
| `--foreground` | `#0f1b2e` | primary text |
| `--muted-foreground` | `#5c6a82` | secondary text |

Both themes keep the existing shadcn-style token *names* already in `globals.css`
(`--background`, `--card`, `--border`, `--muted-foreground`, `--sidebar*`, etc.) — only the values
change, plus the two new `--card-elevated` / `--border-strong` tokens for the hero-card treatment.
Existing `--radius` scale stays structurally the same but increases: cards ~18px, buttons/inputs
~10-12px, badges/pills fully rounded.

### Accent — emerald / money-green

- Primary gradient: `linear-gradient(135deg, #22c55e, #16a34a)` (light-mode buttons use
  `#22c55e → #15803d` for sufficient contrast against white).
- Positive/success badges: `background: rgba(34,197,94,.15)`, text `#4ade80` (dark) / `#16a34a`
  (light).
- Warning (e.g. `needs_admin_review`): amber family — `#c99a5c` label, `#f1c98a` text on
  `rgba(201,154,92,.15)` background, distinct from destructive red (unchanged, keeps existing
  `--destructive` token).
- **Discipline:** gradient fills are reserved for the primary CTA and one hero card's edge/overlay
  per screen. They are not applied to every button, card, or badge — flat surfaces plus the
  restrained accent do most of the work.

### Shadows

Soft and directional, used for elevation only (never purely decorative):
`0 8px 24px -8px rgba(0,0,0,.5)` for standard cards, `0 20px 50px -12px rgba(0,0,0,.6)` for
modals/popovers, a tighter glow-shadow (`0 4px 20px -4px rgba(34,197,94,.4)`) reserved for the
primary CTA to reinforce the accent color.

---

## 2. Typography

- **Body/UI text:** Geist Sans, unchanged — already wired via `next/font/google` in `layout.tsx`,
  zero migration cost.
- **Display (headings, hero numbers):** **Bricolage Grotesque**, added via `next/font/google`
  (`--font-heading` CSS variable already exists as a placeholder in `globals.css:8`, currently
  aliased to `--font-sans` — repoint it to the new font). Used for page titles, section headings,
  and monetary hero figures (the balance amount).
- **Numerals:** all monetary/count figures get `font-variant-numeric: tabular-nums` so digits don't
  shift width on refetch/update.

---

## 3. Motion

New dependency: **`motion`** (Framer Motion's current package name). Scope of use:

- Staggered entrance on page load for stacked cards (~80ms stagger, `fadeUp`-style: opacity 0→1,
  translateY 8px→0, ~300-500ms ease-out).
- Modal/sheet enter-exit transitions (scale+fade for centered dialogs, slide-up for bottom sheets).
- Sidebar collapse/expand (width + label opacity transition).
- Hover lift on interactive cards/buttons (`translateY(-2px) scale(1.01)`, ~150ms).
- A slow "breathing" glow on the primary CTA only (box-shadow pulse, ~2.8s ease-in-out loop) —
  the single most expressive motion in the system, deliberately not repeated elsewhere so it stays
  meaningful.

Kept subtle throughout: short durations (150-500ms), no bounce/spring easing that reads as
cartoonish, no motion that blocks or delays user action (all animations are decorative overlays on
top of an already-interactive element, never gating interactivity).

---

## 4. Layout patterns

### Collapsible icon-rail sidebar (company admin + platform admin)

Replaces the current fixed 256px, non-collapsible `<aside>` in `components/sidebar.tsx` and
`components/platform-sidebar.tsx`. Collapsed state: ~52px icon-only strip, tooltips on hover,
never fully disappears (nav stays reachable at a glance — distinct from a hide-entirely overlay
drawer, which was considered and rejected for costing an extra click to navigate). Expanded state:
current ~256px width with icon + label. Theme toggle (sun/moon) sits at the bottom of the rail,
above sign-out.

### Adaptive modals

One new modal component (`components/ui/dialog.tsx`, built on the `Dialog` primitive already
available via the `radix-ui` package — same dependency `dropdown-menu.tsx` already draws on, no
new install), presentation switches by viewport:
**bottom sheet on mobile** (slides up, drag handle, rounded top corners only), **centered dialog on
desktop** (scrim + centered card). Same trigger, same content, CSS/breakpoint-driven presentation
only.

**Dropdown → modal migration rule:** an interaction becomes a modal when it (a) triggers a
consequential/hard-to-reverse action (e.g. admin `reconcile`/`resolve`/`reissue` on a request,
currently a dropdown in the admin requests table) or (b) needs more than one field of context to
decide. Plain navigation (e.g. the employee header's "Account" / "Sign out" menu) stays a
lightweight dropdown — not everything becomes a modal.

### Responsive employee app (mobile-first, desktop-enhanced)

Mobile (`<lg`): single-column stack, header with logo + account dropdown, full-width cards.
Desktop (`≥lg`): gains the icon rail (replacing the mobile header's account dropdown for
navigation — Home/History/Account/Sign out) and a two-column grid — hero balance card
(width-capped, ~280px in the reference mockup) alongside a stats/recent-activity column. **Content
is capped at a max effective width (~680px) and left-aligned next to the rail rather than
stretching edge-to-edge** on wide monitors — the same width-discipline principle applies to
admin/platform main content areas (data tables are the exception: those may use more horizontal
space since column density is the point).

### Dashboards (admin + platform)

Stat cards (3-up grid, ~180px min each) + existing tables. No charting/sparkline library for this
pass — explicitly deferred, numbers-and-badges only.

---

## 5. Theming mechanics (dark default, light opt-in, no backend)

- `next-themes` (already present in `package.json`, currently unused — only referenced by
  `components/ui/sonner.tsx` for toast theming) gets a `ThemeProvider` added to the root layout
  (`app/layout.tsx`), `attribute="class"`, `defaultTheme="dark"`.
- Persistence is `localStorage` only, exactly matching `next-themes`' default behavior — **no
  backend field, no user-table column, no API call.** Satisfies the "don't touch the backend"
  constraint directly.
- A `<ThemeToggle>` component (new, in `components/`) renders a sun/moon icon button that calls
  `next-themes`' `useTheme().setTheme(...)`. Placement: bottom of the icon rail for admin/platform,
  header (near the account control) for the employee app.
- `globals.css`'s existing `.dark` class selector mechanism is kept as-is — only the token values
  inside `:root` and `.dark` change to the new palettes above.

---

## 6. New tooling

| Package | New dependency? | Purpose |
|---|---|---|
| `motion` | **Yes** | animation (entrance, modal transitions, sidebar, hover, CTA glow) |
| Bricolage Grotesque | No (via `next/font/google`) | display typeface for headings/hero numbers |
| `next-themes` | No — already installed, unused | dark/light theme provider + toggle |

Explicitly **not** adding: a charting library (deferred — stat cards only for now), a new
component-kit CLI/framework (staying on bare `radix-ui` + Tailwind v4, extending the existing
9-primitive `components/ui/` set rather than replacing it — the app was deliberately "de-shadcn"ed
in an earlier pass and this design doesn't reverse that decision).

---

## 7. Scope & phasing

Same design tokens and shared components (theme provider/toggle, modal, sidebar, updated `ui/`
primitives) are built once and reused across all three surfaces. Visual/motion polish is applied
in this order:

1. **Employee app** (`[company]/(protected)/*`, `[company]/{login,signup,verify,create-pin,
   forgot-pin,reset-pin}`) — highest-volume, most market-facing, currently the plainest screens.
2. **Company admin** (`[company]/admin/**`) — icon rail, modal-based row actions, restyled tables.
3. **Platform admin** (`platform/**`) — same treatment, lowest urgency (internal-facing).

## 8. Non-goals

- No backend/API/schema changes of any kind.
- No new routes or features — this is a visual/interaction-layer pass over existing screens.
- No charting/data-visualization library in this pass.
- No replacement of the component primitive strategy (still bare Radix + Tailwind, not a shadcn
  CLI reintroduction or a different UI kit).
- No changes to TanStack Query hooks, auth logic, or business logic — token/style/layout/motion
  changes only, plus the new theme provider/toggle and modal-presentation component.

---

## Reference mockups

Validated interactively during brainstorming (dark-mode base, light-mode pairing, sidebar
behavior, modal presentation, employee home mobile + desktop, admin dashboard with modal row
action, display font) — HTML mockups preserved at
`.superpowers/brainstorm/37637-1785962547/content/` for visual reference during implementation.
