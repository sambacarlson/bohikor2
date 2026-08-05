import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import EmployeeHomePage from "../page";

const mockPush = jest.fn();
const mockMutateAsync = jest.fn();
const mockRefetch = jest.fn();

jest.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush }),
  useParams: () => ({ company: "acme" }),
}));

jest.mock("@/components/providers", () => ({
  useAuth: jest.fn(),
}));

jest.mock("@/hooks/use-eligibility", () => ({
  useEligibility: jest.fn(),
}));

jest.mock("@/hooks/use-advance-requests", () => ({
  useCreateAdvanceRequest: jest.fn(),
}));

const { useAuth } = jest.requireMock("@/components/providers");
const { useEligibility } = jest.requireMock("@/hooks/use-eligibility");
const { useCreateAdvanceRequest } = jest.requireMock("@/hooks/use-advance-requests");

const baseUser = {
  id: "1",
  email: "sarah@acme.com",
  email_verified: true,
  full_name: "Sarah Doe",
  phone_number: "+237600000000",
  phone_verified: true,
  status: "active",
  is_terms_accepted: true,
  terms_accepted_at: null,
  terms_version: null,
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
};

const baseEligibility = {
  eligible: true,
  reasons: [],
  kill_switch_active: false,
  request_window: { start_day: 1, end_day: 25, in_window: true },
  daily_requests_remaining: 1,
  monthly_requests_remaining: 2,
  advance_amount_xaf: "10000",
  phone_verified: true,
  terms_accepted: true,
};

describe("EmployeeHomePage", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    useAuth.mockReturnValue({
      user: baseUser,
      signOut: jest.fn(),
      refreshSubject: jest.fn(),
    });
    useEligibility.mockReturnValue({
      data: baseEligibility,
      isLoading: false,
      isError: false,
      refetch: mockRefetch,
      isRefetching: false,
    });
    useCreateAdvanceRequest.mockReturnValue({
      mutateAsync: mockMutateAsync,
      isPending: false,
    });
  });

  it("renders the header, display name, and eligibility", () => {
    render(<EmployeeHomePage />);
    expect(screen.getByText("Bohikor")).toBeInTheDocument();
    expect(screen.getAllByText("Sarah Doe").length).toBeGreaterThan(0);
    expect(screen.getAllByText("10000 XAF").length).toBeGreaterThan(0);
    expect(screen.getByText("Eligible")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /view transaction history/i })).toHaveAttribute(
      "href",
      "/acme/history"
    );
  });

  it("routes to account?section=terms when terms are not accepted", async () => {
    useAuth.mockReturnValue({
      user: { ...baseUser, is_terms_accepted: false },
      signOut: jest.fn(),
      refreshSubject: jest.fn(),
    });

    const user = userEvent.setup();
    render(<EmployeeHomePage />);

    expect(screen.getByText(/you haven't accepted the terms yet/i)).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /request advance/i }));
    expect(mockPush).toHaveBeenCalledWith("/acme/account?section=terms");
  });

  it("routes to account?section=phone when the profile is incomplete", async () => {
    useAuth.mockReturnValue({
      user: { ...baseUser, phone_verified: false },
      signOut: jest.fn(),
      refreshSubject: jest.fn(),
    });

    const user = userEvent.setup();
    render(<EmployeeHomePage />);

    expect(
      screen.getByText("Verify your phone number to request an advance.")
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /request advance/i }));
    expect(mockPush).toHaveBeenCalledWith("/acme/account?section=phone");
  });

  it("shows the add-phone banner when no phone is set", () => {
    useAuth.mockReturnValue({
      user: { ...baseUser, phone_number: null, phone_verified: false },
      signOut: jest.fn(),
      refreshSubject: jest.fn(),
    });

    render(<EmployeeHomePage />);
    expect(
      screen.getByText("Add a phone number to request an advance.")
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /add phone number/i })).toBeInTheDocument();
  });

  it("opens the confirm modal when the user is ready to request", async () => {
    const user = userEvent.setup();
    render(<EmployeeHomePage />);

    await user.click(screen.getByRole("button", { name: /request advance/i }));
    expect(screen.getByText("Confirm Advance Request")).toBeInTheDocument();
    expect(
      screen.getByText(
        /this amount plus any applicable charges will be deducted from your upcoming salary payment/i
      )
    ).toBeInTheDocument();
  });

  it("confirms the request and navigates to history on success", async () => {
    mockMutateAsync.mockResolvedValue({ id: "r1" });

    const user = userEvent.setup();
    render(<EmployeeHomePage />);

    await user.click(screen.getByRole("button", { name: /request advance/i }));
    await user.click(screen.getByRole("button", { name: "Confirm" }));

    expect(mockMutateAsync).toHaveBeenCalled();
    expect(mockPush).toHaveBeenCalledWith("/acme/history");
  });

  it("keeps the modal open and shows the error when the request fails", async () => {
    mockMutateAsync.mockRejectedValue({
      response: { data: { error: "Company float is insufficient", code: "insufficient_employer_float" } },
    });

    const user = userEvent.setup();
    render(<EmployeeHomePage />);

    await user.click(screen.getByRole("button", { name: /request advance/i }));
    await user.click(screen.getByRole("button", { name: "Confirm" }));

    expect(
      await screen.findByText("Company float is insufficient")
    ).toBeInTheDocument();
    expect(screen.getByText("Confirm Advance Request")).toBeInTheDocument();
    expect(mockPush).not.toHaveBeenCalled();
  });

  it("shows a loading state for eligibility", () => {
    useEligibility.mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      refetch: mockRefetch,
      isRefetching: false,
    });
    render(<EmployeeHomePage />);
    expect(screen.getByText("Loading eligibility...")).toBeInTheDocument();
  });

  it("shows an error state with retry for eligibility", async () => {
    useEligibility.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      refetch: mockRefetch,
      isRefetching: false,
    });

    const user = userEvent.setup();
    render(<EmployeeHomePage />);
    expect(screen.getByText("Failed to load eligibility.")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(mockRefetch).toHaveBeenCalled();
  });

  it("shows the kill-switch banner when active", () => {
    useEligibility.mockReturnValue({
      data: { ...baseEligibility, kill_switch_active: true },
      isLoading: false,
      isError: false,
      refetch: mockRefetch,
      isRefetching: false,
    });
    render(<EmployeeHomePage />);
    expect(screen.getByText("Advances are temporarily disabled")).toBeInTheDocument();
  });

  it("shows not-eligible with reasons when not eligible", () => {
    useEligibility.mockReturnValue({
      data: {
        ...baseEligibility,
        eligible: false,
        reasons: ["Outside request window", "Daily limit reached"],
      },
      isLoading: false,
      isError: false,
      refetch: mockRefetch,
      isRefetching: false,
    });
    render(<EmployeeHomePage />);
    expect(screen.getByText("Not eligible")).toBeInTheDocument();
    expect(screen.getByText("Outside request window")).toBeInTheDocument();
    expect(screen.getByText("Daily limit reached")).toBeInTheDocument();
  });
});
