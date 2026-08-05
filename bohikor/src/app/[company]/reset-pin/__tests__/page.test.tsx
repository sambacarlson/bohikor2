import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import ResetPinPage from "../page";

const mockPush = jest.fn();
const mockReplace = jest.fn();
const mockRefreshSubject = jest.fn();
const mockMutateAsync = jest.fn();

jest.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, replace: mockReplace }),
  useParams: () => ({ company: "acme" }),
}));

jest.mock("@/hooks/use-user", () => ({
  useResetPin: jest.fn(),
}));

jest.mock("@/components/providers", () => ({
  useAuth: jest.fn(),
}));

jest.mock("sonner", () => ({
  toast: { success: jest.fn(), error: jest.fn() },
}));

const { useResetPin } = jest.requireMock("@/hooks/use-user");
const { useAuth } = jest.requireMock("@/components/providers");
const { toast } = jest.requireMock("sonner");

const baseUser = {
  id: "1",
  email: "sarah@acme.com",
  email_verified: true,
  full_name: null,
  phone_number: null,
  phone_verified: false,
  status: "active",
  is_terms_accepted: false,
  terms_accepted_at: null,
  terms_version: null,
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
};

describe("ResetPinPage", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    useResetPin.mockReturnValue({ mutateAsync: mockMutateAsync, isPending: false });
    useAuth.mockReturnValue({
      user: baseUser,
      loading: false,
      refreshSubject: mockRefreshSubject,
    });
  });

  it("renders the form when authenticated", () => {
    render(<ResetPinPage />);
    expect(screen.getByText("Set new PIN")).toBeInTheDocument();
    expect(screen.getByLabelText(/new pin/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/confirm pin/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Set New PIN" })).toBeInTheDocument();
  });

  it("shows 'PINs do not match' without calling the mutation", async () => {
    const user = userEvent.setup();
    render(<ResetPinPage />);

    await user.type(screen.getByLabelText(/new pin/i), "12345");
    await user.type(screen.getByLabelText(/confirm pin/i), "54321");
    await user.click(screen.getByRole("button", { name: "Set New PIN" }));

    expect(screen.getByText("PINs do not match")).toBeInTheDocument();
    expect(mockMutateAsync).not.toHaveBeenCalled();
  });

  it("shows 'PIN must be 5 digits' for a short PIN without calling the mutation", async () => {
    const user = userEvent.setup();
    render(<ResetPinPage />);

    await user.type(screen.getByLabelText(/new pin/i), "123");
    await user.type(screen.getByLabelText(/confirm pin/i), "123");
    await user.click(screen.getByRole("button", { name: "Set New PIN" }));

    expect(screen.getByText("PIN must be 5 digits")).toBeInTheDocument();
    expect(mockMutateAsync).not.toHaveBeenCalled();
  });

  it("calls the endpoint and redirects home on success", async () => {
    mockMutateAsync.mockResolvedValue({ message: "ok" });
    mockRefreshSubject.mockResolvedValue(undefined);

    const user = userEvent.setup();
    render(<ResetPinPage />);

    await user.type(screen.getByLabelText(/new pin/i), "12345");
    await user.type(screen.getByLabelText(/confirm pin/i), "12345");
    await user.click(screen.getByRole("button", { name: "Set New PIN" }));

    expect(mockMutateAsync).toHaveBeenCalledWith("12345");
    expect(mockRefreshSubject).toHaveBeenCalled();
    expect(toast.success).toHaveBeenCalledWith("PIN reset");
    expect(mockPush).toHaveBeenCalledWith("/acme");
  });

  it("shows a loading state while the session loads", () => {
    useAuth.mockReturnValue({
      user: null,
      loading: true,
      refreshSubject: mockRefreshSubject,
    });
    render(<ResetPinPage />);
    expect(screen.getByText("Loading...")).toBeInTheDocument();
  });

  it("redirects to login when not authenticated", () => {
    useAuth.mockReturnValue({
      user: null,
      loading: false,
      refreshSubject: mockRefreshSubject,
    });
    render(<ResetPinPage />);
    expect(mockReplace).toHaveBeenCalledWith("/acme/login");
  });
});
