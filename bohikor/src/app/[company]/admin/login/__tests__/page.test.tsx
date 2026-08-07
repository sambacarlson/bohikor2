import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import AdminLoginPage from "../page";

const mockPush = jest.fn();
const mockReplace = jest.fn();
const mockRefreshSubject = jest.fn();

jest.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, replace: mockReplace }),
  useParams: () => ({ company: "acme" }),
}));

jest.mock("@/components/providers", () => ({
  useAuth: jest.fn(),
}));

jest.mock("@/lib/api", () => ({
  api: {
    post: jest.fn(),
  },
}));

jest.mock("@/lib/auth", () => ({
  setTokens: jest.fn(),
  setSubjectHint: jest.fn(),
}));

const { useAuth } = jest.requireMock("@/components/providers");
const { api } = jest.requireMock("@/lib/api");
const { setTokens, setSubjectHint } = jest.requireMock("@/lib/auth");

describe("AdminLoginPage", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    useAuth.mockReturnValue({
      admin: null,
      subjectType: null,
      loading: false,
      signOut: jest.fn(),
      refreshSubject: mockRefreshSubject,
    });
  });

  it("renders admin login form", () => {
    render(<AdminLoginPage />);
    expect(screen.getByText("acme")).toBeInTheDocument();
    expect(
      screen.getByText(/enter your credentials/i)
    ).toBeInTheDocument();
    expect(screen.getByLabelText(/email/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/password/i)).toBeInTheDocument();
    expect(screen.getByText("Sign In")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /employee\? sign in/i })).toHaveAttribute(
      "href",
      "/acme/login"
    );
  });

  it("calls api post on form submit", async () => {
    api.post.mockResolvedValue({
      data: { data: { access_token: "at", refresh_token: "rt" } },
    });
    mockRefreshSubject.mockResolvedValue(undefined);

    const user = userEvent.setup();
    render(<AdminLoginPage />);

    await user.type(screen.getByLabelText(/email/i), "admin@example.com");
    await user.type(screen.getByLabelText(/password/i), "password123");
    await user.click(screen.getByText("Sign In"));

    expect(api.post).toHaveBeenCalledWith("/api/auth/admin/login", {
      email: "admin@example.com",
      password: "password123",
      company_slug: "acme",
    });
  });

  it("redirects to the company admin dashboard on success", async () => {
    api.post.mockResolvedValue({
      data: { data: { access_token: "at", refresh_token: "rt" } },
    });
    mockRefreshSubject.mockResolvedValue(undefined);

    const user = userEvent.setup();
    render(<AdminLoginPage />);

    await user.type(screen.getByLabelText(/email/i), "admin@example.com");
    await user.type(screen.getByLabelText(/password/i), "password123");
    await user.click(screen.getByText("Sign In"));

    expect(setTokens).toHaveBeenCalledWith("at", "rt");
    expect(setSubjectHint).toHaveBeenCalledWith("admin");
    expect(mockPush).toHaveBeenCalledWith("/acme/admin");
  });

  it("redirects immediately when already authenticated on mount", () => {
    useAuth.mockReturnValue({
      admin: { id: "a1", email: "admin@example.com" },
      subjectType: "admin",
      loading: false,
      signOut: jest.fn(),
      refreshSubject: mockRefreshSubject,
    });

    render(<AdminLoginPage />);

    expect(mockReplace).toHaveBeenCalledWith("/acme/admin");
  });

  it("shows a generic failure message when the rejection has no response object", async () => {
    api.post.mockRejectedValue(new Error("Network Error"));

    const user = userEvent.setup();
    render(<AdminLoginPage />);

    await user.type(screen.getByLabelText(/email/i), "someone@example.com");
    await user.type(screen.getByLabelText(/password/i), "password123");
    await user.click(screen.getByText("Sign In"));

    expect(await screen.findByText("Failed to sign in")).toBeInTheDocument();
  });

  it("shows the fallback message when the response has no error field", async () => {
    api.post.mockRejectedValue({ response: { data: {} } });

    const user = userEvent.setup();
    render(<AdminLoginPage />);

    await user.type(screen.getByLabelText(/email/i), "someone@example.com");
    await user.type(screen.getByLabelText(/password/i), "password123");
    await user.click(screen.getByText("Sign In"));

    expect(
      await screen.findByText("Invalid email or password")
    ).toBeInTheDocument();
  });

  it("shows error on invalid credentials", async () => {
    api.post.mockRejectedValue({
      response: { data: { error: "Invalid email or password" } },
    });

    const user = userEvent.setup();
    render(<AdminLoginPage />);

    await user.type(screen.getByLabelText(/email/i), "wrong@example.com");
    await user.type(screen.getByLabelText(/password/i), "wrong");
    await user.click(screen.getByText("Sign In"));

    expect(
      screen.getByText("Invalid email or password")
    ).toBeInTheDocument();
  });
});
