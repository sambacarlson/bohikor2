# Frontend Test Coverage (≥85%) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Raise `bohikor/`'s Jest coverage from 25.64%/26.75%/23.17%/24.68% (stmts/branch/funcs/lines) to ≥85% on all four metrics, with a jest `coverageThreshold` that enforces it going forward, by writing real behavioral tests for every currently-0%-coverage file.

**Architecture:** Test-only work (plus jest config). Every new test file follows the three existing tests' established convention exactly: `jest.mock("@/module", () => ({ fn: jest.fn() }))` + `jest.requireMock(...)`, never `jest.spyOn` or centralized mock factories. A new `bohikor/src/test-utils.tsx` shares only the `QueryClientProvider` render-wrapper boilerplate the existing invite/requests tests already hand-roll — it does not touch the per-file mocking convention.

**Tech Stack:** Jest 30, `@testing-library/react` 16, `@testing-library/user-event` 14, `@testing-library/jest-dom` 6, `@tanstack/react-query` 5, `next/jest` transform (not `ts-jest`, which is present but unused).

## Global Constraints

- Coverage threshold target: 85% statements, 85% branches, 85% functions, 85% lines, enforced via `bohikor/jest.config.js`'s `coverageThreshold.global`.
- `src/app/layout.tsx` is excluded from coverage collection (root layout wrapping `<html>`/`<body>` + Google fonts — conventionally excluded, not unit-testable in jsdom without disproportionate effort).
- The bare `npm test` script stays as `jest` (fast, ungated, for local dev/watch); a new `npm run test:coverage` (`jest --coverage`) is the gated command that must pass ≥85%.
- Every new test file must follow the existing `jest.mock` + `jest.requireMock` convention — no `jest.spyOn`, no centralized mock modules beyond the shared render-wrapper in `test-utils.tsx`.
- Work continues on the existing `epic-8-unified-web-app` branch (open PR #6) — this is additive test-only work closing that PR's own noted follow-up, not a new branch.
- The three `typeof window === "undefined"` SSR-guard branches in `lib/auth.ts`'s getters are the one accepted, documented, untestable-in-jsdom gap — do not attempt to fake `window` away to chase them; if the final coverage gate is short, look elsewhere first.

---

### Task 1: Jest config — coverage collection, threshold, script, setup polyfills

**Files:**
- Modify: `bohikor/jest.config.js`
- Modify: `bohikor/jest.setup.ts`
- Modify: `bohikor/package.json`

**Interfaces:**
- Produces: `npm run test:coverage` command that every subsequent task's implementer runs to check progress; a failing threshold here is *expected* until Task 17.

- [ ] **Step 1: Update jest.config.js**

Replace the `customJestConfig` object in `bohikor/jest.config.js` with:

```js
const customJestConfig = {
  setupFilesAfterEnv: ["<rootDir>/jest.setup.ts"],
  testEnvironment: "jest-environment-jsdom",
  moduleNameMapper: {
    "^@/(.*)$": "<rootDir>/src/$1",
  },
  collectCoverageFrom: [
    "src/**/*.{ts,tsx}",
    "!src/**/*.d.ts",
    "!src/app/layout.tsx",
    "!src/test-utils.tsx",
    "!src/**/__tests__/**",
  ],
  coverageThreshold: {
    global: {
      statements: 85,
      branches: 85,
      functions: 85,
      lines: 85,
    },
  },
};
```

- [ ] **Step 2: Add Radix polyfills to jest.setup.ts**

Replace the full contents of `bohikor/jest.setup.ts`:

```ts
import "@testing-library/jest-dom";

// Radix UI (dropdown-menu) needs these in jsdom.
Element.prototype.hasPointerCapture = jest.fn();
Element.prototype.releasePointerCapture = jest.fn();
Element.prototype.scrollIntoView = jest.fn();
```

- [ ] **Step 3: Add the test:coverage script**

In `bohikor/package.json`'s `scripts`, add a line after `"test": "jest",`:

```json
    "test:coverage": "jest --coverage",
```

- [ ] **Step 4: Verify the config loads and the threshold fails as expected**

Run: `cd bohikor && npm run test:coverage`
Expected: The existing 4 test suites still pass, but the run exits non-zero with a `Jest: "global" coverage threshold for statements/branches/functions/lines not met` message — this is the correct, expected state at this point in the plan. Confirm no other error (e.g. a config syntax error) is present.

- [ ] **Step 5: Commit**

```bash
cd /Users/carlson/space/bohikor2
git add bohikor/jest.config.js bohikor/jest.setup.ts bohikor/package.json
git commit -m "test(bohikor): add coverage collection, 85% threshold, test:coverage script"
```

---

### Task 2: Shared test-utils helper

**Files:**
- Create: `bohikor/src/test-utils.tsx`

**Interfaces:**
- Produces: `renderWithProviders(ui: React.ReactElement, opts?: { withToaster?: boolean }): RenderResult` — wraps in a fresh retry-disabled `QueryClient`/`QueryClientProvider`, optionally including `<Toaster />` (default `true`). Produces: `renderHookWithClient<T>(hook: () => T): { wrapper, queryClient }` — for `renderHook` calls in hook tests. Consumed by every task from Task 4 onward that renders a page/component needing React Query context.

- [ ] **Step 1: Create test-utils.tsx**

```tsx
import { render } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Toaster } from "sonner";
import type { ReactElement, ReactNode } from "react";

export function makeTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
}

export function renderWithProviders(
  ui: ReactElement,
  { withToaster = true }: { withToaster?: boolean } = {}
) {
  const queryClient = makeTestQueryClient();
  return render(
    <QueryClientProvider client={queryClient}>
      {withToaster && <Toaster />}
      {ui}
    </QueryClientProvider>
  );
}

export function renderHookWithClient() {
  const queryClient = makeTestQueryClient();
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  return { wrapper, queryClient };
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd bohikor && npm run typecheck`
Expected: PASS (this file has no consumers yet, but must be syntactically/type valid).

- [ ] **Step 3: Commit**

```bash
cd /Users/carlson/space/bohikor2
git add bohikor/src/test-utils.tsx
git commit -m "test(bohikor): add shared renderWithProviders/renderHookWithClient test helpers"
```

---

### Task 3: `lib/auth.ts` tests

**Files:**
- Create: `bohikor/src/lib/__tests__/auth.test.ts`

**Interfaces:**
- Consumes: `getAccessToken`, `getRefreshToken`, `setTokens`, `clearTokens`, `getSubjectHint`, `setSubjectHint` from `@/lib/auth` (all real, unmocked — pure localStorage wrappers, jsdom provides `localStorage` natively).

- [ ] **Step 1: Write the test file**

```ts
import {
  getAccessToken,
  getRefreshToken,
  setTokens,
  clearTokens,
  getSubjectHint,
  setSubjectHint,
} from "../auth";

describe("lib/auth", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it("returns null for token/hint getters when nothing is stored", () => {
    expect(getAccessToken()).toBeNull();
    expect(getRefreshToken()).toBeNull();
    expect(getSubjectHint()).toBeNull();
  });

  it("setTokens then getAccessToken/getRefreshToken round-trip", () => {
    setTokens("at-123", "rt-456");
    expect(getAccessToken()).toBe("at-123");
    expect(getRefreshToken()).toBe("rt-456");
  });

  it("setSubjectHint then getSubjectHint round-trips", () => {
    setSubjectHint("admin");
    expect(getSubjectHint()).toBe("admin");

    setSubjectHint("platform_admin");
    expect(getSubjectHint()).toBe("platform_admin");
  });

  it("clearTokens removes access token, refresh token, and subject hint", () => {
    setTokens("at-123", "rt-456");
    setSubjectHint("admin");

    clearTokens();

    expect(getAccessToken()).toBeNull();
    expect(getRefreshToken()).toBeNull();
    expect(getSubjectHint()).toBeNull();
  });
});
```

- [ ] **Step 2: Run the test**

Run: `cd bohikor && npx jest src/lib/__tests__/auth.test.ts -v`
Expected: PASS, 4/4.

- [ ] **Step 3: Commit**

```bash
cd /Users/carlson/space/bohikor2
git add bohikor/src/lib/__tests__/auth.test.ts
git commit -m "test(bohikor): add lib/auth.ts coverage"
```

---

### Task 4: `lib/api.ts` tests (axios 401-refresh interceptor — highest risk file)

**Files:**
- Create: `bohikor/src/lib/__tests__/api.test.ts`

**Interfaces:**
- Consumes: the real `api` axios instance from `@/lib/api` (re-imported fresh per test via `jest.resetModules()` + dynamic `import()` to avoid the module-level `isRefreshing`/`refreshSubscribers` singletons leaking across tests). Mocks: `axios.post` (for the `/api/auth/refresh` call), and `@/lib/auth`'s `getAccessToken`/`getRefreshToken`/`setTokens`/`clearTokens`.

- [ ] **Step 1: Write the test file**

```ts
import axios from "axios";

jest.mock("../auth", () => ({
  getAccessToken: jest.fn(),
  getRefreshToken: jest.fn(),
  setTokens: jest.fn(),
  clearTokens: jest.fn(),
}));

describe("lib/api", () => {
  let authMock: {
    getAccessToken: jest.Mock;
    getRefreshToken: jest.Mock;
    setTokens: jest.Mock;
    clearTokens: jest.Mock;
  };

  beforeEach(() => {
    jest.resetModules();
    authMock = jest.requireMock("../auth");
    authMock.getAccessToken.mockReset();
    authMock.getRefreshToken.mockReset();
    authMock.setTokens.mockReset();
    authMock.clearTokens.mockReset();
    jest.spyOn(axios, "post").mockReset();
    Object.defineProperty(window, "location", {
      writable: true,
      value: { href: "" },
    });
  });

  it("attaches Authorization header when a token exists", async () => {
    authMock.getAccessToken.mockReturnValue("tok-123");
    const { api } = await import("../api");
    const adapter = jest.fn().mockResolvedValue({
      status: 200,
      data: {},
      headers: {},
      config: {},
    });
    api.defaults.adapter = adapter;

    await api.get("/api/ping");

    expect(adapter.mock.calls[0][0].headers.Authorization).toBe("Bearer tok-123");
  });

  it("omits Authorization header when no token exists", async () => {
    authMock.getAccessToken.mockReturnValue(null);
    const { api } = await import("../api");
    const adapter = jest.fn().mockResolvedValue({
      status: 200,
      data: {},
      headers: {},
      config: {},
    });
    api.defaults.adapter = adapter;

    await api.get("/api/ping");

    expect(adapter.mock.calls[0][0].headers.Authorization).toBeUndefined();
  });

  it("passes through a successful response with no refresh call", async () => {
    authMock.getAccessToken.mockReturnValue("tok-123");
    const { api } = await import("../api");
    const adapter = jest.fn().mockResolvedValue({
      status: 200,
      data: { ok: true },
      headers: {},
      config: {},
    });
    api.defaults.adapter = adapter;

    const res = await api.get("/api/ping");

    expect(res.data).toEqual({ ok: true });
    expect(axios.post).not.toHaveBeenCalled();
  });

  it("on 401, refreshes the token and retries the original request", async () => {
    authMock.getAccessToken.mockReturnValue("old-tok");
    authMock.getRefreshToken.mockReturnValue("refresh-tok");
    const { api } = await import("../api");

    const adapter = jest
      .fn()
      .mockImplementationOnce(() => {
        const err: unknown = new Error("Unauthorized");
        (err as { response: unknown; config: unknown }).response = { status: 401, data: {} };
        (err as { config: unknown }).config = { headers: {}, _retry: false };
        return Promise.reject(err);
      })
      .mockImplementationOnce(() =>
        Promise.resolve({ status: 200, data: { ok: true }, headers: {}, config: {} })
      );
    api.defaults.adapter = adapter;

    (axios.post as jest.Mock).mockResolvedValue({
      data: { data: { access_token: "new-tok", refresh_token: "new-refresh" } },
    });

    const res = await api.get("/api/protected");

    expect(authMock.setTokens).toHaveBeenCalledWith("new-tok", "new-refresh");
    expect(res.data).toEqual({ ok: true });
    expect(adapter.mock.calls[1][0].headers.Authorization).toBe("Bearer new-tok");
  });

  it("on 401 with no refresh token, clears tokens and redirects to /", async () => {
    authMock.getAccessToken.mockReturnValue("old-tok");
    authMock.getRefreshToken.mockReturnValue(null);
    const { api } = await import("../api");

    const adapter = jest.fn().mockImplementationOnce(() => {
      const err: unknown = new Error("Unauthorized");
      (err as { response: unknown }).response = { status: 401, data: {} };
      (err as { config: unknown }).config = { headers: {}, _retry: false };
      return Promise.reject(err);
    });
    api.defaults.adapter = adapter;

    await expect(api.get("/api/protected")).rejects.toBeTruthy();

    expect(authMock.clearTokens).toHaveBeenCalled();
    expect(window.location.href).toBe("/");
  });

  it("on 401 when the refresh call itself fails, clears tokens and redirects to /", async () => {
    authMock.getAccessToken.mockReturnValue("old-tok");
    authMock.getRefreshToken.mockReturnValue("refresh-tok");
    const { api } = await import("../api");

    const adapter = jest.fn().mockImplementationOnce(() => {
      const err: unknown = new Error("Unauthorized");
      (err as { response: unknown }).response = { status: 401, data: {} };
      (err as { config: unknown }).config = { headers: {}, _retry: false };
      return Promise.reject(err);
    });
    api.defaults.adapter = adapter;

    (axios.post as jest.Mock).mockRejectedValue(new Error("refresh failed"));

    await expect(api.get("/api/protected")).rejects.toBeTruthy();

    expect(authMock.clearTokens).toHaveBeenCalled();
    expect(window.location.href).toBe("/");
  });

  it("queues concurrent 401s behind a single in-flight refresh", async () => {
    authMock.getAccessToken.mockReturnValue("old-tok");
    authMock.getRefreshToken.mockReturnValue("refresh-tok");
    const { api } = await import("../api");

    const make401 = () => {
      const err: unknown = new Error("Unauthorized");
      (err as { response: unknown }).response = { status: 401, data: {} };
      (err as { config: unknown }).config = { headers: {}, _retry: false };
      return Promise.reject(err);
    };

    const adapter = jest
      .fn()
      .mockImplementationOnce(make401)
      .mockImplementationOnce(make401)
      .mockImplementation(() =>
        Promise.resolve({ status: 200, data: { ok: true }, headers: {}, config: {} })
      );
    api.defaults.adapter = adapter;

    let resolveRefresh: (v: unknown) => void = () => {};
    (axios.post as jest.Mock).mockReturnValue(
      new Promise((resolve) => {
        resolveRefresh = resolve;
      })
    );

    const p1 = api.get("/api/a");
    const p2 = api.get("/api/b");

    // Let both 401s fire before the refresh resolves.
    await new Promise((r) => setTimeout(r, 0));
    expect(axios.post).toHaveBeenCalledTimes(1);

    resolveRefresh({
      data: { data: { access_token: "new-tok", refresh_token: "new-refresh" } },
    });

    const [res1, res2] = await Promise.all([p1, p2]);
    expect(res1.data).toEqual({ ok: true });
    expect(res2.data).toEqual({ ok: true });
  });
});
```

- [ ] **Step 2: Run the test**

Run: `cd bohikor && npx jest src/lib/__tests__/api.test.ts -v`
Expected: PASS, 8/8. If the concurrent-401 test is flaky or the adapter-swap approach proves unworkable for that specific case, add `axios-mock-adapter` as a devDependency (`npm install --save-dev axios-mock-adapter` from `bohikor/`) and rewrite that one test using it — do not delete the test case, only change its mocking mechanism, and note the dependency addition in the commit message.

- [ ] **Step 3: Verify getApiErrorMessage separately**

Add to the same file (or a second `describe` block) tests for the exported `getApiErrorMessage`:

```ts
import { getApiErrorMessage } from "../api";

describe("getApiErrorMessage", () => {
  it("returns the response error message when present", () => {
    const err = {
      isAxiosError: true,
      response: { status: 400, data: { error: "Bad input" } },
    };
    jest.spyOn(axios, "isAxiosError").mockReturnValue(true);
    expect(getApiErrorMessage(err)).toBe("Bad input");
  });

  it("returns a session-expired message for a bare 401", () => {
    const err = {
      isAxiosError: true,
      response: { status: 401, data: {} },
    };
    jest.spyOn(axios, "isAxiosError").mockReturnValue(true);
    expect(getApiErrorMessage(err)).toBe("Session expired. Please log in again.");
  });

  it("returns a generic fallback for a non-axios error", () => {
    jest.spyOn(axios, "isAxiosError").mockReturnValue(false);
    expect(getApiErrorMessage(new Error("boom"))).toBe(
      "Something went wrong. Please try again."
    );
  });
});
```

Run: `cd bohikor && npx jest src/lib/__tests__/api.test.ts -v`
Expected: PASS, 11/11 total in the file.

- [ ] **Step 4: Commit**

```bash
cd /Users/carlson/space/bohikor2
git add bohikor/src/lib/__tests__/api.test.ts bohikor/package.json bohikor/package-lock.json
git commit -m "test(bohikor): add lib/api.ts coverage (401 refresh interceptor)"
```

---

### Task 5: Hooks tests (6 files)

**Files:**
- Create: `bohikor/src/hooks/__tests__/use-admin.test.ts`
- Create: `bohikor/src/hooks/__tests__/use-events.test.ts`
- Create: `bohikor/src/hooks/__tests__/use-invitations.test.ts`
- Create: `bohikor/src/hooks/__tests__/use-requests.test.ts`
- Create: `bohikor/src/hooks/__tests__/use-settings.test.ts`
- Create: `bohikor/src/hooks/__tests__/use-users.test.ts`

**Interfaces:**
- Consumes: `renderHookWithClient` from `@/test-utils` (Task 2). Mocks `@/lib/api`'s `api` object per file (`{ get: jest.fn(), post: jest.fn(), put: jest.fn() }`).

- [ ] **Step 1: use-admin.test.ts**

```ts
import { renderHook, waitFor } from "@testing-library/react";
import { useAdmin } from "../use-admin";
import { renderHookWithClient } from "@/test-utils";

jest.mock("@/lib/api", () => ({ api: { get: jest.fn() } }));
const { api } = jest.requireMock("@/lib/api");

describe("useAdmin", () => {
  beforeEach(() => jest.clearAllMocks());

  it("fetches /api/admin/me and returns the admin", async () => {
    api.get.mockResolvedValue({ data: { data: { id: "1", email: "a@b.com" } } });
    const { wrapper } = renderHookWithClient();
    const { result } = renderHook(() => useAdmin(), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(api.get).toHaveBeenCalledWith("/api/admin/me");
    expect(result.current.data).toEqual({ id: "1", email: "a@b.com" });
  });

  it("does not fetch when enabled=false", () => {
    const { wrapper } = renderHookWithClient();
    renderHook(() => useAdmin(false), { wrapper });

    expect(api.get).not.toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: use-events.test.ts**

```ts
import { renderHook, waitFor } from "@testing-library/react";
import { useEvents } from "../use-events";
import { renderHookWithClient } from "@/test-utils";

jest.mock("@/lib/api", () => ({ api: { get: jest.fn() } }));
const { api } = jest.requireMock("@/lib/api");

describe("useEvents", () => {
  beforeEach(() => jest.clearAllMocks());

  it("fetches /api/admin/events with default page/perPage", async () => {
    api.get.mockResolvedValue({ data: { data: [{ id: "e1" }] } });
    const { wrapper } = renderHookWithClient();
    const { result } = renderHook(() => useEvents(), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(api.get).toHaveBeenCalledWith("/api/admin/events", {
      params: { page: 1, per_page: 50 },
    });
    expect(result.current.data).toEqual([{ id: "e1" }]);
  });

  it("fetches with explicit page/perPage", async () => {
    api.get.mockResolvedValue({ data: { data: [] } });
    const { wrapper } = renderHookWithClient();
    const { result } = renderHook(() => useEvents(2, 10), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(api.get).toHaveBeenCalledWith("/api/admin/events", {
      params: { page: 2, per_page: 10 },
    });
  });
});
```

- [ ] **Step 3: use-invitations.test.ts**

```ts
import { renderHook, waitFor } from "@testing-library/react";
import { useInvitations, useSendInvite } from "../use-invitations";
import { renderHookWithClient } from "@/test-utils";

jest.mock("@/lib/api", () => ({ api: { get: jest.fn(), post: jest.fn() } }));
const { api } = jest.requireMock("@/lib/api");

describe("useInvitations", () => {
  beforeEach(() => jest.clearAllMocks());

  it("fetches /api/admin/invitations", async () => {
    api.get.mockResolvedValue({ data: { data: [{ id: "i1" }] } });
    const { wrapper } = renderHookWithClient();
    const { result } = renderHook(() => useInvitations(), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(api.get).toHaveBeenCalledWith("/api/admin/invitations");
    expect(result.current.data).toEqual([{ id: "i1" }]);
  });
});

describe("useSendInvite", () => {
  beforeEach(() => jest.clearAllMocks());

  it("posts to /api/admin/invite and invalidates the invitations query", async () => {
    api.post.mockResolvedValue({ data: { data: { id: "i2", email: "x@y.com" } } });
    const { wrapper, queryClient } = renderHookWithClient();
    const invalidateSpy = jest.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useSendInvite(), { wrapper });

    result.current.mutate("x@y.com");

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(api.post).toHaveBeenCalledWith("/api/admin/invite", { email: "x@y.com" });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["invitations"] });
  });
});
```

- [ ] **Step 4: use-requests.test.ts**

```ts
import { renderHook, waitFor } from "@testing-library/react";
import { useRequests } from "../use-requests";
import { renderHookWithClient } from "@/test-utils";

jest.mock("@/lib/api", () => ({ api: { get: jest.fn() } }));
const { api } = jest.requireMock("@/lib/api");

describe("useRequests", () => {
  beforeEach(() => jest.clearAllMocks());

  it("fetches /api/admin/requests with default page/perPage", async () => {
    api.get.mockResolvedValue({ data: { data: [{ id: "r1" }] } });
    const { wrapper } = renderHookWithClient();
    const { result } = renderHook(() => useRequests(), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(api.get).toHaveBeenCalledWith("/api/admin/requests", {
      params: { page: 1, per_page: 20 },
    });
  });

  it("fetches with explicit page/perPage", async () => {
    api.get.mockResolvedValue({ data: { data: [] } });
    const { wrapper } = renderHookWithClient();
    const { result } = renderHook(() => useRequests(3, 5), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(api.get).toHaveBeenCalledWith("/api/admin/requests", {
      params: { page: 3, per_page: 5 },
    });
  });
});
```

- [ ] **Step 5: use-settings.test.ts**

```ts
import { renderHook, waitFor } from "@testing-library/react";
import { useSettings, useUpdateSetting } from "../use-settings";
import { renderHookWithClient } from "@/test-utils";

jest.mock("@/lib/api", () => ({ api: { get: jest.fn(), put: jest.fn() } }));
const { api } = jest.requireMock("@/lib/api");

describe("useSettings", () => {
  beforeEach(() => jest.clearAllMocks());

  it("fetches settings and coerces values to strings keyed by setting key", async () => {
    api.get.mockResolvedValue({
      data: { data: [{ key: "a", value: 5 }, { key: "b", value: true }] },
    });
    const { wrapper } = renderHookWithClient();
    const { result } = renderHook(() => useSettings(), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(api.get).toHaveBeenCalledWith("/api/admin/settings");
    expect(result.current.data).toEqual({ a: "5", b: "true" });
  });
});

describe("useUpdateSetting", () => {
  beforeEach(() => jest.clearAllMocks());

  it("puts to /api/admin/settings with the key/value body and invalidates settings", async () => {
    api.put.mockResolvedValue({ data: { data: { advance_amount_xaf: "6000" } } });
    const { wrapper, queryClient } = renderHookWithClient();
    const invalidateSpy = jest.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useUpdateSetting(), { wrapper });

    result.current.mutate({ key: "advance_amount_xaf", value: "6000" });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(api.put).toHaveBeenCalledWith("/api/admin/settings", {
      advance_amount_xaf: "6000",
    });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["settings"] });
  });
});
```

- [ ] **Step 6: use-users.test.ts**

```ts
import { renderHook, waitFor } from "@testing-library/react";
import {
  useUsers,
  useSuspendUser,
  useActivateUser,
  useUnlockUser,
} from "../use-users";
import { renderHookWithClient } from "@/test-utils";

jest.mock("@/lib/api", () => ({ api: { get: jest.fn(), put: jest.fn() } }));
const { api } = jest.requireMock("@/lib/api");

describe("useUsers", () => {
  beforeEach(() => jest.clearAllMocks());

  it("fetches /api/admin/users with default page/perPage", async () => {
    api.get.mockResolvedValue({ data: { data: [{ id: "u1" }] } });
    const { wrapper } = renderHookWithClient();
    const { result } = renderHook(() => useUsers(), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(api.get).toHaveBeenCalledWith("/api/admin/users", {
      params: { page: 1, per_page: 20 },
    });
  });

  it("fetches with explicit page/perPage", async () => {
    api.get.mockResolvedValue({ data: { data: [] } });
    const { wrapper } = renderHookWithClient();
    const { result } = renderHook(() => useUsers(2, 10), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(api.get).toHaveBeenCalledWith("/api/admin/users", {
      params: { page: 2, per_page: 10 },
    });
  });
});

describe.each([
  ["useSuspendUser", useSuspendUser, "suspend"],
  ["useActivateUser", useActivateUser, "activate"],
  ["useUnlockUser", useUnlockUser, "unlock"],
] as const)("%s", (_name, hook, action) => {
  beforeEach(() => jest.clearAllMocks());

  it(`puts to /api/admin/users/:id/${action} and invalidates users`, async () => {
    api.put.mockResolvedValue({ data: { id: "u1", status: "active" } });
    const { wrapper, queryClient } = renderHookWithClient();
    const invalidateSpy = jest.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => hook(), { wrapper });

    result.current.mutate("u1");

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(api.put).toHaveBeenCalledWith(`/api/admin/users/u1/${action}`);
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["users"] });
  });
});
```

- [ ] **Step 7: Run all hook tests**

Run: `cd bohikor && npx jest src/hooks/__tests__ -v`
Expected: PASS, all suites green.

- [ ] **Step 8: Commit**

```bash
cd /Users/carlson/space/bohikor2
git add bohikor/src/hooks/__tests__
git commit -m "test(bohikor): add coverage for all 6 data hooks"
```

---

### Task 6: `auth-provider.tsx` tests

**Files:**
- Create: `bohikor/src/components/providers/__tests__/auth-provider.test.tsx`

**Interfaces:**
- Consumes: `AuthProvider`, `useAuth` from `../auth-provider`. Mocks `@/lib/api` (`api.get`, `api.post`) and `@/lib/auth` (`getAccessToken`, `getSubjectHint`, `clearTokens`).

- [ ] **Step 1: Write the test file**

```tsx
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AuthProvider, useAuth } from "../auth-provider";

