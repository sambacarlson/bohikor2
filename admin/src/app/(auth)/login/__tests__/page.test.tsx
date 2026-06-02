import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import LoginPage from "../page";

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

describe("LoginPage", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    useAuth.mockReturnValue({
      user: null,
      admin: null,
      subjectType: null,
      loading: false,
      signOut: jest.fn(),
      refreshSubject: mockRefreshSubject,
    });
  });

  it("renders phone input and continue button", () => {
    render(<LoginPage />);
    expect(screen.getByText("Bohikor2")).toBeInTheDocument();
    expect(screen.getByText("Salary Advance Pilot")).toBeInTheDocument();
    expect(
      screen.getByPlaceholderText(/e.g. 671234567/)
    ).toBeInTheDocument();
    expect(screen.getByText("Continue")).toBeInTheDocument();
  });

  it("shows login as admin link", () => {
    render(<LoginPage />);
    expect(screen.getByText("Login as admin")).toBeInTheDocument();
  });

  it("disables continue button when phone is empty", () => {
    render(<LoginPage />);
    const continueBtn = screen.getByText("Continue");
    expect(continueBtn).toBeDisabled();
  });

  it("sends OTP when phone is entered", async () => {
    api.post.mockResolvedValue({ data: {} });

    const user = userEvent.setup();
    render(<LoginPage />);
    await user.type(screen.getByPlaceholderText(/e.g. 671234567/), "671234567");
    await user.click(screen.getByText("Continue"));

    expect(api.post).toHaveBeenCalledWith("/api/auth/send-phone-otp", {
      phone_number: "+237671234567",
    });
  });

  it("shows OTP step after code sent", async () => {
    api.post.mockResolvedValue({ data: {} });

    const user = userEvent.setup();
    render(<LoginPage />);
    await user.type(screen.getByPlaceholderText(/e.g. 671234567/), "671234567");
    await user.click(screen.getByText("Continue"));

    expect(screen.getByText("Verification Code")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("000000")).toBeInTheDocument();
  });

  it("shows error when code sending fails", async () => {
    api.post.mockRejectedValue({
      response: { data: { error: "Failed to send verification code." } },
    });

    const user = userEvent.setup();
    render(<LoginPage />);
    await user.type(screen.getByPlaceholderText(/e.g. 671234567/), "600000000");
    await user.click(screen.getByText("Continue"));

    expect(
      screen.getByText("Failed to send verification code.")
    ).toBeInTheDocument();
  });

  it("shows change phone number link in OTP step", async () => {
    api.post.mockResolvedValue({ data: {} });

    const user = userEvent.setup();
    render(<LoginPage />);
    await user.type(screen.getByPlaceholderText(/e.g. 671234567/), "671234567");
    await user.click(screen.getByText("Continue"));

    expect(
      screen.getByText("Change phone number")
    ).toBeInTheDocument();
  });
});