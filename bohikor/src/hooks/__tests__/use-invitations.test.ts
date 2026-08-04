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
