import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import LoginPage from "../page";

const mockPush = jest.fn();
const mockReplace = jest.fn();

jest.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, replace: mockReplace }),
}));

jest.mock("@/hooks/use-user", () => ({
  useUser: jest.fn(),
}));

jest.mock("@/components/providers", () => ({
  useAuth: jest.fn(),
}));

const sendCodeMock = jest.fn();

jest.mock("firebase/auth", () => ({
  RecaptchaVerifier: jest.fn().mockImplementation(() => ({
    clear: jest.fn(),
  })),
  signInWithPhoneNumber: jest.fn().mockImplementation(() => sendCodeMock()),
}));

jest.mock("@/lib/firebase", () => ({
  auth: {},
}));

const { useUser } = jest.requireMock("@/hooks/use-user");
const { useAuth } = jest.requireMock("@/components/providers");
const { signInWithPhoneNumber } = jest.requireMock("firebase/auth");

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

describe("LoginPage", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    useUser.mockReturnValue({
      data: null,
      isLoading: false,
      isFetched: false,
      refetch: jest.fn(),
    });
    useAuth.mockReturnValue({
      user: null,
      loading: false,
    });
    sendCodeMock.mockResolvedValue({});
    signInWithPhoneNumber.mockResolvedValue({
      confirm: jest.fn().mockResolvedValue({}),
    });
  });

  it("renders phone input and continue button", () => {
    renderWithProviders(<LoginPage />);
    expect(screen.getByText("Bohikor2")).toBeInTheDocument();
    expect(screen.getByText("Salary Advance Pilot")).toBeInTheDocument();
    expect(
      screen.getByPlaceholderText(/e.g. 671234567/)
    ).toBeInTheDocument();
    expect(screen.getByText("Continue")).toBeInTheDocument();
  });

  it("shows login as admin link", () => {
    renderWithProviders(<LoginPage />);
    expect(screen.getByText("Login as admin")).toBeInTheDocument();
  });

  it("disables continue button when phone is empty", () => {
    renderWithProviders(<LoginPage />);
    const continueBtn = screen.getByText("Continue");
    expect(continueBtn).toBeDisabled();
  });

  it("sends code when phone is entered", async () => {
    const user = userEvent.setup();
    renderWithProviders(<LoginPage />);
    const input = screen.getByPlaceholderText(/e.g. 671234567/);
    await user.type(input, "671234567");

    const continueBtn = screen.getByText("Continue");
    await user.click(continueBtn);

    expect(signInWithPhoneNumber).toHaveBeenCalled();
  });

  it("shows OTP step after code sent", async () => {
    const user = userEvent.setup();
    const mockConfirm = jest.fn().mockResolvedValue({});
    signInWithPhoneNumber.mockResolvedValue({
      confirm: mockConfirm,
    });

    renderWithProviders(<LoginPage />);
    const input = screen.getByPlaceholderText(/e.g. 671234567/);
    await user.type(input, "671234567");

    const continueBtn = screen.getByText("Continue");
    await user.click(continueBtn);

    expect(screen.getByText("Verification Code")).toBeInTheDocument();
    expect(
      screen.getByPlaceholderText("000000")
    ).toBeInTheDocument();
  });

  it("shows error when code sending fails", async () => {
    const user = userEvent.setup();
    signInWithPhoneNumber.mockRejectedValue(new Error("invalid-phone-number"));

    renderWithProviders(<LoginPage />);
    const input = screen.getByPlaceholderText(/e.g. 671234567/);
    await user.type(input, "600000000");

    const continueBtn = screen.getByText("Continue");
    await user.click(continueBtn);

    expect(
      screen.getByText("Invalid phone number. Please check and try again.")
    ).toBeInTheDocument();
  });

  it("shows change phone number link in OTP step", async () => {
    const user = userEvent.setup();
    signInWithPhoneNumber.mockResolvedValue({
      confirm: jest.fn().mockResolvedValue({}),
    });

    renderWithProviders(<LoginPage />);
    const input = screen.getByPlaceholderText(/e.g. 671234567/);
    await user.type(input, "671234567");

    const continueBtn = screen.getByText("Continue");
    await user.click(continueBtn);

    expect(
      screen.getByText("Change phone number")
    ).toBeInTheDocument();
  });
});
