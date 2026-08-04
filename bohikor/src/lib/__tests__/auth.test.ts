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
