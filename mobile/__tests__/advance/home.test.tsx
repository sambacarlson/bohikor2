import { render, screen, fireEvent } from "@testing-library/react-native";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import HomeScreen from "@/app/(app)/home";

const mockRouter = { push: jest.fn(), back: jest.fn(), replace: jest.fn() };
const mockSignOut = jest.fn();
const mockRefreshUser = jest.fn();

const mockUser = {
  id: "user-1",
  email: "test@example.com",
  email_verified: true,
  full_name: "Test User",
  phone_number: "+237600000000",
  phone_verified: true,
  status: "active" as const,
  is_terms_accepted: true,
  terms_accepted_at: "2024-01-01T00:00:00Z",
  terms_version: "v1",
  user_ip_at_consent: null,
  created_at: "2024-01-01T00:00:00Z",
  updated_at: "2024-01-01T00:00:00Z",
};

const mockCreateRequest = {
  mutateAsync: jest.fn().mockResolvedValue({ id: "req-1" }),
  isPending: false,
  isError: false,
  isSuccess: false,
  error: null,
};

jest.mock("expo-router", () => ({
  useRouter: () => mockRouter,
}));

jest.mock("@/src/providers/auth-provider", () => ({
  useAuth: () => ({
    user: mockUser,
    loading: false,
    signOut: mockSignOut,
    refreshUser: mockRefreshUser,
  }),
}));

jest.mock("@/src/hooks/use-advance", () => ({
  useAcceptTerms: () => ({}),
  useCreateAdvanceRequest: () => mockCreateRequest,
  useAdvanceRequests: () => ({}),
}));

jest.mock("@/src/hooks/use-eligibility", () => ({
  useEligibility: () => ({
    data: {
      eligible: true,
      reasons: [],
      kill_switch_active: false,
      request_window: { start_day: 1, end_day: 31, in_window: true },
      daily_requests_remaining: 1,
      monthly_requests_remaining: 3,
      advance_amount_xaf: "10000",
      phone_verified: true,
      terms_accepted: true,
    },
    isLoading: false,
    isError: false,
  }),
}));

function renderWithProviders(ui: React.ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>);
}

describe("HomeScreen", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockUser.is_terms_accepted = true;
  });

  it("shows Request Advance button", () => {
    renderWithProviders(<HomeScreen />);
    expect(screen.getByTestId("request-advance-button")).toBeTruthy();
    expect(screen.getByText("Request Advance")).toBeTruthy();
    expect(screen.getAllByText("10000 XAF").length).toBe(2);
  });

  it("shows user email and phone in Your Information card", () => {
    renderWithProviders(<HomeScreen />);
    expect(screen.getByText("Your Information")).toBeTruthy();
    expect(screen.getByText("test@example.com")).toBeTruthy();
    expect(screen.getByText("+237600000000")).toBeTruthy();
  });

  it("shows terms accepted status", () => {
    renderWithProviders(<HomeScreen />);
    expect(screen.getByText("Accepted")).toBeTruthy();
  });

  it("shows account status", () => {
    renderWithProviders(<HomeScreen />);
    expect(screen.getByText("active")).toBeTruthy();
  });

  it("shows View Transaction History link", () => {
    renderWithProviders(<HomeScreen />);
    expect(screen.getByText("View Transaction History")).toBeTruthy();
  });

  it("navigates to history when View Transaction History is tapped", () => {
    renderWithProviders(<HomeScreen />);
    fireEvent.press(screen.getByTestId("view-history-link"));
    expect(mockRouter.push).toHaveBeenCalled();
  });

  it("renders dropdown menu button", () => {
    renderWithProviders(<HomeScreen />);
    expect(screen.getByTestId("menu-button")).toBeTruthy();
  });

  it("shows Not accepted when terms not accepted", () => {
    mockUser.is_terms_accepted = false;
    renderWithProviders(<HomeScreen />);
    expect(screen.getByText("Not accepted")).toBeTruthy();
    mockUser.is_terms_accepted = true;
  });

  it("shows terms warning when terms not accepted", () => {
    mockUser.is_terms_accepted = false;
    renderWithProviders(<HomeScreen />);
    expect(screen.getByText("Terms not accepted")).toBeTruthy();
    mockUser.is_terms_accepted = true;
  });
});