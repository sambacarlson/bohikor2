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
