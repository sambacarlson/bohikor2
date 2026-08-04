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
