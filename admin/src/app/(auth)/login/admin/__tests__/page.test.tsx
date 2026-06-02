import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import AdminLoginPage from "../page";

const mockPush = jest.fn();
const mockReplace = jest.fn();
const mockRefreshSubject = jest.fn();

jest.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, replace: mockReplace }),
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
}));

const { useAuth } = jest.requireMock("@/components/providers");
const { api } = jest.requireMock("@/lib/api");
const { setTokens } = jest.requireMock("@/lib/auth");

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
    expect(screen.getByText("Bohikor2 Admin")).toBeInTheDocument();
    expect(
      screen.getByText(/enter your credentials/i)
    ).toBeInTheDocument();
    expect(screen.getByLabelText(/email/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/password/i)).toBeInTheDocument();
    expect(screen.getByText("Sign In")).toBeInTheDocument();
  });

  it("shows back to login link", () => {
    render(<AdminLoginPage />);
    expect(screen.getByText("Back to login")).toBeInTheDocument();
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
    });
  });

  it("redirects to admin dashboard on success", async () => {
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
    expect(mockPush).toHaveBeenCalledWith("/admin");
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