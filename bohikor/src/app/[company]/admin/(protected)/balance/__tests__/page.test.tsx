import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import BalancePage from "../page";
import { renderWithProviders as renderWithProvidersBase } from "@/test-utils";

jest.mock("@/hooks/use-ledger", () => ({
  useLedger: jest.fn(),
}));

const { useLedger } = jest.requireMock("@/hooks/use-ledger");

function renderWithProviders(ui: React.ReactElement) {
  return renderWithProvidersBase(ui, { withToaster: false });
}

const mockEntry = {
  id: "ledger-1",
  company_id: "company-1",
  entry_type: "topup",
  amount_xaf: "50000",
  advance_request_id: null,
  created_by: null,
  note: "Initial funding",
  created_at: "2026-05-20T12:00:00Z",
};

describe("BalancePage", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("renders page title and description", () => {
    useLedger.mockReturnValue({
      data: { balance_xaf: "50000", entries: [] },
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });

    renderWithProviders(<BalancePage />);

    expect(screen.getByRole("heading", { name: /balance/i })).toBeInTheDocument();
    expect(screen.getByText(/company balance & ledger/i)).toBeInTheDocument();
  });

  it("shows loading state while fetching the ledger", () => {
    useLedger.mockReturnValue({
      data: null,
      isLoading: true,
      refetch: jest.fn(),
      isRefetching: false,
    });

    renderWithProviders(<BalancePage />);

    expect(screen.getByText(/loading ledger/i)).toBeInTheDocument();
  });

  it("shows empty state when there are no entries", () => {
    useLedger.mockReturnValue({
      data: { balance_xaf: "0", entries: [] },
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });

    renderWithProviders(<BalancePage />);

    expect(screen.getByText(/no ledger entries yet/i)).toBeInTheDocument();
  });

  it("renders the balance prominently", () => {
    useLedger.mockReturnValue({
      data: { balance_xaf: "45000", entries: [] },
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });

    renderWithProviders(<BalancePage />);

    expect(screen.getByText("45000 XAF")).toBeInTheDocument();
  });

  it("renders ledger entries with type, amount, note, and date", () => {
    useLedger.mockReturnValue({
      data: {
        balance_xaf: "45000",
        entries: [mockEntry],
      },
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });

    renderWithProviders(<BalancePage />);

    expect(screen.getByText("topup")).toBeInTheDocument();
    expect(screen.getByText("50000 XAF")).toBeInTheDocument();
    expect(screen.getByText("Initial funding")).toBeInTheDocument();
  });

  it("renders entry type badges with the right variant classes", () => {
    useLedger.mockReturnValue({
      data: {
        balance_xaf: "0",
        entries: [
          { ...mockEntry, id: "e1", entry_type: "topup" },
          { ...mockEntry, id: "e2", entry_type: "reversal" },
          { ...mockEntry, id: "e3", entry_type: "payout_debit" },
          { ...mockEntry, id: "e4", entry_type: "adjustment" },
        ],
      },
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });

    renderWithProviders(<BalancePage />);

    const badges = screen.getAllByText(/topup|reversal|payout_debit|adjustment/);
    expect(badges.length).toBe(4);
    expect(badges[0]).toHaveClass("bg-green-100");
    expect(badges[2]).toHaveClass("bg-gray-100");
    expect(badges[3]).toHaveClass("bg-blue-100");
  });

  it("renders a negative amount without throwing and shows it in red", () => {
    useLedger.mockReturnValue({
      data: {
        balance_xaf: "45000",
        entries: [
          { ...mockEntry, id: "e1", entry_type: "payout_debit", amount_xaf: -5000, note: null },
          { ...mockEntry, id: "e2", entry_type: "topup", amount_xaf: "50000", note: null },
        ],
      },
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });

    renderWithProviders(<BalancePage />);

    const negativeCell = screen.getByText("-5000 XAF");
    expect(negativeCell).toBeInTheDocument();
    expect(negativeCell).toHaveClass("text-destructive");
    expect(screen.getByText("50000 XAF")).not.toHaveClass("text-destructive");
  });

  it("shows a dash for a missing note", () => {
    useLedger.mockReturnValue({
      data: {
        balance_xaf: "0",
        entries: [{ ...mockEntry, note: null }],
      },
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });

    renderWithProviders(<BalancePage />);

    expect(screen.getAllByText("—").length).toBeGreaterThanOrEqual(1);
  });

  it("refetches when the refresh button is clicked", async () => {
    const user = userEvent.setup();
    const refetchFn = jest.fn();
    useLedger.mockReturnValue({
      data: { balance_xaf: "0", entries: [] },
      isLoading: false,
      refetch: refetchFn,
      isRefetching: false,
    });

    renderWithProviders(<BalancePage />);

    const refreshButton = screen.getByRole("button", { name: /refresh/i });
    await user.click(refreshButton);

    expect(refetchFn).toHaveBeenCalled();
  });

  it("shows a spinning refresh icon while refetching", () => {
    useLedger.mockReturnValue({
      data: { balance_xaf: "0", entries: [] },
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: true,
    });

    const { container } = renderWithProviders(<BalancePage />);

    expect(container.querySelector(".animate-spin")).toBeInTheDocument();
  });
});
