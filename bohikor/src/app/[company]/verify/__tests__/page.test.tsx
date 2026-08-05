import { render, screen } from "@testing-library/react";
import { act, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import VerifyPage from "../page";

const RESEND_SECONDS = 60;

const mockPush = jest.fn();
const mockRefreshSubject = jest.fn();
const mockVerifyOtp = jest.fn();
const mockSendOtp = jest.fn();
const mockForgotPin = jest.fn();
const mockUseSearchParams = jest.fn(() => new URLSearchParams("email=sarah%40acme.com&purpose=signup"));

jest.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush }),
  useParams: () => ({ company: "acme" }),
  useSearchParams: () => mockUseSearchParams(),
}));

jest.mock("@/hooks/use-auth", () => ({
  useVerifyEmailOtp: jest.fn(),
  useSendEmailOtp: jest.fn(),
  useForgotPin: jest.fn(),
}));

jest.mock("@/lib/auth", () => ({
  setTokens: jest.fn(),
  setSubjectHint: jest.fn(),
}));

jest.mock("@/components/providers", () => ({
  useAuth: jest.fn(),
}));

const { useVerifyEmailOtp, useSendEmailOtp, useForgotPin } =
  jest.requireMock("@/hooks/use-auth");
const { setTokens, setSubjectHint } = jest.requireMock("@/lib/auth");
const { useAuth } = jest.requireMock("@/components/providers");

const mockSearchParams = (query: string) =>
  mockUseSearchParams.mockReturnValue(new URLSearchParams(query));

describe("VerifyPage", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockSearchParams("email=sarah%40acme.com&purpose=signup");
    useVerifyEmailOtp.mockReturnValue({ mutateAsync: mockVerifyOtp, isPending: false });
    useSendEmailOtp.mockReturnValue({ mutateAsync: mockSendOtp, isPending: false });
    useForgotPin.mockReturnValue({ mutateAsync: mockForgotPin, isPending: false });
    useAuth.mockReturnValue({ refreshSubject: mockRefreshSubject });
  });

  it("renders the code input and a disabled resend button with countdown", () => {
    render(<VerifyPage />);
    expect(screen.getByText("Verify your email")).toBeInTheDocument();
    expect(screen.getByLabelText(/verification code/i)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /resend code in 60s/i })
    ).toBeDisabled();
  });

  it("shows the email in the description", () => {
    render(<VerifyPage />);
    expect(screen.getByText(/enter the 6-digit code we sent to/i)).toBeInTheDocument();
    expect(screen.getByText("sarah@acme.com")).toBeInTheDocument();
  });

  it("shows an inline error for a code that is not 6 digits without verifying", async () => {
    const user = userEvent.setup();
    render(<VerifyPage />);

    await user.type(screen.getByLabelText(/verification code/i), "123");
    await user.click(screen.getByRole("button", { name: "Verify" }));

    expect(screen.getByText("Enter the 6-digit code")).toBeInTheDocument();
    expect(mockVerifyOtp).not.toHaveBeenCalled();
  });

  it("routes to create-pin for a successful signup verification", async () => {
    mockVerifyOtp.mockResolvedValue(null);

    const user = userEvent.setup();
    render(<VerifyPage />);

    await user.type(screen.getByLabelText(/verification code/i), "123456");
    await user.click(screen.getByRole("button", { name: "Verify" }));

    expect(mockVerifyOtp).toHaveBeenCalledWith({
      email: "sarah@acme.com",
      code: "123456",
      purpose: "signup",
    });
    expect(mockPush).toHaveBeenCalledWith(
      "/acme/create-pin?email=sarah%40acme.com"
    );
    expect(setTokens).not.toHaveBeenCalled();
    expect(setSubjectHint).not.toHaveBeenCalled();
  });

  it("stores tokens and routes to reset-pin for a pin_reset verification", async () => {
    mockSearchParams("email=sarah%40acme.com&purpose=pin_reset");
    mockVerifyOtp.mockResolvedValue({
      user: { id: "1" },
      access_token: "at",
      refresh_token: "rt",
      expires_in: 900,
    });
    mockRefreshSubject.mockResolvedValue(undefined);

    const user = userEvent.setup();
    render(<VerifyPage />);

    await user.type(screen.getByLabelText(/verification code/i), "123456");
    await user.click(screen.getByRole("button", { name: "Verify" }));

    expect(setTokens).toHaveBeenCalledWith("at", "rt");
    expect(setSubjectHint).toHaveBeenCalledWith("user");
    expect(mockRefreshSubject).toHaveBeenCalled();
    expect(mockPush).toHaveBeenCalledWith("/acme/reset-pin");
  });

  it("shows 'Invalid OTP code' for an invalid_otp rejection", async () => {
    mockVerifyOtp.mockRejectedValue({
      response: { data: { error: "Invalid OTP", code: "invalid_otp" } },
    });

    const user = userEvent.setup();
    render(<VerifyPage />);

    await user.type(screen.getByLabelText(/verification code/i), "000000");
    await user.click(screen.getByRole("button", { name: "Verify" }));

    expect(await screen.findByText("Invalid OTP code")).toBeInTheDocument();
  });

  it("shows the server message verbatim for a blocked OTP", async () => {
    mockVerifyOtp.mockRejectedValue({
      response: {
        data: {
          error: "Too many attempts. Try again in 5 minutes.",
          code: "otp_temporarily_blocked",
        },
      },
    });

    const user = userEvent.setup();
    render(<VerifyPage />);

    await user.type(screen.getByLabelText(/verification code/i), "000000");
    await user.click(screen.getByRole("button", { name: "Verify" }));

    expect(
      await screen.findByText("Too many attempts. Try again in 5 minutes.")
    ).toBeInTheDocument();
  });

  it("re-enables resend after the countdown and resends the code", () => {
    jest.useFakeTimers();
    render(<VerifyPage />);

    expect(screen.getByRole("button", { name: /resend code in/i })).toBeDisabled();

    for (let i = 0; i < RESEND_SECONDS; i++) {
      act(() => {
        jest.advanceTimersByTime(1000);
      });
    }

    const resendBtn = screen.getByRole("button", { name: "Resend code" });
    expect(resendBtn).not.toBeDisabled();

    fireEvent.click(resendBtn);
    expect(mockSendOtp).toHaveBeenCalledWith("sarah@acme.com");

    jest.useRealTimers();
  });

  it("uses forgot-pin to resend when purpose is pin_reset", () => {
    mockSearchParams("email=sarah%40acme.com&purpose=pin_reset");
    jest.useFakeTimers();
    render(<VerifyPage />);

    for (let i = 0; i < RESEND_SECONDS; i++) {
      act(() => {
        jest.advanceTimersByTime(1000);
      });
    }

    fireEvent.click(screen.getByRole("button", { name: "Resend code" }));
    expect(mockForgotPin).toHaveBeenCalledWith("sarah@acme.com");
    expect(mockSendOtp).not.toHaveBeenCalled();

    jest.useRealTimers();
  });

  it("renders a fallback state when no email is provided", () => {
    mockSearchParams("purpose=signup");
    render(<VerifyPage />);
    expect(
      screen.getByText(/couldn't load your verification details/i)
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /back to sign in/i })).toHaveAttribute(
      "href",
      "/acme/login"
    );
  });
});
