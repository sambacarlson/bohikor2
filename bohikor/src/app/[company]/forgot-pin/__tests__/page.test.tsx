import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import ForgotPinPage from "../page";

const mockPush = jest.fn();
const mockMutateAsync = jest.fn();

jest.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush }),
  useParams: () => ({ company: "acme" }),
}));

jest.mock("@/hooks/use-auth", () => ({
  useForgotPin: jest.fn(),
}));

const { useForgotPin } = jest.requireMock("@/hooks/use-auth");

describe("ForgotPinPage", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    useForgotPin.mockReturnValue({ mutateAsync: mockMutateAsync, isPending: false });
  });

  it("renders the form with a back link", () => {
    render(<ForgotPinPage />);
    expect(screen.getByText("Forgot PIN?")).toBeInTheDocument();
    expect(screen.getByLabelText(/email/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Send Reset Code" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /back to login/i })).toHaveAttribute(
      "href",
      "/acme/login"
    );
  });

  it("shows a validation error for an invalid email without calling the mutation", async () => {
    const user = userEvent.setup();
    render(<ForgotPinPage />);

    await user.type(screen.getByLabelText(/email/i), "a@b");
    await user.click(screen.getByRole("button", { name: "Send Reset Code" }));

    expect(screen.getByText("Please enter a valid email address")).toBeInTheDocument();
    expect(mockMutateAsync).not.toHaveBeenCalled();
  });

  it("navigates to verify with purpose=pin_reset on success", async () => {
    mockMutateAsync.mockResolvedValue(undefined);

    const user = userEvent.setup();
    render(<ForgotPinPage />);

    await user.type(screen.getByLabelText(/email/i), "sarah@acme.com");
    await user.click(screen.getByRole("button", { name: "Send Reset Code" }));

    expect(mockMutateAsync).toHaveBeenCalledWith("sarah@acme.com");
    expect(mockPush).toHaveBeenCalledWith(
      "/acme/verify?email=sarah%40acme.com&purpose=pin_reset"
    );
  });

  it("shows 'No account found with this email' for not_found without navigating", async () => {
    mockMutateAsync.mockRejectedValue({
      response: { data: { error: "No account found", code: "not_found" } },
    });

    const user = userEvent.setup();
    render(<ForgotPinPage />);

    await user.type(screen.getByLabelText(/email/i), "unknown@acme.com");
    await user.click(screen.getByRole("button", { name: "Send Reset Code" }));

    expect(
      await screen.findByText("No account found with this email")
    ).toBeInTheDocument();
    expect(mockPush).not.toHaveBeenCalled();
  });

  it("shows the server message verbatim for account_locked", async () => {
    mockMutateAsync.mockRejectedValue({
      response: {
        data: {
          error: "Your account has been locked. Please contact your manager.",
          code: "account_locked",
        },
      },
    });

    const user = userEvent.setup();
    render(<ForgotPinPage />);

    await user.type(screen.getByLabelText(/email/i), "locked@acme.com");
    await user.click(screen.getByRole("button", { name: "Send Reset Code" }));

    expect(
      await screen.findByText("Your account has been locked. Please contact your manager.")
    ).toBeInTheDocument();
    expect(mockPush).not.toHaveBeenCalled();
  });
});
