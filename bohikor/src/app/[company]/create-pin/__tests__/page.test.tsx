import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import CreatePinPage from "../page";

const mockPush = jest.fn();
const mockRefreshSubject = jest.fn();
const mockMutateAsync = jest.fn();
const mockUseSearchParams = jest.fn(() =>
  new URLSearchParams("email=sarah%40acme.com")
);

jest.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush }),
  useParams: () => ({ company: "acme" }),
  useSearchParams: () => mockUseSearchParams(),
}));

jest.mock("@/hooks/use-auth", () => ({
  useCreatePin: jest.fn(),
}));

jest.mock("@/lib/auth", () => ({
  setTokens: jest.fn(),
  setSubjectHint: jest.fn(),
}));

jest.mock("@/components/providers", () => ({
  useAuth: jest.fn(),
}));

jest.mock("sonner", () => ({
  toast: { success: jest.fn(), error: jest.fn() },
}));

const { useCreatePin } = jest.requireMock("@/hooks/use-auth");
const { setTokens, setSubjectHint } = jest.requireMock("@/lib/auth");
const { useAuth } = jest.requireMock("@/components/providers");
const { toast } = jest.requireMock("sonner");

describe("CreatePinPage", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockUseSearchParams.mockReturnValue(new URLSearchParams("email=sarah%40acme.com"));
    useCreatePin.mockReturnValue({ mutateAsync: mockMutateAsync, isPending: false });
    useAuth.mockReturnValue({ refreshSubject: mockRefreshSubject });
  });

  it("renders PIN and confirm PIN fields", () => {
    render(<CreatePinPage />);
    expect(screen.getByText("Create your PIN")).toBeInTheDocument();
    expect(screen.getByLabelText(/^pin$/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/confirm pin/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Create account" })).toBeInTheDocument();
  });

  it("shows 'PINs do not match' for mismatched PINs without calling the mutation", async () => {
    const user = userEvent.setup();
    render(<CreatePinPage />);

    await user.type(screen.getByLabelText(/^pin$/i), "12345");
    await user.type(screen.getByLabelText(/confirm pin/i), "54321");
    await user.click(screen.getByRole("button", { name: "Create account" }));

    expect(screen.getByText("PINs do not match")).toBeInTheDocument();
    expect(mockMutateAsync).not.toHaveBeenCalled();
  });

  it("shows 'PIN must be 5 digits' for a short PIN without calling the mutation", async () => {
    const user = userEvent.setup();
    render(<CreatePinPage />);

    await user.type(screen.getByLabelText(/^pin$/i), "123");
    await user.type(screen.getByLabelText(/confirm pin/i), "123");
    await user.click(screen.getByRole("button", { name: "Create account" }));

    expect(screen.getByText("PIN must be 5 digits")).toBeInTheDocument();
    expect(mockMutateAsync).not.toHaveBeenCalled();
  });

  it("calls the mutation with the email and matching PINs", async () => {
    mockMutateAsync.mockResolvedValue({
      user: { id: "1" },
      access_token: "at",
      refresh_token: "rt",
      expires_in: 900,
    });
    mockRefreshSubject.mockResolvedValue(undefined);

    const user = userEvent.setup();
    render(<CreatePinPage />);

    await user.type(screen.getByLabelText(/^pin$/i), "12345");
    await user.type(screen.getByLabelText(/confirm pin/i), "12345");
    await user.click(screen.getByRole("button", { name: "Create account" }));

    expect(mockMutateAsync).toHaveBeenCalledWith({ email: "sarah@acme.com", pin: "12345" });
  });

  it("stores tokens and redirects to home on success", async () => {
    mockMutateAsync.mockResolvedValue({
      user: { id: "1" },
      access_token: "at",
      refresh_token: "rt",
      expires_in: 900,
    });
    mockRefreshSubject.mockResolvedValue(undefined);

    const user = userEvent.setup();
    render(<CreatePinPage />);

    await user.type(screen.getByLabelText(/^pin$/i), "12345");
    await user.type(screen.getByLabelText(/confirm pin/i), "12345");
    await user.click(screen.getByRole("button", { name: "Create account" }));

    expect(setTokens).toHaveBeenCalledWith("at", "rt");
    expect(setSubjectHint).toHaveBeenCalledWith("user");
    expect(mockRefreshSubject).toHaveBeenCalled();
    expect(toast.success).toHaveBeenCalledWith("Account created");
    expect(mockPush).toHaveBeenCalledWith("/acme");
  });

  it("shows the existing-account message with a login link for user_exists", async () => {
    mockMutateAsync.mockRejectedValue({
      response: {
        data: {
          error: "An account with this email already exists",
          code: "user_exists",
        },
      },
    });

    const user = userEvent.setup();
    render(<CreatePinPage />);

    await user.type(screen.getByLabelText(/^pin$/i), "12345");
    await user.type(screen.getByLabelText(/confirm pin/i), "12345");
    await user.click(screen.getByRole("button", { name: "Create account" }));

    expect(
      await screen.findByText(/an account with this email already exists/i)
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /please log in instead/i })).toHaveAttribute(
      "href",
      "/acme/login"
    );
  });

  it("shows the no-invitation error for no_invitation", async () => {
    mockMutateAsync.mockRejectedValue({
      response: { data: { error: "no_invitation", code: "no_invitation" } },
    });

    const user = userEvent.setup();
    render(<CreatePinPage />);

    await user.type(screen.getByLabelText(/^pin$/i), "12345");
    await user.type(screen.getByLabelText(/confirm pin/i), "12345");
    await user.click(screen.getByRole("button", { name: "Create account" }));

    expect(
      await screen.findByText("No invitation found for this email. Contact your manager.")
    ).toBeInTheDocument();
  });

  it("renders a fallback state when no email is provided", () => {
    mockUseSearchParams.mockReturnValue(new URLSearchParams());
    render(<CreatePinPage />);
    expect(
      screen.getByText(/couldn't load your sign-up details/i)
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /back to sign in/i })).toHaveAttribute(
      "href",
      "/acme/login"
    );
  });
});
