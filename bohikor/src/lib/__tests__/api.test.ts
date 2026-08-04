import axios from "axios";
import { getApiErrorMessage } from "../api";

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

  // jsdom's `window.location` is an own, non-configurable accessor property
  // on the Window instance (it implements the WHATWG "unforgeable" location
  // per spec), so neither `delete window.location` nor
  // `Object.defineProperty(window, "location", ...)` work here — delete is a
  // silent no-op (returns false) and defineProperty always throws "Cannot
  // redefine property: location", even on the very first attempt in a fresh
  // test file. Real navigation is also not implemented for non-hash changes:
  // `window.location.href = "/some/path"` logs a "Not implemented:
  // navigation" console error and leaves `.href` unchanged, so we can't
  // observe app-triggered redirects by reading the real Location object
  // either.
  //
  // Instead we shadow the identifier the running code resolves `window`
  // through: jsdom's Window implementation keeps an internal `_globalProxy`
  // that both the test file and any module it dynamically imports (api.ts)
  // read `window` through in this same JS realm. Swapping it for our own
  // Proxy — one that special-cases the `location` property to return a
  // plain, freely-mutable stub object — lets `window.location.href = "/"`
  // be captured exactly as assigned, without touching real navigation at
  // all. This relies on an internal jsdom property (undocumented, but
  // stable across the jsdom versions this repo has used) rather than a
  // public API; the `expect` below fails loudly if jsdom ever removes it.
  const globalProxyHolder = window as unknown as { _globalProxy?: object };
  expect(globalProxyHolder._globalProxy).toBeTruthy();
  const locationStub: { href: string; pathname: string } = { href: "", pathname: "/" };
  const windowProxy = new Proxy(window, {
    get(target, prop, receiver) {
      if (prop === "location") return locationStub;
      return Reflect.get(target, prop, receiver);
    },
    set(target, prop, value) {
      if (prop === "location") {
        locationStub.href = value as string;
        return true;
      }
      return Reflect.set(target, prop, value);
    },
  });
  globalProxyHolder._globalProxy = windowProxy;

  // `jest.resetModules()` clears Jest's whole module registry, not just
  // `../api` — so the `axios` re-imported *inside* a freshly re-imported
  // `../api` is a different module instance than the `axios` bound at the
  // top of this file (that binding was resolved once, before any
  // `resetModules()` call, and ES module bindings don't retroactively
  // repoint). Spying on the stale top-of-file `axios` therefore never
  // intercepts the `axios.post` call the interceptor makes internally —
  // it silently falls through to a real network request instead. Import
  // "axios" fresh alongside "../api" in the same post-reset registry
  // generation so both resolve to the same module instance, and spy on
  // that one.
  async function loadApi() {
    const { default: freshAxios } = await import("axios");
    const { api } = await import("../api");
    jest.spyOn(freshAxios, "post").mockReset();
    return { api, freshAxios };
  }

  beforeEach(() => {
    jest.resetModules();
    authMock = jest.requireMock("../auth");
    authMock.getAccessToken.mockReset();
    authMock.getRefreshToken.mockReset();
    authMock.setTokens.mockReset();
    authMock.clearTokens.mockReset();
    // In the real module, `setTokens` writes to localStorage and
    // `getAccessToken` reads it back — they share state. That matters here
    // because a successful refresh retries the original request via
    // `api(originalRequest)`, which re-runs the *request* interceptor and
    // calls `getAccessToken()` again. A static mock would clobber the new
    // token's Authorization header back to the pre-refresh one, which is
    // not what the real localStorage-backed implementation does. Link the
    // two mocks so `getAccessToken` reflects whatever `setTokens` last set.
    authMock.setTokens.mockImplementation((accessToken: string) => {
      authMock.getAccessToken.mockReturnValue(accessToken);
    });
    locationStub.href = "";
    locationStub.pathname = "/";
  });

  it("attaches Authorization header when a token exists", async () => {
    authMock.getAccessToken.mockReturnValue("tok-123");
    const { api } = await loadApi();
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
    const { api } = await loadApi();
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
    const { api, freshAxios } = await loadApi();
    const adapter = jest.fn().mockResolvedValue({
      status: 200,
      data: { ok: true },
      headers: {},
      config: {},
    });
    api.defaults.adapter = adapter;

    const res = await api.get("/api/ping");

    expect(res.data).toEqual({ ok: true });
    expect(freshAxios.post).not.toHaveBeenCalled();
  });

  it("on 401, refreshes the token and retries the original request", async () => {
    authMock.getAccessToken.mockReturnValue("old-tok");
    authMock.getRefreshToken.mockReturnValue("refresh-tok");
    const { api, freshAxios } = await loadApi();

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

    (freshAxios.post as jest.Mock).mockResolvedValue({
      data: { data: { access_token: "new-tok", refresh_token: "new-refresh" } },
    });

    const res = await api.get("/api/protected");

    expect(authMock.setTokens).toHaveBeenCalledWith("new-tok", "new-refresh");
    expect(res.data).toEqual({ ok: true });
    expect(adapter.mock.calls[1][0].headers.Authorization).toBe("Bearer new-tok");
  });

  it("on 401 with no refresh token, clears tokens and redirects to the company admin login", async () => {
    authMock.getAccessToken.mockReturnValue("old-tok");
    authMock.getRefreshToken.mockReturnValue(null);
    locationStub.pathname = "/acme/admin/requests";
    const { api } = await loadApi();

    const adapter = jest.fn().mockImplementationOnce(() => {
      const err: unknown = new Error("Unauthorized");
      (err as { response: unknown }).response = { status: 401, data: {} };
      (err as { config: unknown }).config = { headers: {}, _retry: false };
      return Promise.reject(err);
    });
    api.defaults.adapter = adapter;

    await expect(api.get("/api/protected")).rejects.toBeTruthy();

    expect(authMock.clearTokens).toHaveBeenCalled();
    expect(window.location.href).toBe("/acme/admin/login");
  });

  it("on 401 when the refresh call itself fails, clears tokens and redirects to /platform/login from a platform route", async () => {
    authMock.getAccessToken.mockReturnValue("old-tok");
    authMock.getRefreshToken.mockReturnValue("refresh-tok");
    locationStub.pathname = "/platform";
    const { api, freshAxios } = await loadApi();

    const adapter = jest.fn().mockImplementationOnce(() => {
      const err: unknown = new Error("Unauthorized");
      (err as { response: unknown }).response = { status: 401, data: {} };
      (err as { config: unknown }).config = { headers: {}, _retry: false };
      return Promise.reject(err);
    });
    api.defaults.adapter = adapter;

    (freshAxios.post as jest.Mock).mockRejectedValue(new Error("refresh failed"));

    await expect(api.get("/api/protected")).rejects.toBeTruthy();

    expect(authMock.clearTokens).toHaveBeenCalled();
    expect(window.location.href).toBe("/platform/login");
  });

  it("on 401 with no refresh token and no company segment in the path, falls back to /", async () => {
    authMock.getAccessToken.mockReturnValue("old-tok");
    authMock.getRefreshToken.mockReturnValue(null);
    locationStub.pathname = "/";
    const { api } = await loadApi();

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

  it("passes a non-401 error straight through without attempting a refresh", async () => {
    authMock.getAccessToken.mockReturnValue("old-tok");
    const { api, freshAxios } = await loadApi();

    const adapter = jest.fn().mockImplementationOnce(() => {
      const err: unknown = new Error("Server error");
      (err as { response: unknown }).response = { status: 500, data: {} };
      (err as { config: unknown }).config = { headers: {}, _retry: false };
      return Promise.reject(err);
    });
    api.defaults.adapter = adapter;

    await expect(api.get("/api/protected")).rejects.toThrow("Server error");

    expect(freshAxios.post).not.toHaveBeenCalled();
    expect(authMock.clearTokens).not.toHaveBeenCalled();
  });

  it("queues concurrent 401s behind a single in-flight refresh", async () => {
    authMock.getAccessToken.mockReturnValue("old-tok");
    authMock.getRefreshToken.mockReturnValue("refresh-tok");
    const { api, freshAxios } = await loadApi();

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
    (freshAxios.post as jest.Mock).mockReturnValue(
      new Promise((resolve) => {
        resolveRefresh = resolve;
      })
    );

    const p1 = api.get("/api/a");
    const p2 = api.get("/api/b");

    // Let both 401s fire before the refresh resolves.
    await new Promise((r) => setTimeout(r, 0));
    expect(freshAxios.post).toHaveBeenCalledTimes(1);

    resolveRefresh({
      data: { data: { access_token: "new-tok", refresh_token: "new-refresh" } },
    });

    const [res1, res2] = await Promise.all([p1, p2]);
    expect(res1.data).toEqual({ ok: true });
    expect(res2.data).toEqual({ ok: true });
  });
});

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

  it("returns a generic fallback for an axios error with no message and a non-401 status", () => {
    const err = {
      isAxiosError: true,
      response: { status: 500, data: {} },
    };
    jest.spyOn(axios, "isAxiosError").mockReturnValue(true);
    expect(getApiErrorMessage(err)).toBe("Something went wrong. Please try again.");
  });
});
