import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Toaster } from "sonner";
import InvitePage from "../page";

jest.mock("@/hooks/use-invitations", () => ({
  useInvitations: jest.fn(),
  useSendInvite: jest.fn(),
}));

const { useInvitations, useSendInvite } = jest.requireMock(
  "@/hooks/use-invitations"
);

function renderWithProviders(ui: React.ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <Toaster />
      {ui}
    </QueryClientProvider>
  );
}

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
