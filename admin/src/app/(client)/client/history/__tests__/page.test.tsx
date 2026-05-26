import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import HistoryPage from "../page";

const mockBack = jest.fn();

jest.mock("next/navigation", () => ({
  useRouter: () => ({ back: mockBack }),
}));

jest.mock("@/hooks/use-advance", () => ({
  useAdvanceRequests: jest.fn(),
}));

const { useAdvanceRequests } = jest.requireMock("@/hooks/use-advance");

function renderWithProviders(ui: React.ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  return render(
    <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>
  );
}

const mockRequest = {
  id: "req-1",
  user_id: "user-1",
  amount_xaf: "10000.00",
  status: "success",
  campay_payout_ref: "campay-ref-123",
  failure_reason: null,
  payout_duration_seconds: 30,
  created_at: "2026-05-20T12:00:00Z",
  updated_at: "2026-05-20T12:00:30Z",
};

describe("HistoryPage", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("renders page title", () => {
    useAdvanceRequests.mockReturnValue({
      data: [],
      isLoading: false,
      isError: false,
      refetch: jest.fn(),
      isRefetching: false,
    });

    renderWithProviders(<HistoryPage />);
    expect(screen.getByText("History")).toBeInTheDocument();
  });

  it("shows loading state", () => {
    useAdvanceRequests.mockReturnValue({
      data: null,
      isLoading: true,
      isError: false,
      refetch: jest.fn(),
      isRefetching: false,
    });

    renderWithProviders(<HistoryPage />);
    expect(screen.getByTestId("history-loading")).toBeInTheDocument();
  });

  it("shows empty state when no requests exist", () => {
    useAdvanceRequests.mockReturnValue({
      data: [],
      isLoading: false,
      isError: false,
      refetch: jest.fn(),
      isRefetching: false,
    });

    renderWithProviders(<HistoryPage />);
    expect(
      screen.getByText("No advance requests yet.")
    ).toBeInTheDocument();
  });

  it("shows error state", () => {
    useAdvanceRequests.mockReturnValue({
      data: null,
      isLoading: false,
      isError: true,
      refetch: jest.fn(),
      isRefetching: false,
    });

    renderWithProviders(<HistoryPage />);
    expect(screen.getByText("Failed to load history")).toBeInTheDocument();
    expect(screen.getByText("Retry")).toBeInTheDocument();
  });

  it("displays request details", () => {
    useAdvanceRequests.mockReturnValue({
      data: [mockRequest],
      isLoading: false,
      isError: false,
      refetch: jest.fn(),
      isRefetching: false,
    });

    renderWithProviders(<HistoryPage />);
    expect(screen.getByText("10000.00 XAF")).toBeInTheDocument();
    expect(screen.getByText("success")).toBeInTheDocument();
    expect(screen.getByText("Ref: campay-ref-123")).toBeInTheDocument();
  });

  it("shows failure reason when present", () => {
    useAdvanceRequests.mockReturnValue({
      data: [
        {
          ...mockRequest,
          status: "failed",
          failure_reason: "Insufficient balance",
          campay_payout_ref: null,
        },
      ],
      isLoading: false,
      isError: false,
      refetch: jest.fn(),
      isRefetching: false,
    });

    renderWithProviders(<HistoryPage />);
    expect(screen.getByText("Insufficient balance")).toBeInTheDocument();
  });

  it("handles multiple requests", () => {
    useAdvanceRequests.mockReturnValue({
      data: [
        mockRequest,
        {
          ...mockRequest,
          id: "req-2",
          status: "pending",
          created_at: "2026-05-21T12:00:00Z",
        },
      ],
      isLoading: false,
      isError: false,
      refetch: jest.fn(),
      isRefetching: false,
    });

    renderWithProviders(<HistoryPage />);
    const amounts = screen.getAllByText("10000.00 XAF");
    expect(amounts).toHaveLength(2);
    expect(screen.getByText("success")).toBeInTheDocument();
    expect(screen.getByText("pending")).toBeInTheDocument();
  });

  it("refetches when refresh button is clicked", async () => {
    const user = userEvent.setup();
    const refetchFn = jest.fn();
    useAdvanceRequests.mockReturnValue({
      data: [],
      isLoading: false,
      isError: false,
      refetch: refetchFn,
      isRefetching: false,
    });

    renderWithProviders(<HistoryPage />);
    const refreshButton = screen.getByTestId("refresh-button");
    await user.click(refreshButton);

    expect(refetchFn).toHaveBeenCalled();
  });
});
