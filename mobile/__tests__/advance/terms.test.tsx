import { render, screen, fireEvent, waitFor } from "@testing-library/react-native";
import TermsScreen from "@/app/(app)/terms";

const mockRouter = { back: jest.fn(), push: jest.fn() };
const mockRefreshUser = jest.fn();

const mockUser = {
  id: "user-1",
  email: "test@example.com",
  email_verified: true,
  full_name: "Test User",
  phone_number: "+237600000000",
  phone_verified: true,
  status: "active" as const,
  is_terms_accepted: false,
  terms_accepted_at: null,
  terms_version: null,
  user_ip_at_consent: null,
  created_at: "2024-01-01T00:00:00Z",
  updated_at: "2024-01-01T00:00:00Z",
};

jest.mock("expo-router", () => ({
  useRouter: () => mockRouter,
}));

jest.mock("@/src/providers/auth-provider", () => ({
  useAuth: () => ({
    user: mockUser,
    loading: false,
    signOut: jest.fn(),
    refreshUser: mockRefreshUser,
  }),
}));

jest.mock("@/src/hooks/use-advance", () => ({
  useAcceptTerms: () => ({
    mutateAsync: jest.fn().mockResolvedValue({}),
    isPending: false,
    isError: false,
    isSuccess: false,
  }),
  useCreateAdvanceRequest: () => ({}),
  useAdvanceRequests: () => ({}),
}));

describe("TermsScreen", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockUser.is_terms_accepted = false;
  });

  it("renders terms text and checkbox", () => {
    render(<TermsScreen />);
    expect(screen.getByText("Terms & Conditions")).toBeTruthy();
    expect(screen.getByText("I have read and accept the terms and conditions")).toBeTruthy();
  });

  it("has disabled accept button when checkbox is unchecked", () => {
    render(<TermsScreen />);
    const button = screen.getByTestId("accept-terms-button");
    expect(button.props.accessibilityState.disabled).toBe(true);
  });

  it("enables accept button after checking checkbox", () => {
    render(<TermsScreen />);
    const checkbox = screen.getByTestId("terms-checkbox");
    fireEvent.press(checkbox);
    const button = screen.getByTestId("accept-terms-button");
    expect(button.props.accessibilityState.disabled).toBe(false);
  });

  it("calls accept and navigates back on success", async () => {
    render(<TermsScreen />);
    const checkbox = screen.getByTestId("terms-checkbox");
    fireEvent.press(checkbox);
    const button = screen.getByTestId("accept-terms-button");
    fireEvent.press(button);
    await waitFor(() => {
      expect(mockRouter.back).toHaveBeenCalled();
    });
  });
});

describe("TermsScreen - already accepted", () => {
  it("shows already accepted message when terms are accepted", () => {
    mockUser.is_terms_accepted = true;
    render(<TermsScreen />);
    expect(screen.getByText("Terms Already Accepted")).toBeTruthy();
    mockUser.is_terms_accepted = false;
  });
});