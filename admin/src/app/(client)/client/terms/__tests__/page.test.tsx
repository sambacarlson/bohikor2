import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import TermsPage from "../page";

const mockBack = jest.fn();
const mockPush = jest.fn();

jest.mock("next/navigation", () => ({
  useRouter: () => ({ back: mockBack, push: mockPush }),
}));

jest.mock("@/hooks/use-user", () => ({
  useUser: jest.fn(),
}));

jest.mock("@/hooks/use-advance", () => ({
  useAcceptTerms: jest.fn(),
}));

const { useUser } = jest.requireMock("@/hooks/use-user");
const { useAcceptTerms } = jest.requireMock("@/hooks/use-advance");

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
  is_terms_accepted: false,
  status: "active",
  phone_number: "+237671234567",
  firebase_uid: "fb-uid",
  email_verified: true,
  phone_verified: true,
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
};

describe("TermsPage", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    useUser.mockReturnValue({
      data: mockUser,
      isLoading: false,
    });
    useAcceptTerms.mockReturnValue({
      mutateAsync: jest.fn(),
      isPending: false,
      isError: false,
    });
  });

  it("renders terms text", () => {
    renderWithProviders(<TermsPage />);
    expect(screen.getByText("Terms & Conditions")).toBeInTheDocument();
    expect(screen.getByText(/one-time pilot advance/i)).toBeInTheDocument();
  });

  it("shows checkbox and accept button", () => {
    renderWithProviders(<TermsPage />);
    expect(screen.getByTestId("terms-checkbox")).toBeInTheDocument();
    expect(
      screen.getByText("I have read and accept the terms and conditions")
    ).toBeInTheDocument();
    expect(
      screen.getByTestId("accept-terms-button")
    ).toBeInTheDocument();
  });

  it("accept button is disabled until checkbox is checked", () => {
    renderWithProviders(<TermsPage />);
    const acceptBtn = screen.getByTestId("accept-terms-button");
    expect(acceptBtn).toBeDisabled();
  });

  it("accept button is enabled after checkbox checked", async () => {
    const user = userEvent.setup();
    renderWithProviders(<TermsPage />);
    const checkbox = screen.getByTestId("terms-checkbox");
    await user.click(checkbox);

    const acceptBtn = screen.getByTestId("accept-terms-button");
    expect(acceptBtn).not.toBeDisabled();
  });

  it("calls acceptTerms on accept", async () => {
    const user = userEvent.setup();
    const mutateAsync = jest.fn().mockResolvedValue({});
    useAcceptTerms.mockReturnValue({
      mutateAsync,
      isPending: false,
      isError: false,
    });

    renderWithProviders(<TermsPage />);
    const checkbox = screen.getByTestId("terms-checkbox");
    await user.click(checkbox);

    const acceptBtn = screen.getByTestId("accept-terms-button");
    await user.click(acceptBtn);

    expect(mutateAsync).toHaveBeenCalledWith({ version: "v1" });
  });

  it("navigates to client home after accepting terms", async () => {
    const user = userEvent.setup();
    useUser.mockReturnValue({
      data: mockUser,
      isLoading: false,
    });
    const mutateAsync = jest.fn().mockResolvedValue({});
    useAcceptTerms.mockReturnValue({
      mutateAsync,
      isPending: false,
      isError: false,
    });

    renderWithProviders(<TermsPage />);
    const checkbox = screen.getByTestId("terms-checkbox");
    await user.click(checkbox);

    const acceptBtn = screen.getByTestId("accept-terms-button");
    await user.click(acceptBtn);

    expect(mockBack).not.toHaveBeenCalled();
  });

  it("shows already accepted state when user already accepted terms", () => {
    useUser.mockReturnValue({
      data: { ...mockUser, is_terms_accepted: true },
      isLoading: false,
    });

    renderWithProviders(<TermsPage />);
    expect(
      screen.getByText("Terms Already Accepted")
    ).toBeInTheDocument();
    expect(screen.getByText("Go Back")).toBeInTheDocument();
  });

  it("navigates to client home from already accepted state", async () => {
    const user = userEvent.setup();
    useUser.mockReturnValue({
      data: { ...mockUser, is_terms_accepted: true },
      isLoading: false,
    });

    renderWithProviders(<TermsPage />);
    const goBackBtn = screen.getByText("Go Back");
    await user.click(goBackBtn);

    expect(mockPush).toHaveBeenCalledWith("/client");
  });
});
