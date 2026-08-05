import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import SignupPage from "../page";

const mockPush = jest.fn();
const mockRefetch = jest.fn();
const mockMutateAsync = jest.fn();
const mockUseSearchParams = jest.fn(() => new URLSearchParams());

jest.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush }),
  useParams: () => ({ company: "acme" }),
  useSearchParams: () => mockUseSearchParams(),
}));

jest.mock("@/hooks/use-auth", () => ({
  useCheckInvite: jest.fn(),
  useSendEmailOtp: jest.fn(),
}));

jest.mock("sonner", () => ({
  toast: { info: jest.fn(), error: jest.fn() },
}));

const { useCheckInvite, useSendEmailOtp } = jest.requireMock("@/hooks/use-auth");
const { toast } = jest.requireMock("sonner");

const mockSearchParams = (email: string) =>
  mockUseSearchParams.mockReturnValue(
    email ? new URLSearchParams({ email }) : new URLSearchParams()
  );

describe("SignupPage", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockSearchParams("");
    useCheckInvite.mockReturnValue({ refetch: mockRefetch });
    useSendEmailOtp.mockReturnValue({ mutateAsync: mockMutateAsync, isPending: false });
  });

  it("renders the signup form with a continue button", () => {
    render(<SignupPage />);
    expect(screen.getByText("Create your account")).toBeInTheDocument();
    expect(screen.getByLabelText(/email/i)).toBeInTheDocument();
    expect(screen.getByText("Continue")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /already have an account\? sign in/i })).toHaveAttribute(
      "href",
      "/acme/login"
    );
  });

  it("pre-fills the email field from the ?email= query param", () => {
    mockSearchParams("sarah@acme.com");
    render(<SignupPage />);
    expect(screen.getByLabelText(/email/i)).toHaveValue("sarah@acme.com");
  });

  it("shows an inline validation error for an invalid email without checking", async () => {
    const user = userEvent.setup();
    render(<SignupPage />);

    await user.type(screen.getByLabelText(/email/i), "a@b");
    await user.click(screen.getByText("Continue"));

    expect(screen.getByText("Enter a valid email address")).toBeInTheDocument();
    expect(mockRefetch).not.toHaveBeenCalled();
  });

  it("shows the no-invitation error when check-invite rejects", async () => {
    mockRefetch.mockRejectedValue(new Error("no_invitation"));

    const user = userEvent.setup();
    render(<SignupPage />);

    await user.type(screen.getByLabelText(/email/i), "sarah@acme.com");
    await user.click(screen.getByText("Continue"));

    expect(
      await screen.findByText("No invitation found for this email. Contact your manager.")
    ).toBeInTheDocument();
  });

  it("redirects to login when the invitation is already accepted", async () => {
    mockRefetch.mockResolvedValue({ data: { has_invitation: true, status: "accepted" } });

    const user = userEvent.setup();
    render(<SignupPage />);

    await user.type(screen.getByLabelText(/email/i), "sarah@acme.com");
    await user.click(screen.getByText("Continue"));

    expect(toast.info).toHaveBeenCalledWith("You already have an account — please sign in.");
    expect(mockPush).toHaveBeenCalledWith("/acme/login");
    expect(mockMutateAsync).not.toHaveBeenCalled();
  });

  it("sends an OTP and navigates to verify for a pending invitation", async () => {
    mockRefetch.mockResolvedValue({ data: { has_invitation: true, status: "pending" } });
    mockMutateAsync.mockResolvedValue(undefined);

    const user = userEvent.setup();
    render(<SignupPage />);

    await user.type(screen.getByLabelText(/email/i), "sarah@acme.com");
    await user.click(screen.getByText("Continue"));

    await screen.findByText("Continue");
    expect(mockMutateAsync).toHaveBeenCalledWith("sarah@acme.com");
    expect(mockPush).toHaveBeenCalledWith(
      "/acme/verify?email=sarah%40acme.com&purpose=signup"
    );
  });

  it("sends an OTP for a sent invitation too", async () => {
    mockRefetch.mockResolvedValue({ data: { has_invitation: true, status: "sent" } });
    mockMutateAsync.mockResolvedValue(undefined);

    const user = userEvent.setup();
    render(<SignupPage />);

    await user.type(screen.getByLabelText(/email/i), "sarah@acme.com");
    await user.click(screen.getByText("Continue"));

    expect(mockMutateAsync).toHaveBeenCalledWith("sarah@acme.com");
    expect(mockPush).toHaveBeenCalledWith(
      "/acme/verify?email=sarah%40acme.com&purpose=signup"
    );
  });

  it("shows the no-invitation error for an unexpected status", async () => {
    mockRefetch.mockResolvedValue({ data: { has_invitation: true, status: "revoked" } });

    const user = userEvent.setup();
    render(<SignupPage />);

    await user.type(screen.getByLabelText(/email/i), "sarah@acme.com");
    await user.click(screen.getByText("Continue"));

    expect(
      await screen.findByText("No invitation found for this email. Contact your manager.")
    ).toBeInTheDocument();
  });
});
