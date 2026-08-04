import { screen, waitFor } from "@testing-library/react";
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
  ...jest.requireActual("sonner"),
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

  it("shows an error toast when a suspend action's onError fires", async () => {
    suspendMutate.mockImplementation((_id, callbacks) => callbacks.onError());
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

    await waitFor(() => expect(toast.error).toHaveBeenCalled());
  });

  it("shows a success toast when an activate action's onSuccess fires", async () => {
    activateMutate.mockImplementation((_id, callbacks) => callbacks.onSuccess());
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

    await waitFor(() => expect(toast.success).toHaveBeenCalled());
  });

  it("shows a success toast when an unlock action's onSuccess fires", async () => {
    unlockMutate.mockImplementation((_id, callbacks) => callbacks.onSuccess());
    useUsers.mockReturnValue({
      data: [baseUser({ id: "u3", status: "locked" })],
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });
    const user = userEvent.setup();
    renderWithProviders(<UsersPage />);

    await user.click(screen.getByRole("button", { name: "" }));
    await user.click(screen.getByText("Unlock"));

    await waitFor(() => expect(toast.success).toHaveBeenCalled());
  });

  it("shows an error toast when an unlock action's onError fires", async () => {
    unlockMutate.mockImplementation((_id, callbacks) => callbacks.onError());
    useUsers.mockReturnValue({
      data: [baseUser({ id: "u3", status: "locked" })],
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });
    const user = userEvent.setup();
    renderWithProviders(<UsersPage />);

    await user.click(screen.getByRole("button", { name: "" }));
    await user.click(screen.getByText("Unlock"));

    await waitFor(() => expect(toast.error).toHaveBeenCalled());
  });

  it("shows a spinning refresh icon while refetching", () => {
    useUsers.mockReturnValue({ data: [], isLoading: false, refetch: jest.fn(), isRefetching: true });
    const { container } = renderWithProviders(<UsersPage />);

    expect(container.querySelector(".animate-spin")).toBeInTheDocument();
  });

  it("renders the formatted locked_until timestamp when present instead of a dash", () => {
    useUsers.mockReturnValue({
      data: [baseUser({ status: "locked", locked_until: "2026-05-01T10:00:00Z" })],
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });
    renderWithProviders(<UsersPage />);

    expect(screen.queryByText("—")).not.toBeInTheDocument();
  });
});
