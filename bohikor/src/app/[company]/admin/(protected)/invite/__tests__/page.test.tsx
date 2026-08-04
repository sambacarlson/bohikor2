import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import InvitePage from "../page";
import { renderWithProviders } from "@/test-utils";

jest.mock("@/hooks/use-invitations", () => ({
  useInvitations: jest.fn(),
  useSendInvite: jest.fn(),
}));

const { useInvitations, useSendInvite } = jest.requireMock(
  "@/hooks/use-invitations"
);

describe("InvitePage", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("renders form and empty state", () => {
    useInvitations.mockReturnValue({
      data: [],
      isLoading: false,
      refetch: jest.fn(),
    });
    useSendInvite.mockReturnValue({
      mutate: jest.fn(),
      isPending: false,
    });

    renderWithProviders(<InvitePage />);

    expect(
      screen.getByRole("heading", { name: /invite employees/i })
    ).toBeInTheDocument();
    expect(screen.getByLabelText(/email address/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /invite/i })).toBeInTheDocument();
    expect(screen.getByText(/no invitations sent yet/i)).toBeInTheDocument();
  });

  it("shows loading state while fetching invitations", () => {
    useInvitations.mockReturnValue({
      data: null,
      isLoading: true,
      refetch: jest.fn(),
    });
    useSendInvite.mockReturnValue({
      mutate: jest.fn(),
      isPending: false,
    });

    renderWithProviders(<InvitePage />);

    expect(screen.getByText(/loading invitations/i)).toBeInTheDocument();
  });

  it("displays invitations when available", () => {
    useInvitations.mockReturnValue({
      data: [
        {
          id: "1",
          email: "test@example.com",
          status: "sent",
          sent_at: "2026-05-20T00:00:00Z",
          accepted_at: null,
        },
        {
          id: "2",
          email: "accepted@example.com",
          status: "accepted",
          sent_at: "2026-05-19T00:00:00Z",
          accepted_at: "2026-05-19T12:00:00Z",
        },
      ],
      isLoading: false,
      refetch: jest.fn(),
    });
    useSendInvite.mockReturnValue({
      mutate: jest.fn(),
      isPending: false,
    });

    renderWithProviders(<InvitePage />);

    expect(screen.getByText("test@example.com")).toBeInTheDocument();
    expect(screen.getByText("accepted@example.com")).toBeInTheDocument();
    expect(screen.getByText("sent")).toBeInTheDocument();
    expect(screen.getByText("accepted")).toBeInTheDocument();
  });

  it("shows error when submitting empty email", async () => {
    const user = userEvent.setup();
    useInvitations.mockReturnValue({
      data: [],
      isLoading: false,
      refetch: jest.fn(),
    });
    useSendInvite.mockReturnValue({
      mutate: jest.fn(),
      isPending: false,
    });

    renderWithProviders(<InvitePage />);

    const submitButton = screen.getByRole("button", { name: /invite/i });
    await user.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText(/email is required/i)).toBeInTheDocument();
    });
  });

  it("calls sendInvite mutation with valid email", async () => {
    const user = userEvent.setup();
    const mutateFn = jest.fn();
    useInvitations.mockReturnValue({
      data: [],
      isLoading: false,
      refetch: jest.fn(),
    });
    useSendInvite.mockReturnValue({
      mutate: mutateFn,
      isPending: false,
    });

    renderWithProviders(<InvitePage />);

    const emailInput = screen.getByLabelText(/email address/i);
    await user.type(emailInput, "newadmin@example.com");

    const submitButton = screen.getByRole("button", { name: /invite/i });
    await user.click(submitButton);

    expect(mutateFn).toHaveBeenCalledWith("newadmin@example.com", {
      onSuccess: expect.any(Function),
      onError: expect.any(Function),
    });
  });

  it("shows duplicate error on 409 response", async () => {
    const user = userEvent.setup();
    const mutateFn = jest.fn().mockImplementation((_, callbacks) => {
      const err = new Error("Conflict");
      (err as unknown as { response: { status: number } }).response = {
        status: 409,
      };
      callbacks.onError(err);
    });
    useInvitations.mockReturnValue({
      data: [],
      isLoading: false,
      refetch: jest.fn(),
    });
    useSendInvite.mockReturnValue({
      mutate: mutateFn,
      isPending: false,
    });

    renderWithProviders(<InvitePage />);

    const emailInput = screen.getByLabelText(/email address/i);
    await user.type(emailInput, "existing@example.com");

    const submitButton = screen.getByRole("button", { name: /invite/i });
    await user.click(submitButton);

    await waitFor(() => {
      expect(
        screen.getByText(/an active invitation already exists/i)
      ).toBeInTheDocument();
    });
  });

  it("shows a success toast and clears the email field on send success", async () => {
    const user = userEvent.setup();
    const mutateFn = jest.fn().mockImplementation((_email, callbacks) => {
      callbacks.onSuccess();
    });
    useInvitations.mockReturnValue({
      data: [],
      isLoading: false,
      refetch: jest.fn(),
    });
    useSendInvite.mockReturnValue({
      mutate: mutateFn,
      isPending: false,
    });

    renderWithProviders(<InvitePage />);

    const emailInput = screen.getByLabelText(/email address/i) as HTMLInputElement;
    await user.type(emailInput, "newadmin@example.com");
    await user.click(screen.getByRole("button", { name: /invite/i }));

    await waitFor(() => {
      expect(emailInput.value).toBe("");
    });
  });

  it("shows a generic error for a non-409 failure", async () => {
    const user = userEvent.setup();
    const mutateFn = jest.fn().mockImplementation((_email, callbacks) => {
      callbacks.onError(new Error("Network error"));
    });
    useInvitations.mockReturnValue({
      data: [],
      isLoading: false,
      refetch: jest.fn(),
    });
    useSendInvite.mockReturnValue({
      mutate: mutateFn,
      isPending: false,
    });

    renderWithProviders(<InvitePage />);

    await user.type(screen.getByLabelText(/email address/i), "someone@example.com");
    await user.click(screen.getByRole("button", { name: /invite/i }));

    await waitFor(() => {
      expect(screen.getAllByText(/failed to send invitation/i).length).toBeGreaterThan(0);
    });
  });

  it("shows Sending... while the send mutation is pending", () => {
    useInvitations.mockReturnValue({
      data: [],
      isLoading: false,
      refetch: jest.fn(),
    });
    useSendInvite.mockReturnValue({
      mutate: jest.fn(),
      isPending: true,
    });

    renderWithProviders(<InvitePage />);

    expect(screen.getByText(/sending\.\.\./i)).toBeInTheDocument();
  });

  it("renders pending/other status badges and a dash for a missing sent_at", () => {
    useInvitations.mockReturnValue({
      data: [
        {
          id: "3",
          email: "pending@example.com",
          status: "pending",
          sent_at: null,
          accepted_at: null,
        },
        {
          id: "4",
          email: "other@example.com",
          status: "revoked",
          sent_at: "2026-05-18T00:00:00Z",
          accepted_at: null,
        },
      ],
      isLoading: false,
      refetch: jest.fn(),
    });
    useSendInvite.mockReturnValue({
      mutate: jest.fn(),
      isPending: false,
    });

    renderWithProviders(<InvitePage />);

    expect(screen.getByText("pending")).toBeInTheDocument();
    expect(screen.getByText("revoked")).toBeInTheDocument();
    expect(screen.getAllByText("—").length).toBeGreaterThanOrEqual(1);
  });

  it("refetches invitations when refresh is clicked", async () => {
    const user = userEvent.setup();
    const refetchFn = jest.fn();
    useInvitations.mockReturnValue({
      data: [],
      isLoading: false,
      refetch: refetchFn,
    });
    useSendInvite.mockReturnValue({
      mutate: jest.fn(),
      isPending: false,
    });

    renderWithProviders(<InvitePage />);

    const refreshButton = screen.getByRole("button", { name: /refresh/i });
    await user.click(refreshButton);

    expect(refetchFn).toHaveBeenCalled();
  });
});
