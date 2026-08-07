import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import EmployeeLoginPage from "../page";

const mockPush = jest.fn();
const mockReplace = jest.fn();
const mockRefreshSubject = jest.fn();
const mockMutateAsync = jest.fn();

jest.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, replace: mockReplace }),
  useParams: () => ({ company: "acme" }),
}));

jest.mock("@/components/providers", () => ({
  useAuth: jest.fn(),
}));

jest.mock("@/hooks/use-auth", () => ({
  useLogin: jest.fn(),
}));

jest.mock("@/hooks/use-company", () => ({
  useCompanyBySlug: jest.fn(),
}));

jest.mock("@/lib/auth", () => ({
  setTokens: jest.fn(),
  setSubjectHint: jest.fn(),
}));

const { useAuth } = jest.requireMock("@/components/providers");
const { useLogin } = jest.requireMock("@/hooks/use-auth");
const { useCompanyBySlug } = jest.requireMock("@/hooks/use-company");
const { setTokens, setSubjectHint } = jest.requireMock("@/lib/auth");

describe("EmployeeLoginPage", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    useAuth.mockReturnValue({
      user: null,
      subjectType: null,
      loading: false,
      signOut: jest.fn(),
      refreshSubject: mockRefreshSubject,
    });
    useLogin.mockReturnValue({
      mutateAsync: mockMutateAsync,
      isPending: false,
    });
    useCompanyBySlug.mockReturnValue({
      data: { slug: "acme", name: "acme" },
      isLoading: false,
      isError: false,
    });
  });

  it("renders the login form and secondary links", () => {
    render(<EmployeeLoginPage />);
    expect(screen.getByText("acme")).toBeInTheDocument();
    expect(screen.getByLabelText(/email/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/pin/i)).toBeInTheDocument();
    expect(screen.getByText("Sign In")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /forgot your pin/i })).toHaveAttribute(
      "href",
      "/acme/forgot-pin"
    );
    expect(screen.getByRole("link", { name: /new here\? sign up/i })).toHaveAttribute(
      "href",
      "/acme/signup"
    );
    expect(screen.getByRole("link", { name: /company admin\? sign in/i })).toHaveAttribute(
      "href",
      "/acme/admin/login"
    );
  });

  it("submits email and pin through the login mutation", async () => {
    mockMutateAsync.mockResolvedValue({
      user: { id: "1" },
      access_token: "at",
      refresh_token: "rt",
      expires_in: 900,
    });
    mockRefreshSubject.mockResolvedValue(undefined);

    const user = userEvent.setup();
    render(<EmployeeLoginPage />);

    await user.type(screen.getByLabelText(/email/i), "employee@example.com");
    await user.type(screen.getByLabelText(/pin/i), "12345");
    await user.click(screen.getByText("Sign In"));

    expect(mockMutateAsync).toHaveBeenCalledWith({
      email: "employee@example.com",
      pin: "12345",
      company_slug: "acme",
    });
  });

  it("shows an inline validation error for a non-5-digit PIN without submitting", async () => {
    const user = userEvent.setup();
    render(<EmployeeLoginPage />);

    await user.type(screen.getByLabelText(/email/i), "employee@example.com");
    await user.type(screen.getByLabelText(/pin/i), "123");
    await user.click(screen.getByText("Sign In"));

    expect(screen.getByText("PIN must be exactly 5 digits")).toBeInTheDocument();
    expect(mockMutateAsync).not.toHaveBeenCalled();
  });

  it("stores tokens, sets the user hint, and redirects on success", async () => {
    mockMutateAsync.mockResolvedValue({
      user: { id: "1" },
      access_token: "at",
      refresh_token: "rt",
      expires_in: 900,
    });
    mockRefreshSubject.mockResolvedValue(undefined);

    const user = userEvent.setup();
    render(<EmployeeLoginPage />);

    await user.type(screen.getByLabelText(/email/i), "employee@example.com");
    await user.type(screen.getByLabelText(/pin/i), "12345");
    await user.click(screen.getByText("Sign In"));

    expect(setTokens).toHaveBeenCalledWith("at", "rt");
    expect(setSubjectHint).toHaveBeenCalledWith("user");
    expect(mockRefreshSubject).toHaveBeenCalled();
    expect(mockPush).toHaveBeenCalledWith("/acme");
  });

  it("redirects immediately when already authenticated as a user on mount", () => {
    useAuth.mockReturnValue({
      user: { id: "1", email: "employee@example.com" },
      subjectType: "user",
      loading: false,
      signOut: jest.fn(),
      refreshSubject: mockRefreshSubject,
    });

    render(<EmployeeLoginPage />);

    expect(mockReplace).toHaveBeenCalledWith("/acme");
  });

  it("shows 'Invalid email or PIN' for invalid_credentials", async () => {
    mockMutateAsync.mockRejectedValue({
      response: { data: { error: "Invalid email or PIN", code: "invalid_credentials" } },
    });

    const user = userEvent.setup();
    render(<EmployeeLoginPage />);

    await user.type(screen.getByLabelText(/email/i), "a@b.com");
    await user.type(screen.getByLabelText(/pin/i), "12345");
    await user.click(screen.getByText("Sign In"));

    expect(await screen.findByText("Invalid email or PIN")).toBeInTheDocument();
  });

  it("shows the suspension copy for company_suspended", async () => {
    mockMutateAsync.mockRejectedValue({
      response: {
        data: {
          error: "Your company account is suspended. Please contact support.",
          code: "company_suspended",
        },
      },
    });

    const user = userEvent.setup();
    render(<EmployeeLoginPage />);

    await user.type(screen.getByLabelText(/email/i), "a@b.com");
    await user.type(screen.getByLabelText(/pin/i), "12345");
    await user.click(screen.getByText("Sign In"));

    expect(
      await screen.findByText("Your company account is suspended. Please contact support.")
    ).toBeInTheDocument();
  });

  it("shows the server message verbatim for account_locked", async () => {
    mockMutateAsync.mockRejectedValue({
      response: {
        data: { error: "Your account has been locked. Please contact your manager.", code: "account_locked" },
      },
    });

    const user = userEvent.setup();
    render(<EmployeeLoginPage />);

    await user.type(screen.getByLabelText(/email/i), "a@b.com");
    await user.type(screen.getByLabelText(/pin/i), "12345");
    await user.click(screen.getByText("Sign In"));

    expect(
      await screen.findByText("Your account has been locked. Please contact your manager.")
    ).toBeInTheDocument();
  });

  it("shows a generic failure message when the rejection has no response", async () => {
    mockMutateAsync.mockRejectedValue(new Error("Network Error"));

    const user = userEvent.setup();
    render(<EmployeeLoginPage />);

    await user.type(screen.getByLabelText(/email/i), "a@b.com");
    await user.type(screen.getByLabelText(/pin/i), "12345");
    await user.click(screen.getByText("Sign In"));

    expect(await screen.findByText("Failed to sign in")).toBeInTheDocument();
  });

  it("does not render the login form while the company slug is still resolving", () => {
    useCompanyBySlug.mockReturnValue({ data: undefined, isLoading: true, isError: false });

    render(<EmployeeLoginPage />);

    expect(screen.queryByLabelText(/email/i)).not.toBeInTheDocument();
  });

  it("shows a 'company not found' message instead of the login form for an unknown slug", () => {
    useCompanyBySlug.mockReturnValue({ data: undefined, isLoading: false, isError: true });

    render(<EmployeeLoginPage />);

    expect(screen.getByText("Company not found")).toBeInTheDocument();
    expect(screen.queryByLabelText(/email/i)).not.toBeInTheDocument();
  });
});
