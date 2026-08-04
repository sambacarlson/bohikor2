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
