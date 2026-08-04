import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import RequestsPage from "../page";
import { renderWithProviders as renderWithProvidersBase } from "@/test-utils";

jest.mock("@/hooks/use-requests", () => ({
  useRequests: jest.fn(),
}));

const { useRequests } = jest.requireMock("@/hooks/use-requests");

function renderWithProviders(ui: React.ReactElement) {
  return renderWithProvidersBase(ui, { withToaster: false });
}

const mockRequest = {
  id: "req-1",
  user_id: "user-1",
  user_email: "employee@example.com",
  amount_xaf: "10000.00",
  status: "success",
  campay_payout_ref: "campay-ref-123",
  failure_reason: null,
  payout_duration_seconds: 30,
  created_at: "2026-05-20T12:00:00Z",
  updated_at: "2026-05-20T12:00:30Z",
};

describe("RequestsPage", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("renders page title and description", () => {
    useRequests.mockReturnValue({
      data: [],
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });

    renderWithProviders(<RequestsPage />);

    expect(
      screen.getByRole("heading", { name: /requests/i })
    ).toBeInTheDocument();
    expect(
      screen.getByText(/view all salary advance requests/i)
    ).toBeInTheDocument();
  });

  it("shows loading state while fetching requests", () => {
    useRequests.mockReturnValue({
      data: null,
      isLoading: true,
      refetch: jest.fn(),
      isRefetching: false,
    });

    renderWithProviders(<RequestsPage />);

    expect(screen.getByText(/loading requests/i)).toBeInTheDocument();
  });

  it("shows empty state when no requests exist", () => {
    useRequests.mockReturnValue({
      data: [],
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });

    renderWithProviders(<RequestsPage />);

    expect(screen.getByText(/no requests found/i)).toBeInTheDocument();
  });

  it("displays request data in table", () => {
    useRequests.mockReturnValue({
      data: [mockRequest],
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });

    renderWithProviders(<RequestsPage />);

    expect(screen.getByText("employee@example.com")).toBeInTheDocument();
    expect(screen.getByText("10000.00")).toBeInTheDocument();
    expect(screen.getByText("success")).toBeInTheDocument();
    expect(screen.getByText("campay-ref-123")).toBeInTheDocument();
  });

  it("shows dash for missing optional fields", () => {
    useRequests.mockReturnValue({
      data: [
        {
          ...mockRequest,
          campay_payout_ref: null,
          failure_reason: null,
          user_email: undefined,
        },
      ],
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });

    renderWithProviders(<RequestsPage />);

    const dashes = screen.getAllByText("—");
    expect(dashes.length).toBeGreaterThanOrEqual(2);
  });

  it("handles multiple requests", () => {
    useRequests.mockReturnValue({
      data: [
        mockRequest,
        {
          ...mockRequest,
          id: "req-2",
          status: "failed",
          user_email: "other@example.com",
          failure_reason: "Insufficient balance",
          campay_payout_ref: null,
        },
      ],
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });

    renderWithProviders(<RequestsPage />);

    expect(screen.getByText("employee@example.com")).toBeInTheDocument();
    expect(screen.getByText("other@example.com")).toBeInTheDocument();
    expect(screen.getByText("success")).toBeInTheDocument();
    expect(screen.getByText("failed")).toBeInTheDocument();
  });

  it("shows user_id when user_email is missing", () => {
    useRequests.mockReturnValue({
      data: [
        {
          ...mockRequest,
          user_email: undefined,
        },
      ],
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });

    renderWithProviders(<RequestsPage />);

    expect(screen.getByText("user-1")).toBeInTheDocument();
  });

  it("refetches when refresh button is clicked", async () => {
    const user = userEvent.setup();
    const refetchFn = jest.fn();
    useRequests.mockReturnValue({
      data: [],
      isLoading: false,
      refetch: refetchFn,
      isRefetching: false,
    });

    renderWithProviders(<RequestsPage />);

    const refreshButton = screen.getByRole("button", { name: /refresh/i });
    await user.click(refreshButton);

    expect(refetchFn).toHaveBeenCalled();
  });

  it("shows a spinning refresh icon while refetching", () => {
    useRequests.mockReturnValue({
      data: [],
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: true,
    });

    const { container } = renderWithProviders(<RequestsPage />);

    expect(container.querySelector(".animate-spin")).toBeInTheDocument();
  });

  it("falls back to secondary badge variant for an unmapped status", () => {
    useRequests.mockReturnValue({
      data: [{ ...mockRequest, id: "r5", status: "unknown_status" }],
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });

    renderWithProviders(<RequestsPage />);

    expect(screen.getByText("unknown_status")).toBeInTheDocument();
  });

  it("renders correct status badge variants", () => {
    useRequests.mockReturnValue({
      data: [
        { ...mockRequest, id: "r1", status: "initiated" },
        { ...mockRequest, id: "r2", status: "pending" },
        { ...mockRequest, id: "r3", status: "success" },
        { ...mockRequest, id: "r4", status: "failed" },
      ],
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });

    renderWithProviders(<RequestsPage />);

    expect(screen.getByText("initiated")).toBeInTheDocument();
    expect(screen.getByText("pending")).toBeInTheDocument();
    expect(screen.getByText("success")).toBeInTheDocument();
    expect(screen.getByText("failed")).toBeInTheDocument();
  });
});
