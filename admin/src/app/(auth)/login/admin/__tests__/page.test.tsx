import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import AdminLoginPage from "../page";

const mockPush = jest.fn();
const mockReplace = jest.fn();

jest.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, replace: mockReplace }),
}));

jest.mock("@/components/providers", () => ({
  useAuth: jest.fn(),
}));

const signInMock = jest.fn();

jest.mock("firebase/auth", () => ({
  signInWithEmailAndPassword: jest.fn().mockImplementation(() => signInMock()),
}));

jest.mock("@/lib/firebase", () => ({
  auth: {},
}));

const { signInWithEmailAndPassword } = jest.requireMock("firebase/auth");
const { useAuth } = jest.requireMock("@/components/providers");

describe("AdminLoginPage", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    useAuth.mockReturnValue({
      user: null,
      loading: false,
    });
    signInMock.mockResolvedValue({});
    signInWithEmailAndPassword.mockImplementation(() => signInMock());
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

  it("shows loading spinner while auth is loading", () => {
    useAuth.mockReturnValue({ user: null, loading: true });
    const { container } = render(<AdminLoginPage />);
    expect(container.querySelector(".animate-spin")).toBeInTheDocument();
  });

  it("redirects to admin if already logged in", () => {
    useAuth.mockReturnValue({ user: { uid: "test" }, loading: false });
    render(<AdminLoginPage />);
    expect(mockReplace).toHaveBeenCalledWith("/admin");
  });

  it("calls signIn on form submit", async () => {
    const user = userEvent.setup();
    render(<AdminLoginPage />);

    const emailInput = screen.getByLabelText(/email/i);
    const passwordInput = screen.getByLabelText(/password/i);

    await user.type(emailInput, "admin@example.com");
    await user.type(passwordInput, "password123");

    const signInBtn = screen.getByText("Sign In");
    await user.click(signInBtn);

    expect(signInWithEmailAndPassword).toHaveBeenCalled();
  });

  it("redirects to admin dashboard on success", async () => {
    const user = userEvent.setup();
    signInMock.mockResolvedValue({});

    render(<AdminLoginPage />);

    const emailInput = screen.getByLabelText(/email/i);
    const passwordInput = screen.getByLabelText(/password/i);

    await user.type(emailInput, "admin@example.com");
    await user.type(passwordInput, "password123");

    const signInBtn = screen.getByText("Sign In");
    await user.click(signInBtn);

    expect(mockPush).toHaveBeenCalledWith("/admin");
  });

  it("shows error on invalid credentials", async () => {
    const user = userEvent.setup();
    const error = new Error("Firebase auth error");
    (error as Record<string, string>).code = "auth/invalid-credential";
    signInMock.mockRejectedValue(error);

    render(<AdminLoginPage />);

    const emailInput = screen.getByLabelText(/email/i);
    const passwordInput = screen.getByLabelText(/password/i);

    await user.type(emailInput, "wrong@example.com");
    await user.type(passwordInput, "wrong");

    const signInBtn = screen.getByText("Sign In");
    await user.click(signInBtn);

    expect(
      screen.getByText("Invalid email or password")
    ).toBeInTheDocument();
  });
});