jest.mock("@/lib/api", () => ({ api: { get: jest.fn(), post: jest.fn() } }));
jest.mock("@/lib/auth", () => ({
  getAccessToken: jest.fn(),
  getSubjectHint: jest.fn(),
  clearTokens: jest.fn(),
}));

const { api } = jest.requireMock("@/lib/api");
const { getAccessToken, getSubjectHint, clearTokens } = jest.requireMock("@/lib/auth");

function Probe() {
  const ctx = useAuth();
  return (
    <div>
      <div data-testid="state">
        {JSON.stringify({ admin: ctx.admin, subjectType: ctx.subjectType, loading: ctx.loading })}
      </div>
      <button onClick={() => ctx.signOut()}>sign out</button>
      <button onClick={() => ctx.refreshSubject()}>refresh</button>
    </div>
  );
}

describe("AuthProvider", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("no token or no hint results in unauthenticated state without an api call", async () => {
    getAccessToken.mockReturnValue(null);
    getSubjectHint.mockReturnValue(null);

    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>
    );

    await waitFor(() =>
      expect(screen.getByTestId("state")).toHaveTextContent(
        JSON.stringify({ admin: null, subjectType: null, loading: false })
      )
    );
    expect(api.get).not.toHaveBeenCalled();
  });

  it("platform_admin hint short-circuits without calling /api/admin/me", async () => {
    getAccessToken.mockReturnValue("tok");
    getSubjectHint.mockReturnValue("platform_admin");

    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>
    );

    await waitFor(() =>
      expect(screen.getByTestId("state")).toHaveTextContent(
        JSON.stringify({ admin: null, subjectType: "platform_admin", loading: false })
      )
    );
    expect(api.get).not.toHaveBeenCalled();
  });

  it("admin hint fetches /api/admin/me and populates admin on success", async () => {
    getAccessToken.mockReturnValue("tok");
    getSubjectHint.mockReturnValue("admin");
    const adminFixture = { id: "1", email: "a@b.com", company_slug: "acme", created_at: "now" };
    api.get.mockResolvedValue({ data: { data: adminFixture } });

    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>
    );

    await waitFor(() =>
      expect(screen.getByTestId("state")).toHaveTextContent(
        JSON.stringify({ admin: adminFixture, subjectType: "admin", loading: false })
      )
    );
  });

  it("admin hint clears tokens and stays unauthenticated when /api/admin/me fails", async () => {
    getAccessToken.mockReturnValue("tok");
    getSubjectHint.mockReturnValue("admin");
    api.get.mockRejectedValue(new Error("401"));

    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>
    );

    await waitFor(() =>
      expect(screen.getByTestId("state")).toHaveTextContent(
        JSON.stringify({ admin: null, subjectType: null, loading: false })
      )
    );
    expect(clearTokens).toHaveBeenCalled();
  });

  it("signOut posts to /api/auth/logout, clears tokens, and does not reset context state", async () => {
    getAccessToken.mockReturnValue("tok");
    getSubjectHint.mockReturnValue("admin");
    const adminFixture = { id: "1", email: "a@b.com", company_slug: "acme", created_at: "now" };
    api.get.mockResolvedValue({ data: { data: adminFixture } });
    api.post.mockResolvedValue({});

    const user = userEvent.setup();
    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>
    );

    await waitFor(() =>
      expect(screen.getByTestId("state")).toHaveTextContent('"subjectType":"admin"')
    );

    await user.click(screen.getByText("sign out"));

    await waitFor(() => expect(api.post).toHaveBeenCalledWith("/api/auth/logout"));
    expect(clearTokens).toHaveBeenCalled();
    // signOut does not itself reset admin/subjectType state.
    expect(screen.getByTestId("state")).toHaveTextContent('"subjectType":"admin"');
  });

  it("signOut swallows a failing logout call and still clears tokens", async () => {
    getAccessToken.mockReturnValue("tok");
    getSubjectHint.mockReturnValue("admin");
    api.get.mockResolvedValue({
      data: { data: { id: "1", email: "a@b.com", company_slug: "acme", created_at: "now" } },
    });
    api.post.mockRejectedValue(new Error("network"));

    const user = userEvent.setup();
    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>
    );

    await waitFor(() =>
      expect(screen.getByTestId("state")).toHaveTextContent('"subjectType":"admin"')
    );

    await user.click(screen.getByText("sign out"));

    await waitFor(() => expect(clearTokens).toHaveBeenCalled());
  });

  it("refreshSubject re-runs the loader and picks up new data", async () => {
    getAccessToken.mockReturnValue("tok");
    getSubjectHint.mockReturnValue("admin");
    const first = { id: "1", email: "a@b.com", company_slug: "acme", created_at: "now" };
    const second = { id: "1", email: "a@b.com", company_slug: "other", created_at: "now" };
    api.get.mockResolvedValueOnce({ data: { data: first } }).mockResolvedValueOnce({
      data: { data: second },
    });

    const user = userEvent.setup();
    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>
    );

    await waitFor(() =>
      expect(screen.getByTestId("state")).toHaveTextContent('"company_slug":"acme"')
    );

    await user.click(screen.getByText("refresh"));

    await waitFor(() =>
      expect(screen.getByTestId("state")).toHaveTextContent('"company_slug":"other"')
    );
  });
});
```

- [ ] **Step 2: Run the test**

Run: `cd bohikor && npx jest src/components/providers/__tests__/auth-provider.test.tsx -v`
Expected: PASS, 7/7.

- [ ] **Step 3: Commit**

```bash
cd /Users/carlson/space/bohikor2
git add bohikor/src/components/providers/__tests__/auth-provider.test.tsx
git commit -m "test(bohikor): add AuthProvider coverage (all subject-hint branches)"
```

---

### Task 7: `auth-guard.tsx` tests

**Files:**
- Create: `bohikor/src/components/__tests__/auth-guard.test.tsx`

**Interfaces:**
- Consumes: `AuthGuard` from `../auth-guard`. Mocks `@/components/providers` (`useAuth`) and `next/navigation` (`useRouter`).

- [ ] **Step 1: Write the test file**

```tsx
import { render, screen } from "@testing-library/react";
import { AuthGuard } from "../auth-guard";

