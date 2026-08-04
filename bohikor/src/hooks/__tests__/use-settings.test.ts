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
