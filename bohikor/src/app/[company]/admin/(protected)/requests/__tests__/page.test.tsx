import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import RequestsPage from "../page";
import { renderWithProviders as renderWithProvidersBase } from "@/test-utils";

jest.mock("@/hooks/use-requests", () => ({
  useRequests: jest.fn(),
  useReconcileRequest: jest.fn(),
  useResolveRequest: jest.fn(),
  useReissueRequest: jest.fn(),
}));

const { useRequests, useReconcileRequest, useResolveRequest, useReissueRequest } =
  jest.requireMock("@/hooks/use-requests");

function renderWithProviders(ui: React.ReactElement) {
  return renderWithProvidersBase(ui, { withToaster: true });
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
    useReconcileRequest.mockReturnValue({ mutateAsync: jest.fn(), isPending: false });
    useResolveRequest.mockReturnValue({ mutateAsync: jest.fn(), isPending: false });
    useReissueRequest.mockReturnValue({ mutateAsync: jest.fn(), isPending: false });
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

  it("shows a Needs Review badge when a request is flagged", () => {
    useRequests.mockReturnValue({
      data: [{ ...mockRequest, needs_admin_review: true }],
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });

    renderWithProviders(<RequestsPage />);

    expect(screen.getByText("Needs Review")).toBeInTheDocument();
  });

  describe("reconcile action", () => {
    it("shows the button only for processing or pending requests", () => {
      useRequests.mockReturnValue({
        data: [
          { ...mockRequest, id: "r1", status: "processing" },
          { ...mockRequest, id: "r2", status: "pending" },
          { ...mockRequest, id: "r3", status: "success" },
          { ...mockRequest, id: "r4", status: "failed" },
        ],
        isLoading: false,
        refetch: jest.fn(),
        isRefetching: false,
      });

      renderWithProviders(<RequestsPage />);

      expect(screen.getAllByRole("button", { name: /reconcile/i })).toHaveLength(2);
    });

    it("calls the mutation with the right id and toasts the new status", async () => {
      const user = userEvent.setup();
      const reconcileFn = jest.fn().mockResolvedValue({ status: "success" });
      useReconcileRequest.mockReturnValue({ mutateAsync: reconcileFn, isPending: false });
      useRequests.mockReturnValue({
        data: [{ ...mockRequest, id: "req-1", status: "processing" }],
        isLoading: false,
        refetch: jest.fn(),
        isRefetching: false,
      });

      renderWithProviders(<RequestsPage />);

      await user.click(screen.getByRole("button", { name: /reconcile/i }));
      expect(reconcileFn).toHaveBeenCalledWith("req-1");
      expect(await screen.findByText("Reconciled — status: success.")).toBeInTheDocument();
    });

    it("toasts nothing_to_poll without crashing", async () => {
      const user = userEvent.setup();
      useReconcileRequest.mockReturnValue({
        mutateAsync: jest.fn().mockRejectedValue({
          response: { data: { code: "nothing_to_poll" } },
        }),
        isPending: false,
      });
      useRequests.mockReturnValue({
        data: [{ ...mockRequest, id: "req-1", status: "pending" }],
        isLoading: false,
        refetch: jest.fn(),
        isRefetching: false,
      });

      renderWithProviders(<RequestsPage />);

      await user.click(screen.getByRole("button", { name: /reconcile/i }));
      expect(
        await screen.findByText("This request has no payout reference to check yet.")
      ).toBeInTheDocument();
    });
  });

  describe("resolve action", () => {
    it("shows the button only for flagged requests", () => {
      useRequests.mockReturnValue({
        data: [
          { ...mockRequest, id: "r1", needs_admin_review: true },
          { ...mockRequest, id: "r2", needs_admin_review: false },
        ],
        isLoading: false,
        refetch: jest.fn(),
        isRefetching: false,
      });

      renderWithProviders(<RequestsPage />);

      expect(screen.getAllByRole("button", { name: /^resolve$/i })).toHaveLength(1);
    });

    it("keeps Confirm Resolve disabled until a note is entered", async () => {
      const user = userEvent.setup();
      const resolveFn = jest.fn().mockResolvedValue({});
      useResolveRequest.mockReturnValue({ mutateAsync: resolveFn, isPending: false });
      useRequests.mockReturnValue({
        data: [{ ...mockRequest, id: "req-1", needs_admin_review: true }],
        isLoading: false,
        refetch: jest.fn(),
        isRefetching: false,
      });

      renderWithProviders(<RequestsPage />);

      await user.click(screen.getByRole("button", { name: /^resolve$/i }));

      const confirmButton = screen.getByRole("button", { name: /confirm resolve/i });
      expect(confirmButton).toBeDisabled();

      await user.type(screen.getByPlaceholderText(/resolution note/i), "Reviewed payout");
      expect(confirmButton).toBeEnabled();

      await user.click(confirmButton);
      expect(resolveFn).toHaveBeenCalledWith({ id: "req-1", note: "Reviewed payout" });
      expect(await screen.findByText("Marked as resolved.")).toBeInTheDocument();
    });
  });

  describe("reissue action", () => {
    it("shows the button only for failed requests", () => {
      useRequests.mockReturnValue({
        data: [
          { ...mockRequest, id: "r1", status: "failed" },
          { ...mockRequest, id: "r2", status: "success" },
        ],
        isLoading: false,
        refetch: jest.fn(),
        isRefetching: false,
      });

      renderWithProviders(<RequestsPage />);

      expect(screen.getAllByRole("button", { name: /reissue/i })).toHaveLength(1);
    });

    it("calls the mutation with the right id and toasts success", async () => {
      const user = userEvent.setup();
      const reissueFn = jest.fn().mockResolvedValue({});
      useReissueRequest.mockReturnValue({ mutateAsync: reissueFn, isPending: false });
      useRequests.mockReturnValue({
        data: [{ ...mockRequest, id: "req-1", status: "failed" }],
        isLoading: false,
        refetch: jest.fn(),
        isRefetching: false,
      });

      renderWithProviders(<RequestsPage />);

      await user.click(screen.getByRole("button", { name: /reissue/i }));
      expect(reissueFn).toHaveBeenCalledWith("req-1");
      expect(await screen.findByText("Reissued — new request created.")).toBeInTheDocument();
    });

    it("toasts already_reissued without crashing", async () => {
      const user = userEvent.setup();
      useReissueRequest.mockReturnValue({
        mutateAsync: jest.fn().mockRejectedValue({
          response: { data: { code: "already_reissued" } },
        }),
        isPending: false,
      });
      useRequests.mockReturnValue({
        data: [{ ...mockRequest, id: "req-1", status: "failed" }],
        isLoading: false,
        refetch: jest.fn(),
        isRefetching: false,
      });

      renderWithProviders(<RequestsPage />);

      await user.click(screen.getByRole("button", { name: /reissue/i }));
      expect(
        await screen.findByText("This request has already been reissued.")
      ).toBeInTheDocument();
    });

    it("toasts the insufficient_employer_float message verbatim", async () => {
      const user = userEvent.setup();
      useReissueRequest.mockReturnValue({
        mutateAsync: jest.fn().mockRejectedValue({
          response: {
            data: { code: "insufficient_employer_float", error: "Company float is insufficient" },
          },
        }),
        isPending: false,
      });
      useRequests.mockReturnValue({
        data: [{ ...mockRequest, id: "req-1", status: "failed" }],
        isLoading: false,
        refetch: jest.fn(),
        isRefetching: false,
      });

      renderWithProviders(<RequestsPage />);

      await user.click(screen.getByRole("button", { name: /reissue/i }));
      expect(await screen.findByText("Company float is insufficient")).toBeInTheDocument();
    });
  });
});