const mockPush = jest.fn();

jest.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush }),
}));

jest.mock("@/components/providers", () => ({
  useAuth: jest.fn(),
}));

const { useAuth } = jest.requireMock("@/components/providers");

describe("AuthGuard", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("shows only a spinner while loading, does not push, does not render children", () => {
    useAuth.mockReturnValue({ subjectType: null, loading: true });

    const { container } = render(
      <AuthGuard loginHref="/acme/admin/login">
        <div>secret content</div>
      </AuthGuard>
    );

    expect(screen.queryByText("secret content")).not.toBeInTheDocument();
    expect(container.querySelector(".animate-spin")).toBeInTheDocument();
    expect(mockPush).not.toHaveBeenCalled();
  });

  it("redirects to loginHref and renders nothing when unauthenticated", () => {
    useAuth.mockReturnValue({ subjectType: null, loading: false });

    render(
      <AuthGuard loginHref="/acme/admin/login">
        <div>secret content</div>
      </AuthGuard>
    );

    expect(screen.queryByText("secret content")).not.toBeInTheDocument();
    expect(mockPush).toHaveBeenCalledWith("/acme/admin/login");
  });

  it("renders children and does not redirect when authenticated", () => {
    useAuth.mockReturnValue({ subjectType: "admin", loading: false });

    render(
      <AuthGuard loginHref="/acme/admin/login">
        <div>secret content</div>
      </AuthGuard>
    );

    expect(screen.getByText("secret content")).toBeInTheDocument();
    expect(mockPush).not.toHaveBeenCalled();
  });

  it("redirects only after transitioning from authenticated to unauthenticated", () => {
    useAuth.mockReturnValue({ subjectType: "admin", loading: false });
    const { rerender: rerenderComponent } = render(
      <AuthGuard loginHref="/acme/admin/login">
        <div>secret content</div>
      </AuthGuard>
    );
    expect(mockPush).not.toHaveBeenCalled();

    useAuth.mockReturnValue({ subjectType: null, loading: false });
    rerenderComponent(
      <AuthGuard loginHref="/acme/admin/login">
        <div>secret content</div>
      </AuthGuard>
    );

    expect(mockPush).toHaveBeenCalledWith("/acme/admin/login");
  });
});
```

- [ ] **Step 2: Run the test**

Run: `cd bohikor && npx jest src/components/__tests__/auth-guard.test.tsx -v`
Expected: PASS, 4/4.

- [ ] **Step 3: Commit**

```bash
cd /Users/carlson/space/bohikor2
git add bohikor/src/components/__tests__/auth-guard.test.tsx
git commit -m "test(bohikor): add AuthGuard coverage"
```

---

### Task 8: `forbidden.tsx` tests

**Files:**
- Create: `bohikor/src/components/__tests__/forbidden.test.tsx`

**Interfaces:**
- Consumes: `ForbiddenPage` from `../forbidden`. Mocks `@/components/providers` (`useAuth`) and `next/navigation` (`useRouter`).

- [ ] **Step 1: Write the test file**

```tsx
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ForbiddenPage } from "../forbidden";

