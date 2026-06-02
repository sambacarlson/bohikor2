import { render, screen, fireEvent, waitFor } from "@testing-library/react-native";
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

describe("HomeScreen", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockUser.is_terms_accepted = true;
  });

  it("shows Request Advance button", () => {
    render(<HomeScreen />);
    expect(screen.getByTestId("request-advance-button")).toBeTruthy();
    expect(screen.getByText("Request Advance")).toBeTruthy();
    expect(screen.getByText("10,000 XAF")).toBeTruthy();
  });

  it("shows user email and phone in Your Information card", () => {
    render(<HomeScreen />);
    expect(screen.getByText("Your Information")).toBeTruthy();
    expect(screen.getByText("test@example.com")).toBeTruthy();
    expect(screen.getByText("+237600000000")).toBeTruthy();
  });

  it("shows terms accepted status", () => {
    render(<HomeScreen />);
    expect(screen.getByText("Accepted")).toBeTruthy();
  });

  it("shows account status", () => {
    render(<HomeScreen />);
    expect(screen.getByText("active")).toBeTruthy();
  });

  it("shows View Transaction History link", () => {
    render(<HomeScreen />);
    expect(screen.getByText("View Transaction History")).toBeTruthy();
  });

  it("navigates to history when View Transaction History is tapped", () => {
    render(<HomeScreen />);
    fireEvent.press(screen.getByTestId("view-history-link"));
    expect(mockRouter.push).toHaveBeenCalled();
  });

  it("renders dropdown menu button", () => {
    render(<HomeScreen />);
    expect(screen.getByTestId("menu-button")).toBeTruthy();
  });

  it("shows Not accepted when terms not accepted", () => {
    mockUser.is_terms_accepted = false;
    render(<HomeScreen />);
    expect(screen.getByText("Not accepted")).toBeTruthy();
    mockUser.is_terms_accepted = true;
  });

  it("shows terms warning when terms not accepted", () => {
    mockUser.is_terms_accepted = false;
    render(<HomeScreen />);
    expect(screen.getByText("Terms not accepted")).toBeTruthy();
    mockUser.is_terms_accepted = true;
  });
});