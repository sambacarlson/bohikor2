import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import PlatformLoginPage from "../page";

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
  api: { post: jest.fn() },
}));

jest.mock("@/lib/auth", () => ({
  setTokens: jest.fn(),
  setSubjectHint: jest.fn(),
}));

const { useAuth } = jest.requireMock("@/components/providers");
const { api } = jest.requireMock("@/lib/api");
const { setTokens, setSubjectHint } = jest.requireMock("@/lib/auth");

describe("PlatformLoginPage", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    useAuth.mockReturnValue({
      subjectType: null,
      refreshSubject: mockRefreshSubject,
    });
  });

  it("renders the platform login form", () => {
    render(<PlatformLoginPage />);
    expect(screen.getByText("Platform Admin")).toBeInTheDocument();
    expect(screen.getByText(/sign in as a platform administrator/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/email/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/password/i)).toBeInTheDocument();
    expect(screen.getByText("Sign In")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /not a platform administrator/i })
    ).toHaveAttribute("href", "/");
  });

  it("redirects to /platform immediately when already a platform_admin", () => {
    useAuth.mockReturnValue({
      subjectType: "platform_admin",
      refreshSubject: mockRefreshSubject,
    });
    render(<PlatformLoginPage />);
    expect(mockReplace).toHaveBeenCalledWith("/platform");
  });

  it("submits credentials, stores tokens/hint, and redirects on success", async () => {
    api.post.mockResolvedValue({
      data: { data: { access_token: "at", refresh_token: "rt" } },
    });
    mockRefreshSubject.mockResolvedValue(undefined);

    const user = userEvent.setup();
    render(<PlatformLoginPage />);

    await user.type(screen.getByLabelText(/email/i), "platform@example.com");
    await user.type(screen.getByLabelText(/password/i), "password123");
    await user.click(screen.getByText("Sign In"));

    expect(api.post).toHaveBeenCalledWith("/api/auth/platform/login", {
      email: "platform@example.com",
      password: "password123",
    });
    expect(setTokens).toHaveBeenCalledWith("at", "rt");
    expect(setSubjectHint).toHaveBeenCalledWith("platform_admin");
    expect(mockPush).toHaveBeenCalledWith("/platform");
  });

  it("shows the response error message when present", async () => {
    api.post.mockRejectedValue({
      response: { data: { error: "Invalid platform credentials" } },
    });

    const user = userEvent.setup();
    render(<PlatformLoginPage />);

    await user.type(screen.getByLabelText(/email/i), "wrong@example.com");
    await user.type(screen.getByLabelText(/password/i), "wrong");
    await user.click(screen.getByText("Sign In"));

    expect(await screen.findByText("Invalid platform credentials")).toBeInTheDocument();
  });

  it("falls back to a generic invalid-credentials message when response has no error field", async () => {
    api.post.mockRejectedValue({ response: { data: {} } });

    const user = userEvent.setup();
    render(<PlatformLoginPage />);

    await user.type(screen.getByLabelText(/email/i), "wrong@example.com");
    await user.type(screen.getByLabelText(/password/i), "wrong");
    await user.click(screen.getByText("Sign In"));

    expect(await screen.findByText("Invalid email or password")).toBeInTheDocument();
  });

  it("shows a failed-to-sign-in message when the error has no response property", async () => {
    api.post.mockRejectedValue(new Error("network down"));

    const user = userEvent.setup();
    render(<PlatformLoginPage />);

    await user.type(screen.getByLabelText(/email/i), "x@example.com");
    await user.type(screen.getByLabelText(/password/i), "x");
    await user.click(screen.getByText("Sign In"));

    expect(await screen.findByText("Failed to sign in")).toBeInTheDocument();
  });
});