const mockPush = jest.fn();
const mockBack = jest.fn();
const mockSignOut = jest.fn().mockResolvedValue(undefined);

jest.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, back: mockBack }),
}));

jest.mock("@/components/providers", () => ({
  useAuth: () => ({ signOut: mockSignOut }),
}));

describe("ForbiddenPage", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("renders the default 403 content and label", () => {
    render(<ForbiddenPage />);

    expect(screen.getByText("403")).toBeInTheDocument();
    expect(screen.getByText("Access Denied")).toBeInTheDocument();
    expect(screen.getByText("Go to Login")).toBeInTheDocument();
  });

  it("renders a custom backLabel when provided", () => {
    render(<ForbiddenPage backHref="/acme/admin/login" backLabel="Back to sign in" />);

    expect(screen.getByText("Back to sign in")).toBeInTheDocument();
  });

  it("calls signOut then pushes backHref when the primary button is clicked", async () => {
    const user = userEvent.setup();
    render(<ForbiddenPage backHref="/acme/admin/login" backLabel="Back to sign in" />);

    await user.click(screen.getByText("Back to sign in"));

    expect(mockSignOut).toHaveBeenCalled();
    expect(mockPush).toHaveBeenCalledWith("/acme/admin/login");
  });

  it("calls router.back when Go Back is clicked", async () => {
    const user = userEvent.setup();
    render(<ForbiddenPage />);

    await user.click(screen.getByText("Go Back"));

    expect(mockBack).toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run the test**

Run: `cd bohikor && npx jest src/components/__tests__/forbidden.test.tsx -v`
Expected: PASS, 4/4.

- [ ] **Step 3: Commit**

```bash
cd /Users/carlson/space/bohikor2
git add bohikor/src/components/__tests__/forbidden.test.tsx
git commit -m "test(bohikor): add ForbiddenPage coverage"
```

---

### Task 9: `sidebar.tsx` tests

**Files:**
- Create: `bohikor/src/components/__tests__/sidebar.test.tsx`

**Interfaces:**
- Consumes: `Sidebar` from `../sidebar`. Mocks `next/navigation` (`usePathname`, `useParams`) and `@/components/providers` (`useAuth`).

- [ ] **Step 1: Write the test file**

```tsx
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Sidebar } from "../sidebar";

const mockSignOut = jest.fn();

jest.mock("next/navigation", () => ({
  usePathname: jest.fn(),
  useParams: () => ({ company: "acme" }),
}));

jest.mock("@/components/providers", () => ({
  useAuth: () => ({ signOut: mockSignOut }),
}));

const { usePathname } = jest.requireMock("next/navigation");

describe("Sidebar", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    usePathname.mockReturnValue("/acme/admin");
  });

  it("renders all 7 nav links with slug-prefixed hrefs", () => {
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
      const link = screen.getByText(label).closest("a");
      expect(link).toHaveAttribute("href", href);
    }
  });

  it("applies the active styling class to the current route's link", () => {
    usePathname.mockReturnValue("/acme/admin/users");
    render(<Sidebar />);

    const usersLink = screen.getByText("Users").closest("a");
    const dashboardLink = screen.getByText("Dashboard").closest("a");

    expect(usersLink?.className).toContain("bg-sidebar-primary");
    expect(dashboardLink?.className).not.toContain("bg-sidebar-primary");
  });

  it("calls signOut when the sign out button is clicked", async () => {
    const user = userEvent.setup();
    render(<Sidebar />);

    await user.click(screen.getByText("Sign Out"));

    expect(mockSignOut).toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run the test**

Run: `cd bohikor && npx jest src/components/__tests__/sidebar.test.tsx -v`
Expected: PASS, 3/3.

- [ ] **Step 3: Commit**

```bash
cd /Users/carlson/space/bohikor2
git add bohikor/src/components/__tests__/sidebar.test.tsx
git commit -m "test(bohikor): add Sidebar coverage"
```

---

### Task 10: `[company]/layout.tsx` tests

**Files:**
- Create: `bohikor/src/app/[company]/__tests__/layout.test.tsx`

**Interfaces:**
- Consumes: default export from `../layout`. Mocks `next/navigation` (`useParams`, `useRouter`) and `@/components/providers` (`useAuth`).

- [ ] **Step 1: Write the test file**

```tsx
import { render } from "@testing-library/react";
import CompanyLayout from "../layout";

const mockReplace = jest.fn();

jest.mock("next/navigation", () => ({
  useParams: () => ({ company: "acme" }),
  useRouter: () => ({ replace: mockReplace }),
}));

jest.mock("@/components/providers", () => ({
  useAuth: jest.fn(),
}));

const { useAuth } = jest.requireMock("@/components/providers");

describe("CompanyLayout", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("does not redirect while loading", () => {
    useAuth.mockReturnValue({ admin: null, subjectType: "admin", loading: true });
    render(
      <CompanyLayout>
        <div>child</div>
      </CompanyLayout>
    );
    expect(mockReplace).not.toHaveBeenCalled();
  });

  it("does not redirect for a non-admin subjectType", () => {
    useAuth.mockReturnValue({ admin: null, subjectType: "platform_admin", loading: false });
    render(
      <CompanyLayout>
        <div>child</div>
      </CompanyLayout>
    );
    expect(mockReplace).not.toHaveBeenCalled();
  });

  it("does not redirect when the admin's company_slug matches the URL param", () => {
    useAuth.mockReturnValue({
      admin: { company_slug: "acme" },
      subjectType: "admin",
      loading: false,
    });
    render(
      <CompanyLayout>
        <div>child</div>
      </CompanyLayout>
    );
    expect(mockReplace).not.toHaveBeenCalled();
  });

  it("redirects to the admin's own company when the slug mismatches", () => {
    useAuth.mockReturnValue({
      admin: { company_slug: "other" },
      subjectType: "admin",
      loading: false,
    });
    render(
      <CompanyLayout>
        <div>child</div>
      </CompanyLayout>
    );
    expect(mockReplace).toHaveBeenCalledWith("/other/admin");
  });
});
```

- [ ] **Step 2: Run the test**

Run: `cd bohikor && npx jest "src/app/\[company\]/__tests__/layout.test.tsx" -v`
Expected: PASS, 4/4.

- [ ] **Step 3: Commit**

```bash
cd /Users/carlson/space/bohikor2
git add "bohikor/src/app/[company]/__tests__/layout.test.tsx"
git commit -m "test(bohikor): add [company]/layout.tsx slug-mismatch guard coverage"
```

---

### Task 11: `[company]/admin/(protected)/layout.tsx` tests

**Files:**
- Create: `bohikor/src/app/[company]/admin/(protected)/__tests__/layout.test.tsx`

**Interfaces:**
- Consumes: default export from `../layout`. Mocks `next/navigation` (`useParams`), `@/components/auth-guard` (`AuthGuard` — stubbed to a passthrough so this test isolates the `AdminCheck` logic, since `AuthGuard` itself has its own dedicated tests from Task 7), `@/hooks/use-admin` (`useAdmin`). Uses the real `@/components/forbidden` and `@/components/sidebar`, which in turn need `@/components/providers` (`useAuth`) and `next/navigation` (`usePathname`, `useParams`) mocked transitively.

- [ ] **Step 1: Write the test file**

```tsx
import { render, screen } from "@testing-library/react";
import CompanyAdminLayout from "../layout";

jest.mock("next/navigation", () => ({
  useParams: () => ({ company: "acme" }),
  usePathname: () => "/acme/admin",
  useRouter: () => ({ push: jest.fn(), back: jest.fn() }),
}));

jest.mock("@/components/auth-guard", () => ({
  AuthGuard: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

jest.mock("@/components/providers", () => ({
  useAuth: () => ({ signOut: jest.fn() }),
}));

jest.mock("@/hooks/use-admin", () => ({
  useAdmin: jest.fn(),
}));

const { useAdmin } = jest.requireMock("@/hooks/use-admin");

describe("CompanyAdminLayout", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("shows only a spinner while the admin profile is loading", () => {
    useAdmin.mockReturnValue({ data: undefined, isLoading: true });
    const { container } = render(
      <CompanyAdminLayout>
        <div>dashboard content</div>
      </CompanyAdminLayout>
    );

    expect(screen.queryByText("dashboard content")).not.toBeInTheDocument();
    expect(screen.queryByText("403")).not.toBeInTheDocument();
    expect(container.querySelector(".animate-spin")).toBeInTheDocument();
  });

  it("renders ForbiddenPage when the admin profile fetch returns no admin", () => {
    useAdmin.mockReturnValue({ data: undefined, isLoading: false });
    render(
      <CompanyAdminLayout>
        <div>dashboard content</div>
      </CompanyAdminLayout>
    );

    expect(screen.getByText("403")).toBeInTheDocument();
    expect(screen.queryByText("dashboard content")).not.toBeInTheDocument();
  });

  it("renders the Sidebar and children when an admin is present", () => {
    useAdmin.mockReturnValue({ data: { id: "1", email: "a@b.com" }, isLoading: false });
    render(
      <CompanyAdminLayout>
        <div>dashboard content</div>
      </CompanyAdminLayout>
    );

    expect(screen.getByText("dashboard content")).toBeInTheDocument();
    expect(screen.getByText("Dashboard")).toBeInTheDocument(); // sidebar nav item
  });
});
```

- [ ] **Step 2: Run the test**

Run: `cd bohikor && npx jest "src/app/\[company\]/admin/\(protected\)/__tests__/layout.test.tsx" -v`
Expected: PASS, 3/3.

- [ ] **Step 3: Commit**

```bash
cd /Users/carlson/space/bohikor2
git add "bohikor/src/app/[company]/admin/(protected)/__tests__/layout.test.tsx"
git commit -m "test(bohikor): add admin (protected) layout AdminCheck coverage"
```

---

### Task 12: `platform/(protected)/layout.tsx` tests

**Files:**
- Create: `bohikor/src/app/platform/(protected)/__tests__/layout.test.tsx`

**Interfaces:**
- Consumes: default export from `../layout`. Mocks `@/components/auth-guard` (`AuthGuard`).

- [ ] **Step 1: Write the test file**

```tsx
import { render, screen } from "@testing-library/react";
import PlatformLayout from "../layout";

jest.mock("@/components/auth-guard", () => ({
  AuthGuard: jest.fn(({ children, loginHref }: { children: React.ReactNode; loginHref: string }) => (
    <div data-testid="guard" data-loginhref={loginHref}>
      {children}
    </div>
  )),
}));

describe("PlatformLayout", () => {
  it("wraps children in AuthGuard with loginHref=/platform/login", () => {
    render(
      <PlatformLayout>
        <div>platform content</div>
      </PlatformLayout>
    );

    const guard = screen.getByTestId("guard");
    expect(guard).toHaveAttribute("data-loginhref", "/platform/login");
    expect(screen.getByText("platform content")).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run the test**

Run: `cd bohikor && npx jest "src/app/platform/\(protected\)/__tests__/layout.test.tsx" -v`
Expected: PASS, 1/1.

- [ ] **Step 3: Commit**

```bash
cd /Users/carlson/space/bohikor2
git add "bohikor/src/app/platform/(protected)/__tests__/layout.test.tsx"
git commit -m "test(bohikor): add platform protected layout coverage"
```

---

### Task 13: `platform/login/page.tsx` tests

**Files:**
- Create: `bohikor/src/app/platform/login/__tests__/page.test.tsx`

**Interfaces:**
- Consumes: default export from `../page`. Mocks `next/navigation` (`useRouter`), `@/components/providers` (`useAuth`), `@/lib/api` (`api`), `@/lib/auth` (`setTokens`, `setSubjectHint`) — same shape as the existing `[company]/admin/login` test.

- [ ] **Step 1: Write the test file**

```tsx
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import PlatformLoginPage from "../page";

const mockPush = jest.fn();
const mockReplace = jest.fn();
const mockRefreshSubject = jest.fn();

jest.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, replace: mockReplace }),
}));

jest.mock("@/components/providers", () => ({
  useAuth: jest.fn(),
}));

jest.mock("@/lib/api", () => ({
  api: { post: jest.fn() },
}));

jest.mock("@/lib/auth", () => ({
  setTokens: jest.fn(),
  setSubjectHint: jest.fn(),
}));

const { useAuth } = jest.requireMock("@/components/providers");
const { api } = jest.requireMock("@/lib/api");
const { setTokens, setSubjectHint } = jest.requireMock("@/lib/auth");

describe("PlatformLoginPage", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    useAuth.mockReturnValue({
      subjectType: null,
      refreshSubject: mockRefreshSubject,
    });
  });

  it("renders the platform login form", () => {
    render(<PlatformLoginPage />);
    expect(screen.getByText("Platform Admin")).toBeInTheDocument();
    expect(screen.getByText(/sign in as a platform administrator/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/email/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/password/i)).toBeInTheDocument();
    expect(screen.getByText("Sign In")).toBeInTheDocument();
  });

  it("redirects to /platform immediately when already a platform_admin", () => {
    useAuth.mockReturnValue({
      subjectType: "platform_admin",
      refreshSubject: mockRefreshSubject,
    });
    render(<PlatformLoginPage />);
    expect(mockReplace).toHaveBeenCalledWith("/platform");
  });

  it("submits credentials, stores tokens/hint, and redirects on success", async () => {
    api.post.mockResolvedValue({
      data: { data: { access_token: "at", refresh_token: "rt" } },
    });
    mockRefreshSubject.mockResolvedValue(undefined);

    const user = userEvent.setup();
    render(<PlatformLoginPage />);

    await user.type(screen.getByLabelText(/email/i), "platform@example.com");
    await user.type(screen.getByLabelText(/password/i), "password123");
    await user.click(screen.getByText("Sign In"));

    expect(api.post).toHaveBeenCalledWith("/api/auth/platform/login", {
      email: "platform@example.com",
      password: "password123",
    });
    expect(setTokens).toHaveBeenCalledWith("at", "rt");
    expect(setSubjectHint).toHaveBeenCalledWith("platform_admin");
    expect(mockPush).toHaveBeenCalledWith("/platform");
  });

  it("shows the response error message when present", async () => {
    api.post.mockRejectedValue({
      response: { data: { error: "Invalid platform credentials" } },
    });

    const user = userEvent.setup();
    render(<PlatformLoginPage />);

    await user.type(screen.getByLabelText(/email/i), "wrong@example.com");
    await user.type(screen.getByLabelText(/password/i), "wrong");
    await user.click(screen.getByText("Sign In"));

    expect(await screen.findByText("Invalid platform credentials")).toBeInTheDocument();
  });

  it("falls back to a generic invalid-credentials message when response has no error field", async () => {
    api.post.mockRejectedValue({ response: { data: {} } });

    const user = userEvent.setup();
    render(<PlatformLoginPage />);

    await user.type(screen.getByLabelText(/email/i), "wrong@example.com");
    await user.type(screen.getByLabelText(/password/i), "wrong");
    await user.click(screen.getByText("Sign In"));

    expect(await screen.findByText("Invalid email or password")).toBeInTheDocument();
  });

  it("shows a failed-to-sign-in message when the error has no response property", async () => {
    api.post.mockRejectedValue(new Error("network down"));

    const user = userEvent.setup();
    render(<PlatformLoginPage />);

    await user.type(screen.getByLabelText(/email/i), "x@example.com");
    await user.type(screen.getByLabelText(/password/i), "x");
    await user.click(screen.getByText("Sign In"));

    expect(await screen.findByText("Failed to sign in")).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run the test**

Run: `cd bohikor && npx jest "src/app/platform/login/__tests__/page.test.tsx" -v`
Expected: PASS, 6/6.

- [ ] **Step 3: Commit**

```bash
cd /Users/carlson/space/bohikor2
git add "bohikor/src/app/platform/login/__tests__/page.test.tsx"
git commit -m "test(bohikor): add platform login page coverage"
```

---

### Task 14: `settings/page.tsx` tests

**Files:**
- Create: `bohikor/src/app/[company]/admin/(protected)/settings/__tests__/page.test.tsx`

**Interfaces:**
- Consumes: default export from `../page`, `renderWithProviders` from `@/test-utils`. Mocks `@/hooks/use-settings` (`useSettings`, `useUpdateSetting`) and `sonner` (`toast`).

- [ ] **Step 1: Write the test file**

```tsx
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import SettingsPage from "../page";
import { renderWithProviders } from "@/test-utils";

jest.mock("@/hooks/use-settings", () => ({
  useSettings: jest.fn(),
  useUpdateSetting: jest.fn(),
}));

jest.mock("sonner", () => ({
  toast: { success: jest.fn(), error: jest.fn() },
}));

const { useSettings, useUpdateSetting } = jest.requireMock("@/hooks/use-settings");
const { toast } = jest.requireMock("sonner");

const settingsFixture = {
  advance_amount_xaf: "5000",
  kill_switch_enabled: "false",
  request_window_start_day: "1",
  request_window_end_day: "25",
  daily_request_limit: "3",
  monthly_request_limit: "10",
};

describe("SettingsPage", () => {
  let mutateFn: jest.Mock;

  beforeEach(() => {
    jest.clearAllMocks();
    mutateFn = jest.fn();
    useUpdateSetting.mockReturnValue({ mutate: mutateFn, isPending: false });
  });

  it("shows only the loading text while settings are loading", () => {
    useSettings.mockReturnValue({
      data: undefined,
      isLoading: true,
      refetch: jest.fn(),
      isRefetching: false,
    });
    renderWithProviders(<SettingsPage />);

    expect(screen.getByText("Loading settings...")).toBeInTheDocument();
    expect(screen.queryByText("Settings")).not.toBeInTheDocument();
  });

  it("populates all four cards from fetched settings", () => {
    useSettings.mockReturnValue({
      data: settingsFixture,
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });
    renderWithProviders(<SettingsPage />);

    expect(screen.getByText("Advance Amount")).toBeInTheDocument();
    expect(screen.getByDisplayValue("5000")).toBeInTheDocument();
    expect(screen.getByText("Advance requests allowed")).toBeInTheDocument();
    expect(screen.getByDisplayValue("1")).toBeInTheDocument();
    expect(screen.getByDisplayValue("25")).toBeInTheDocument();
    expect(screen.getByDisplayValue("3")).toBeInTheDocument();
    expect(screen.getByDisplayValue("10")).toBeInTheDocument();
  });

  it("shows the blocked label when kill_switch_enabled is true", () => {
    useSettings.mockReturnValue({
      data: { ...settingsFixture, kill_switch_enabled: "true" },
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });
    renderWithProviders(<SettingsPage />);

    expect(screen.getByText("Advance requests blocked")).toBeInTheDocument();
  });

  it("refetches when the refresh button is clicked", async () => {
    const refetchFn = jest.fn();
    useSettings.mockReturnValue({
      data: settingsFixture,
      isLoading: false,
      refetch: refetchFn,
      isRefetching: false,
    });
    const user = userEvent.setup();
    renderWithProviders(<SettingsPage />);

    await user.click(screen.getByText("Refresh"));

    expect(refetchFn).toHaveBeenCalled();
  });

  it("edits and saves the Advance Amount card, calling mutate with the right key/value", async () => {
    useSettings.mockReturnValue({
      data: settingsFixture,
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });
    const user = userEvent.setup();
    renderWithProviders(<SettingsPage />);

    const editButtons = screen.getAllByText("Edit");
    await user.click(editButtons[0]);

    const amountInput = screen.getByDisplayValue("5000");
    await user.clear(amountInput);
    await user.type(amountInput, "7000");

    await user.click(screen.getByText("Save"));

    expect(mutateFn).toHaveBeenCalledWith(
      { key: "advance_amount_xaf", value: "7000" },
      expect.objectContaining({ onSuccess: expect.any(Function), onError: expect.any(Function) })
    );
  });

  it("shows a success toast and exits edit mode on save success", async () => {
    useSettings.mockReturnValue({
      data: settingsFixture,
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });
    mutateFn.mockImplementation((_vars, callbacks) => callbacks.onSuccess());
    const user = userEvent.setup();
    renderWithProviders(<SettingsPage />);

    await user.click(screen.getAllByText("Edit")[0]);
    await user.click(screen.getByText("Save"));

    await waitFor(() => expect(toast.success).toHaveBeenCalled());
    expect(screen.getAllByText("Edit").length).toBeGreaterThan(0);
  });

  it("shows an error toast and stays in edit mode on save failure", async () => {
    useSettings.mockReturnValue({
      data: settingsFixture,
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });
    mutateFn.mockImplementation((_vars, callbacks) => callbacks.onError());
    const user = userEvent.setup();
    renderWithProviders(<SettingsPage />);

    await user.click(screen.getAllByText("Edit")[0]);
    await user.click(screen.getByText("Save"));

    await waitFor(() => expect(toast.error).toHaveBeenCalled());
    expect(screen.getByText("Save")).toBeInTheDocument();
  });

  it("saves the Request Window card by calling mutate twice, once per field", async () => {
    useSettings.mockReturnValue({
      data: settingsFixture,
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });
    const user = userEvent.setup();
    renderWithProviders(<SettingsPage />);

    // Third "Edit" button corresponds to the Request Window card
    // (Advance Amount, Kill Switch, Request Window, Rate Limits — in DOM order).
    const editButtons = screen.getAllByText("Edit");
    await user.click(editButtons[2]);

    const saveButtons = screen.getAllByText("Save");
    await user.click(saveButtons[0]);

    expect(mutateFn).toHaveBeenCalledWith(
      expect.objectContaining({ key: "request_window_start_day" }),
      expect.anything()
    );
    expect(mutateFn).toHaveBeenCalledWith(
      expect.objectContaining({ key: "request_window_end_day" }),
      expect.anything()
    );
  });

  it("saves the Rate Limits card by calling mutate twice, once per field", async () => {
    useSettings.mockReturnValue({
      data: settingsFixture,
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });
    const user = userEvent.setup();
    renderWithProviders(<SettingsPage />);

    const editButtons = screen.getAllByText("Edit");
    await user.click(editButtons[3]);

    const saveButtons = screen.getAllByText("Save");
    await user.click(saveButtons[0]);

    expect(mutateFn).toHaveBeenCalledWith(
      expect.objectContaining({ key: "daily_request_limit" }),
      expect.anything()
    );
    expect(mutateFn).toHaveBeenCalledWith(
      expect.objectContaining({ key: "monthly_request_limit" }),
      expect.anything()
    );
  });

  it("does not re-populate fields from a later settings payload after initial mount", () => {
    useSettings.mockReturnValue({
      data: settingsFixture,
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });
    const { rerender } = renderWithProviders(<SettingsPage />);
    expect(screen.getByDisplayValue("5000")).toBeInTheDocument();

    useSettings.mockReturnValue({
      data: { ...settingsFixture, advance_amount_xaf: "9999" },
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });
    rerender(<SettingsPage />);

    expect(screen.getByDisplayValue("5000")).toBeInTheDocument();
    expect(screen.queryByDisplayValue("9999")).not.toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run the test**

Run: `cd bohikor && npx jest "src/app/\[company\]/admin/\(protected\)/settings/__tests__/page.test.tsx" -v`
Expected: PASS, 11/11. If the "Edit"/"Save" button index assumptions (`editButtons[2]`, `editButtons[3]`) don't match actual DOM order when run, adjust the indices to match — the card order in the source is Advance Amount, Kill Switch, Request Window, Rate Limits, so indices 0-3 in that order; do not change the assertions' intent, only the index if needed.

- [ ] **Step 3: Commit**

```bash
cd /Users/carlson/space/bohikor2
git add "bohikor/src/app/[company]/admin/(protected)/settings/__tests__/page.test.tsx"
git commit -m "test(bohikor): add settings page coverage (edit/save flow, all 4 cards)"
```

---

### Task 15: `users/page.tsx` tests

**Files:**
- Create: `bohikor/src/app/[company]/admin/(protected)/users/__tests__/page.test.tsx`

**Interfaces:**
- Consumes: default export from `../page`, `renderWithProviders` from `@/test-utils`. Mocks `@/hooks/use-users` (`useUsers`, `useSuspendUser`, `useActivateUser`, `useUnlockUser`) and `sonner` (`toast`).

- [ ] **Step 1: Write the test file**

```tsx
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import UsersPage from "../page";
import { renderWithProviders } from "@/test-utils";

jest.mock("@/hooks/use-users", () => ({
  useUsers: jest.fn(),
  useSuspendUser: jest.fn(),
  useActivateUser: jest.fn(),
  useUnlockUser: jest.fn(),
}));

jest.mock("sonner", () => ({
  toast: { success: jest.fn(), error: jest.fn() },
}));

const { useUsers, useSuspendUser, useActivateUser, useUnlockUser } =
  jest.requireMock("@/hooks/use-users");
const { toast } = jest.requireMock("sonner");

function baseUser(overrides: Record<string, unknown> = {}) {
  return {
    id: "u1",
    email: "active@example.com",
    full_name: "Active User",
    phone_number: "+237000000",
    phone_verified: true,
    status: "active",
    pin_hash: "hash",
    locked_until: null,
    is_terms_accepted: true,
    created_at: "2026-05-01T00:00:00Z",
    ...overrides,
  };
}

describe("UsersPage", () => {
  let suspendMutate: jest.Mock;
  let activateMutate: jest.Mock;
  let unlockMutate: jest.Mock;

  beforeEach(() => {
    jest.clearAllMocks();
    suspendMutate = jest.fn();
    activateMutate = jest.fn();
    unlockMutate = jest.fn();
    useSuspendUser.mockReturnValue({ mutate: suspendMutate });
    useActivateUser.mockReturnValue({ mutate: activateMutate });
    useUnlockUser.mockReturnValue({ mutate: unlockMutate });
  });

  it("shows only the loading text while users are loading", () => {
    useUsers.mockReturnValue({ data: undefined, isLoading: true, refetch: jest.fn(), isRefetching: false });
    renderWithProviders(<UsersPage />);

    expect(screen.getByText("Loading users...")).toBeInTheDocument();
  });

  it("shows the empty-state row when there are no users", () => {
    useUsers.mockReturnValue({ data: [], isLoading: false, refetch: jest.fn(), isRefetching: false });
    renderWithProviders(<UsersPage />);

    expect(screen.getByText("No users found")).toBeInTheDocument();
  });

  it("renders every badge/fallback branch for an active, fully-populated user", () => {
    useUsers.mockReturnValue({
      data: [baseUser()],
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });
    renderWithProviders(<UsersPage />);

    expect(screen.getByText("active@example.com")).toBeInTheDocument();
    expect(screen.getByText("Active User")).toBeInTheDocument();
    expect(screen.getByText("Yes")).toBeInTheDocument(); // phone_verified
    expect(screen.getByText("active")).toBeInTheDocument();
    expect(screen.getByText("Set")).toBeInTheDocument(); // pin_hash
    expect(screen.getByText("Accepted")).toBeInTheDocument(); // is_terms_accepted
  });

  it("renders '—' fallbacks for missing optional fields", () => {
    useUsers.mockReturnValue({
      data: [
        baseUser({
          full_name: null,
          phone_number: null,
          phone_verified: false,
          pin_hash: null,
          locked_until: null,
          is_terms_accepted: false,
        }),
      ],
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });
    renderWithProviders(<UsersPage />);

    expect(screen.getByText("No")).toBeInTheDocument(); // phone_verified false
    expect(screen.getByText("None")).toBeInTheDocument(); // pin_hash null
    expect(screen.getByText("Pending")).toBeInTheDocument(); // terms not accepted
    const dashes = screen.getAllByText("—");
    expect(dashes.length).toBeGreaterThanOrEqual(3); // name, phone, locked_until
  });

  it("refetches when the refresh button is clicked", async () => {
    const refetchFn = jest.fn();
    useUsers.mockReturnValue({ data: [], isLoading: false, refetch: refetchFn, isRefetching: false });
    const user = userEvent.setup();
    renderWithProviders(<UsersPage />);

    await user.click(screen.getByText("Refresh"));

    expect(refetchFn).toHaveBeenCalled();
  });

  it("shows only Suspend for an active user (no Unlock/Activate)", async () => {
    useUsers.mockReturnValue({
      data: [baseUser({ status: "active" })],
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });
    const user = userEvent.setup();
    renderWithProviders(<UsersPage />);

    await user.click(screen.getByRole("button", { name: "" })); // MoreHorizontal trigger, unlabeled

    expect(screen.getByText("Suspend")).toBeInTheDocument();
    expect(screen.queryByText("Activate")).not.toBeInTheDocument();
    expect(screen.queryByText("Unlock")).not.toBeInTheDocument();

    await user.click(screen.getByText("Suspend"));
    expect(suspendMutate).toHaveBeenCalledWith(
      "u1",
      expect.objectContaining({ onSuccess: expect.any(Function), onError: expect.any(Function) })
    );
  });

  it("shows only Activate for a suspended user (no Unlock/Suspend)", async () => {
    useUsers.mockReturnValue({
      data: [baseUser({ id: "u2", status: "suspended" })],
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });
    const user = userEvent.setup();
    renderWithProviders(<UsersPage />);

    await user.click(screen.getByRole("button", { name: "" }));

    expect(screen.getByText("Activate")).toBeInTheDocument();
    expect(screen.queryByText("Suspend")).not.toBeInTheDocument();
    expect(screen.queryByText("Unlock")).not.toBeInTheDocument();

    await user.click(screen.getByText("Activate"));
    expect(activateMutate).toHaveBeenCalledWith("u2", expect.anything());
  });

  it("shows only Unlock for a locked user (no Suspend/Activate)", async () => {
    useUsers.mockReturnValue({
      data: [baseUser({ id: "u3", status: "locked" })],
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });
    const user = userEvent.setup();
    renderWithProviders(<UsersPage />);

    await user.click(screen.getByRole("button", { name: "" }));

    expect(screen.getByText("Unlock")).toBeInTheDocument();
    expect(screen.queryByText("Suspend")).not.toBeInTheDocument();
    expect(screen.queryByText("Activate")).not.toBeInTheDocument();

    await user.click(screen.getByText("Unlock"));
    expect(unlockMutate).toHaveBeenCalledWith("u3", expect.anything());
  });

  it("shows a success toast when a suspend action's onSuccess fires", async () => {
    suspendMutate.mockImplementation((_id, callbacks) => callbacks.onSuccess());
    useUsers.mockReturnValue({
      data: [baseUser({ status: "active" })],
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });
    const user = userEvent.setup();
    renderWithProviders(<UsersPage />);

    await user.click(screen.getByRole("button", { name: "" }));
    await user.click(screen.getByText("Suspend"));

    await waitFor(() => expect(toast.success).toHaveBeenCalled());
  });

  it("shows an error toast when an activate action's onError fires", async () => {
    activateMutate.mockImplementation((_id, callbacks) => callbacks.onError());
    useUsers.mockReturnValue({
      data: [baseUser({ id: "u2", status: "suspended" })],
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });
    const user = userEvent.setup();
    renderWithProviders(<UsersPage />);

    await user.click(screen.getByRole("button", { name: "" }));
    await user.click(screen.getByText("Activate"));

    await waitFor(() => expect(toast.error).toHaveBeenCalled());
  });
});
```

Note: the `MoreHorizontal` dropdown trigger `Button` has no accessible name (icon-only) — `screen.getByRole("button", { name: "" })` targets it given each test renders exactly one data row and one such trigger; if this proves ambiguous against the "Refresh" button (which does have visible text and thus a non-empty accessible name), it will not match and is fine as written, but if the implementer finds it flaky, switch to `screen.getAllByRole("button")` and select the last one, or add a `data-testid` to the trigger — prefer the `getByRole("button", {name: ""})` approach first since it requires no source changes.

- [ ] **Step 2: Run the test**

Run: `cd bohikor && npx jest "src/app/\[company\]/admin/\(protected\)/users/__tests__/page.test.tsx" -v`
Expected: PASS, 10/10.

- [ ] **Step 3: Commit**

```bash
cd /Users/carlson/space/bohikor2
git add "bohikor/src/app/[company]/admin/(protected)/users/__tests__/page.test.tsx"
git commit -m "test(bohikor): add users page coverage (all status/badge branches, dropdown actions)"
```

---

### Task 16: Trivial stub pages + Toaster mount test

**Files:**
- Create: `bohikor/src/app/__tests__/stub-pages.test.tsx`
- Create: `bohikor/src/components/ui/__tests__/sonner.test.tsx`

**Interfaces:**
- Consumes: all 13 trivial stub page default exports, plus `Toaster` from `../sonner`. No mocks needed — these components have no hooks/data dependencies.

- [ ] **Step 1: Write the batched stub-pages test**

```tsx
import { render, screen } from "@testing-library/react";
import LandingPage from "../page";
import EmployeeHomePage from "../[company]/page";
import EmployeeLoginPage from "../[company]/login/page";
import SignupPage from "../[company]/signup/page";
import VerifyPage from "../[company]/verify/page";
import CreatePinPage from "../[company]/create-pin/page";
import ForgotPinPage from "../[company]/forgot-pin/page";
import ResetPinPage from "../[company]/reset-pin/page";
import HistoryPage from "../[company]/history/page";
import AccountPage from "../[company]/account/page";
import EventsPage from "../[company]/admin/(protected)/events/page";
import BalancePage from "../[company]/admin/(protected)/balance/page";
import PlatformConsolePage from "../platform/(protected)/page";

const stubs: [string, () => React.ReactElement, string][] = [
  ["LandingPage", LandingPage, "Sign in — coming soon."],
  ["EmployeeHomePage", EmployeeHomePage, "Employee home — coming soon."],
  ["EmployeeLoginPage", EmployeeLoginPage, "Employee sign in — coming soon."],
  ["SignupPage", SignupPage, "Sign up — coming soon."],
  ["VerifyPage", VerifyPage, "Verify — coming soon."],
  ["CreatePinPage", CreatePinPage, "Create PIN — coming soon."],
  ["ForgotPinPage", ForgotPinPage, "Forgot PIN — coming soon."],
  ["ResetPinPage", ResetPinPage, "Reset PIN — coming soon."],
  ["HistoryPage", HistoryPage, "History — coming soon."],
  ["AccountPage", AccountPage, "Account — coming soon."],
  ["EventsPage", EventsPage, "Events — coming soon."],
  ["BalancePage", BalancePage, "Balance & ledger — coming soon."],
  ["PlatformConsolePage", PlatformConsolePage, "Platform console — coming soon."],
];

describe.each(stubs)("%s", (_name, Component, expectedText) => {
  it(`renders "${expectedText}"`, () => {
    render(<Component />);
    expect(screen.getByText(expectedText)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Write the Toaster mount test**

```tsx
import { render } from "@testing-library/react";
import { Toaster } from "../sonner";

describe("Toaster", () => {
  it("mounts without throwing", () => {
    const { container } = render(<Toaster />);
    expect(container).toBeInTheDocument();
  });
});
```

- [ ] **Step 3: Run both tests**

Run: `cd bohikor && npx jest src/app/__tests__/stub-pages.test.tsx src/components/ui/__tests__/sonner.test.tsx -v`
Expected: PASS, 13/13 stub cases + 1/1 Toaster.

- [ ] **Step 4: Commit**

```bash
cd /Users/carlson/space/bohikor2
git add bohikor/src/app/__tests__/stub-pages.test.tsx bohikor/src/components/ui/__tests__/sonner.test.tsx
git commit -m "test(bohikor): add batched stub-page coverage and Toaster mount test"
```

---

### Task 17: Coverage gate verification (mandatory)

**Files:** None modified — this task only runs checks and, if needed, adds narrowly-targeted supplementary tests to whichever specific file(s) are short.

- [ ] **Step 1: Run the full coverage suite**

Run: `cd bohikor && npm run test:coverage`
Expected: exits 0. Read the full per-file table in the output.

- [ ] **Step 2: If the threshold passes**

Confirm all four global metrics (statements/branches/functions/lines) show ≥85% in the summary line, and note in the task report which files (if any) are still below 85% individually even though the aggregate passes — informational only, not a blocker.

- [ ] **Step 3: If the threshold fails**

Identify the specific file(s) and branch(es) dragging the aggregate down from the printed table. Add 1-3 targeted test cases to the relevant existing test file(s) from Tasks 3-16 (do not create new source files, do not lower the threshold in `jest.config.js`) to close the gap. Re-run `npm run test:coverage` and repeat until it passes. The one pre-approved exception: the three `typeof window === "undefined"` branches in `bohikor/src/lib/auth.ts` (Task 3) may remain uncovered and documented as accepted — do not attempt to fake `window` away to chase them.

- [ ] **Step 4: Run the full verification suite**

Run: `cd bohikor && npm run typecheck && npm run lint && npm run test:coverage && npm run build`
Expected: all four commands pass.

- [ ] **Step 5: Commit any supplementary test additions from Step 3**

```bash
cd /Users/carlson/space/bohikor2
git add bohikor/src
git commit -m "test(bohikor): close remaining coverage gaps to meet 85% threshold"
```

If no supplementary tests were needed (threshold passed on the first run), skip this commit — there is nothing to commit.
