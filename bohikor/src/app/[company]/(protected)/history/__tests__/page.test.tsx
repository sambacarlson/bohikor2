import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import HistoryPage from "../page";

const mockBack = jest.fn();
const mockRefetch = jest.fn();
const mockMutateAsync = jest.fn();

jest.mock("next/navigation", () => ({
  useRouter: () => ({ back: mockBack, push: jest.fn() }),
  useParams: () => ({ company: "acme" }),
}));

jest.mock("@/hooks/use-advance-requests", () => ({
  useMyAdvanceRequests: jest.fn(),
  useRetryAdvanceRequest: jest.fn(),
}));

jest.mock("sonner", () => ({
  toast: { success: jest.fn(), error: jest.fn() },
}));

const { useMyAdvanceRequests } = jest.requireMock("@/hooks/use-advance-requests");
const { useRetryAdvanceRequest } = jest.requireMock("@/hooks/use-advance-requests");
const { toast } = jest.requireMock("sonner");

const baseRequest = {
  id: "r1",
  user_id: "u1",
  amount_xaf: "5000",
  status: "pending",
  campay_payout_ref: "PAY-123",
  failure_reason: null,
  payout_duration_seconds: null,
  created_at: "2026-07-01T10:30:00Z",
  updated_at: "2026-07-01T10:30:00Z",
};

function mockList(requests: unknown[]) {
  useMyAdvanceRequests.mockReturnValue({
    data: requests,
    isLoading: false,
    isError: false,
    refetch: mockRefetch,
    isRefetching: false,
  });
  useRetryAdvanceRequest.mockReturnValue({ mutateAsync: mockMutateAsync, isPending: false });
}

describe("HistoryPage", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockList([]);
  });

  it("shows a loading state", () => {
    useMyAdvanceRequests.mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      refetch: mockRefetch,
      isRefetching: false,
    });
    render(<HistoryPage />);
    expect(screen.getByText("Loading history...")).toBeInTheDocument();
  });

  it("shows an error state with retry", async () => {
    useMyAdvanceRequests.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      refetch: mockRefetch,
      isRefetching: false,
    });
    const user = userEvent.setup();
    render(<HistoryPage />);
    expect(screen.getByText("Failed to load history")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(mockRefetch).toHaveBeenCalled();
  });

  it("shows an empty state", () => {
    render(<HistoryPage />);
    expect(screen.getByText("No advance requests yet.")).toBeInTheDocument();
  });

  it("renders requests with amounts, refs, and formatted dates", () => {
    mockList([
      { ...baseRequest, amount_xaf: "10000", status: "success", campay_payout_ref: "PAY-1" },
      { ...baseRequest, id: "r2", amount_xaf: "7500", status: "processing", campay_payout_ref: null },
    ]);
    render(<HistoryPage />);

    expect(screen.getByText("10000 XAF")).toBeInTheDocument();
    expect(screen.getByText("7500 XAF")).toBeInTheDocument();
    expect(screen.getByText(/Ref: PAY-1/)).toBeInTheDocument();
    expect(screen.getByText("success")).toBeInTheDocument();
    expect(screen.getByText("processing")).toBeInTheDocument();
  });

  it("shows failure reason in red text", () => {
    mockList([
      { ...baseRequest, status: "failed", failure_reason: "Insufficient funds" },
    ]);
    render(<HistoryPage />);
    expect(screen.getByText("Insufficient funds")).toBeInTheDocument();
  });

  it("shows a Retry button only for failed rows and retries with the right id", async () => {
    mockMutateAsync.mockResolvedValue({ id: "r1" });
    mockList([
      { ...baseRequest, id: "r1", status: "failed", failure_reason: "Declined" },
      { ...baseRequest, id: "r2", amount_xaf: "3000", status: "success" },
    ]);

    const user = userEvent.setup();
    render(<HistoryPage />);

    const retryButtons = screen.getAllByRole("button", { name: "Retry" });
    expect(retryButtons).toHaveLength(1);

    await user.click(retryButtons[0]);
    expect(mockMutateAsync).toHaveBeenCalledWith("r1");
    await waitFor(() => expect(toast.success).toHaveBeenCalled());
  });

  it("shows a toast error when retry fails", async () => {
    mockMutateAsync.mockRejectedValue({
      response: { data: { error: "Something went wrong" } },
    });
    mockList([{ ...baseRequest, id: "r1", status: "failed" }]);

    const user = userEvent.setup();
    render(<HistoryPage />);

    await user.click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith("Something went wrong")
    );
  });

  it("refreshes when the refresh button is clicked", async () => {
    const user = userEvent.setup();
    render(<HistoryPage />);

    await user.click(screen.getByRole("button", { name: /refresh/i }));
    expect(mockRefetch).toHaveBeenCalled();
  });
});
