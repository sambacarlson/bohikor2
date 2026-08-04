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
