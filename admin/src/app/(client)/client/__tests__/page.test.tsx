import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import ClientHomePage from "../page";

const mockPush = jest.fn();
const mockReplace = jest.fn();
const mockSignOut = jest.fn();

jest.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, replace: mockReplace }),
}));

jest.mock("@/hooks/use-user", () => ({
  useUser: jest.fn(),
}));

jest.mock("@/hooks/use-advance", () => ({
  useCreateAdvanceRequest: jest.fn(),
}));

jest.mock("@/components/providers", () => ({
  useAuth: jest.fn(),
}));

jest.mock("@/lib/api", () => ({
  getApiErrorMessage: jest.fn(() => "Something went wrong. Please try again."),
}));

const { useUser } = jest.requireMock("@/hooks/use-user");
const { useCreateAdvanceRequest } = jest.requireMock("@/hooks/use-advance");
const { useAuth } = jest.requireMock("@/components/providers");

function renderWithProviders(ui: React.ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  return render(
    <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>
  );
}

const mockUser = {
  id: "user-1",
  email: "employee@example.com",
  full_name: "John Doe",
  phone_number: "+237671234567",
  status: "active",
  is_terms_accepted: true,
  email_verified: true,
  phone_verified: true,
  password_hash: "$2a$10$hashed",
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
};

describe("ClientHomePage", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    useUser.mockReturnValue({
      data: mockUser,
      isLoading: false,
      refetch: jest.fn(),
    });
    useCreateAdvanceRequest.mockReturnValue({
      mutateAsync: jest.fn(),
      isPending: false,
      isError: false,
      error: null,
    });
    useAuth.mockReturnValue({
      signOut: mockSignOut,
    });
  });

  it("renders user name and email", () => {
    renderWithProviders(<ClientHomePage />);
    const nameElements = screen.getAllByText("John Doe");
    expect(nameElements.length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText("employee@example.com")).toBeInTheDocument();
  });

  it("renders request advance button", () => {
    renderWithProviders(<ClientHomePage />);
    expect(
      screen.getByTestId("request-advance-button")
    ).toBeInTheDocument();
  });

  it("shows view transaction history link", () => {
    renderWithProviders(<ClientHomePage />);
    expect(screen.getByTestId("view-history-link")).toBeInTheDocument();
  });

  it("navigates to terms if terms not accepted", async () => {
    const user = userEvent.setup();
    useUser.mockReturnValue({
      data: { ...mockUser, is_terms_accepted: false },
      isLoading: false,
      refetch: jest.fn(),
    });

    renderWithProviders(<ClientHomePage />);
    const requestBtn = screen.getByTestId("request-advance-button");
    await user.click(requestBtn);

    expect(mockPush).toHaveBeenCalledWith("/client/terms");
  });

  it("shows confirmation dialog when terms accepted and clicking request", async () => {
    const user = userEvent.setup();

    renderWithProviders(<ClientHomePage />);
    const requestBtn = screen.getByTestId("request-advance-button");
    await user.click(requestBtn);

    expect(screen.getByText("Confirm Advance Request")).toBeInTheDocument();
  });

  it("calls createRequest on confirm", async () => {
    const user = userEvent.setup();
    const mutateAsync = jest.fn().mockResolvedValue({});
    useCreateAdvanceRequest.mockReturnValue({
      mutateAsync,
      isPending: false,
      isError: false,
      error: null,
    });

    renderWithProviders(<ClientHomePage />);
    const requestBtn = screen.getByTestId("request-advance-button");
    await user.click(requestBtn);

    const confirmBtn = screen.getByTestId("confirm-request-button");
    await user.click(confirmBtn);

    expect(mutateAsync).toHaveBeenCalledWith({
      phoneNumber: "+237671234567",
    });
  });

  it("navigates to history after successful request", async () => {
    const user = userEvent.setup();
    const mutateAsync = jest.fn().mockResolvedValue({});
    useCreateAdvanceRequest.mockReturnValue({
      mutateAsync,
      isPending: false,
      isError: false,
      error: null,
    });

    renderWithProviders(<ClientHomePage />);
    const requestBtn = screen.getByTestId("request-advance-button");
    await user.click(requestBtn);

    const confirmBtn = screen.getByTestId("confirm-request-button");
    await user.click(confirmBtn);

    expect(mockPush).toHaveBeenCalledWith("/client/history");
  });

  it("shows terms warning when terms not accepted", () => {
    useUser.mockReturnValue({
      data: { ...mockUser, is_terms_accepted: false },
      isLoading: false,
      refetch: jest.fn(),
    });

    renderWithProviders(<ClientHomePage />);
    expect(screen.getByTestId("accept-terms-link")).toBeInTheDocument();
  });

  it("hides terms warning when terms accepted", () => {
    renderWithProviders(<ClientHomePage />);
    expect(screen.queryByTestId("accept-terms-link")).not.toBeInTheDocument();
  });

  it("shows user info in card", () => {
    renderWithProviders(<ClientHomePage />);
    expect(screen.getByText("Your Information")).toBeInTheDocument();
    expect(screen.getByText("+237671234567")).toBeInTheDocument();
  });

  it("signs out when menu item is clicked", async () => {
    const user = userEvent.setup();

    renderWithProviders(<ClientHomePage />);
    const menuBtn = screen.getByTestId("menu-button");
    await user.click(menuBtn);

    const signOutItem = screen.getByTestId("signout-menu-item");
    await user.click(signOutItem);

    expect(mockSignOut).toHaveBeenCalled();
    expect(mockReplace).toHaveBeenCalledWith("/login");
  });
});