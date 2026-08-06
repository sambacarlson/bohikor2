import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import PlatformConsolePage from "../page";
import { renderWithProviders } from "@/test-utils";

jest.mock("@/hooks/use-companies", () => ({
  useCompanies: jest.fn(),
  useCreateCompany: jest.fn(),
  useCompany: jest.fn(),
  useUpdateCompanyStatus: jest.fn(),
  useTopUpCompany: jest.fn(),
  useAdjustCompanyLedger: jest.fn(),
  useCreateCompanyAdmin: jest.fn(),
}));

jest.mock("@/hooks/use-platform-requests", () => ({
  useRequestsHealth: jest.fn(),
  useRequestsNeedingReview: jest.fn(),
}));

const {
  useCompanies,
  useCreateCompany,
  useCompany,
  useUpdateCompanyStatus,
  useTopUpCompany,
  useAdjustCompanyLedger,
  useCreateCompanyAdmin,
} = jest.requireMock("@/hooks/use-companies");

const { useRequestsHealth, useRequestsNeedingReview } =
  jest.requireMock("@/hooks/use-platform-requests");

const mockCompany = {
  id: "company-1",
  slug: "acme",
  name: "Acme Corp",
  status: "active",
  balance_xaf: "50000",
  created_at: "2026-05-20T12:00:00Z",
  updated_at: "2026-05-20T12:00:00Z",
};

describe("PlatformConsolePage", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    useCompanies.mockReturnValue({
      data: [],
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });
    useCreateCompany.mockReturnValue({ mutateAsync: jest.fn(), isPending: false });
    useCompany.mockReturnValue({ data: undefined, isLoading: true });
    useUpdateCompanyStatus.mockReturnValue({ mutateAsync: jest.fn(), isPending: false });
    useTopUpCompany.mockReturnValue({ mutateAsync: jest.fn(), isPending: false });
    useAdjustCompanyLedger.mockReturnValue({ mutateAsync: jest.fn(), isPending: false });
    useCreateCompanyAdmin.mockReturnValue({ mutateAsync: jest.fn(), isPending: false });
    useRequestsHealth.mockReturnValue({ data: [], isLoading: false });
    useRequestsNeedingReview.mockReturnValue({ data: [], isLoading: false });
  });

  it("renders the page title and description", () => {
    renderWithProviders(<PlatformConsolePage />);

    expect(screen.getByRole("heading", { name: /platform console/i })).toBeInTheDocument();
  });

  it("shows loading state", () => {
    useCompanies.mockReturnValue({
      data: null,
      isLoading: true,
      refetch: jest.fn(),
      isRefetching: false,
    });

    renderWithProviders(<PlatformConsolePage />);

    expect(screen.getByText(/loading companies/i)).toBeInTheDocument();
  });

  it("shows an empty state", () => {
    renderWithProviders(<PlatformConsolePage />);

    expect(screen.getByText("No companies yet.")).toBeInTheDocument();
  });

  it("renders companies with status badges", () => {
    useCompanies.mockReturnValue({
      data: [
        mockCompany,
        { ...mockCompany, id: "c2", slug: "globex", name: "Globex", status: "suspended" },
      ],
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });

    renderWithProviders(<PlatformConsolePage />);

    expect(screen.getByText("Acme Corp")).toBeInTheDocument();
    expect(screen.getByText("Globex")).toBeInTheDocument();
    expect(screen.getAllByText("50000 XAF").length).toBeGreaterThan(0);
    expect(screen.getAllByText("active").length).toBe(1);
    expect(screen.getByText("suspended")).toBeInTheDocument();
  });

  it("validates the slug format client-side before submit", async () => {
    const user = userEvent.setup();
    const createFn = jest.fn();
    useCreateCompany.mockReturnValue({ mutateAsync: createFn, isPending: false });

    renderWithProviders(<PlatformConsolePage />);

    await user.click(screen.getByRole("button", { name: /create company/i }));
    await user.type(screen.getByLabelText("Name"), "Acme Corp");
    await user.type(screen.getByLabelText("Slug"), "Acme Corp!");
    await user.click(screen.getByRole("button", { name: /^create$/i }));

    expect(
      screen.getByText("Slug must be lowercase letters, numbers, and hyphens only.")
    ).toBeInTheDocument();
    expect(createFn).not.toHaveBeenCalled();
  });

  it("submits the mutation with the right payload", async () => {
    const user = userEvent.setup();
    const createFn = jest.fn().mockResolvedValue({});
    useCreateCompany.mockReturnValue({ mutateAsync: createFn, isPending: false });

    renderWithProviders(<PlatformConsolePage />);

    await user.click(screen.getByRole("button", { name: /create company/i }));
    await user.type(screen.getByLabelText("Name"), "Acme Corp");
    await user.type(screen.getByLabelText("Slug"), "acme-corp");
    await user.click(screen.getByRole("button", { name: /^create$/i }));

    expect(createFn).toHaveBeenCalledWith({ slug: "acme-corp", name: "Acme Corp" });
  });

  it("shows an inline error for a duplicate slug", async () => {
    const user = userEvent.setup();
    useCreateCompany.mockReturnValue({
      mutateAsync: jest.fn().mockRejectedValue({
        response: { data: { code: "create_failed" } },
      }),
      isPending: false,
    });

    renderWithProviders(<PlatformConsolePage />);

    await user.click(screen.getByRole("button", { name: /create company/i }));
    await user.type(screen.getByLabelText("Name"), "Acme Corp");
    await user.type(screen.getByLabelText("Slug"), "acme");
    await user.click(screen.getByRole("button", { name: /^create$/i }));

    expect(await screen.findByText("That slug is already taken.")).toBeInTheDocument();
  });

  it("gives the Create Company dialog an accessible dialog role", async () => {
    const user = userEvent.setup();
    renderWithProviders(<PlatformConsolePage />);

    await user.click(screen.getByRole("button", { name: /create company/i }));
    expect(screen.getByRole("dialog")).toHaveAccessibleName("Create Company");
  });

  it("refetches when the refresh button is clicked", async () => {
    const user = userEvent.setup();
    const refetchFn = jest.fn();
    useCompanies.mockReturnValue({
      data: [],
      isLoading: false,
      refetch: refetchFn,
      isRefetching: false,
    });

    renderWithProviders(<PlatformConsolePage />);

    await user.click(screen.getByRole("button", { name: /refresh/i }));
    expect(refetchFn).toHaveBeenCalled();
  });

  describe("company detail panel", () => {
    beforeEach(() => {
      useCompanies.mockReturnValue({
        data: [mockCompany],
        isLoading: false,
        refetch: jest.fn(),
        isRefetching: false,
      });
      useCompany.mockReturnValue({ data: mockCompany, isLoading: false });
    });

    async function openDetail(user: ReturnType<typeof userEvent.setup>) {
      renderWithProviders(<PlatformConsolePage />);
      await user.click(screen.getByRole("button", { name: /manage/i }));
      expect(
        await screen.findByRole("heading", { name: /top up/i })
      ).toBeInTheDocument();
    }

    it("shows balance, slug, and status in the detail panel", async () => {
      const user = userEvent.setup();
      await openDetail(user);
      expect(screen.getAllByText("Acme Corp").length).toBeGreaterThan(0);
      expect(screen.getAllByText("acme").length).toBeGreaterThan(0);
      expect(screen.getAllByText("active").length).toBeGreaterThan(0);
    });

    it("gives the loaded Manage modal an accessible dialog role", async () => {
      const user = userEvent.setup();
      await openDetail(user);
      expect(screen.getByRole("dialog")).toHaveAccessibleName("Acme Corp");
    });

    it("gives the modal an accessible title even while company data is still loading", async () => {
      useCompany.mockReturnValue({ data: undefined, isLoading: true });
      const user = userEvent.setup();

      renderWithProviders(<PlatformConsolePage />);
      await user.click(screen.getByRole("button", { name: /manage/i }));

      expect(await screen.findByText("Loading company...")).toBeInTheDocument();
      expect(screen.getByRole("dialog")).toHaveAccessibleName("Loading company");
    });

    it("requires two clicks to suspend a company", async () => {
      const user = userEvent.setup();
      const updateFn = jest.fn().mockResolvedValue({});
      useUpdateCompanyStatus.mockReturnValue({ mutateAsync: updateFn, isPending: false });

      await openDetail(user);

      await user.click(screen.getByRole("button", { name: /^suspend$/i }));
      expect(updateFn).not.toHaveBeenCalled();
      expect(screen.getByRole("button", { name: /confirm suspend/i })).toBeInTheDocument();

      await user.click(screen.getByRole("button", { name: /confirm suspend/i }));
      expect(updateFn).toHaveBeenCalledWith({ id: "company-1", status: "suspended" });
    });

    it("re-arms the two-step suspend confirmation on reopen instead of carrying it over", async () => {
      const user = userEvent.setup();
      const updateFn = jest.fn().mockResolvedValue({});
      useUpdateCompanyStatus.mockReturnValue({ mutateAsync: updateFn, isPending: false });

      await openDetail(user);
      await user.click(screen.getByRole("button", { name: /^suspend$/i }));
      expect(screen.getByRole("button", { name: /confirm suspend/i })).toBeInTheDocument();

      // Close without confirming, then reopen the *same* company.
      await user.click(screen.getByRole("button", { name: "Close" }));
      await user.click(screen.getByRole("button", { name: /manage/i }));
      await screen.findByRole("heading", { name: /top up/i });

      // The armed "Confirm Suspend?" must not have survived the close/reopen —
      // otherwise a single click here would suspend the company immediately.
      expect(screen.getByRole("button", { name: /^suspend$/i })).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: /confirm suspend/i })).not.toBeInTheDocument();

      await user.click(screen.getByRole("button", { name: /^suspend$/i }));
      expect(updateFn).not.toHaveBeenCalled();
    });

    it("activates a suspended company with one click", async () => {
      const user = userEvent.setup();
      const updateFn = jest.fn().mockResolvedValue({});
      useUpdateCompanyStatus.mockReturnValue({ mutateAsync: updateFn, isPending: false });
      useCompany.mockReturnValue({
        data: { ...mockCompany, status: "suspended" },
        isLoading: false,
      });

      await openDetail(user);

      await user.click(screen.getByRole("button", { name: /^activate$/i }));
      expect(updateFn).toHaveBeenCalledWith({ id: "company-1", status: "active" });
    });

    it("blocks a non-positive top-up and submits a valid one", async () => {
      const user = userEvent.setup();
      const topUpFn = jest.fn().mockResolvedValue({});
      useTopUpCompany.mockReturnValue({ mutateAsync: topUpFn, isPending: false });

      await openDetail(user);

      const amountInputs = screen.getAllByLabelText("Amount (XAF)");
      const topUpInput = amountInputs[0];

      await user.type(topUpInput, "0");
      await user.click(screen.getByRole("button", { name: /^top up$/i }));
      expect(screen.getByText("Amount must be a positive number.")).toBeInTheDocument();
      expect(topUpFn).not.toHaveBeenCalled();

      await user.clear(topUpInput);
      await user.type(topUpInput, "50000");
      await user.click(screen.getByRole("button", { name: /^top up$/i }));
      expect(topUpFn).toHaveBeenCalledWith({
        companyId: "company-1",
        amount_xaf: "50000",
        note: undefined,
      });
    });

    it("requires a non-zero amount and a note for adjustments", async () => {
      const user = userEvent.setup();
      const adjustFn = jest.fn().mockResolvedValue({});
      useAdjustCompanyLedger.mockReturnValue({ mutateAsync: adjustFn, isPending: false });

      await openDetail(user);

      const adjustInput = screen.getAllByLabelText("Amount (XAF)")[1];

      await user.type(adjustInput, "0");
      await user.click(screen.getByRole("button", { name: /apply adjustment/i }));
      expect(screen.getByText("Amount must be non-zero.")).toBeInTheDocument();
      expect(adjustFn).not.toHaveBeenCalled();

      await user.clear(adjustInput);
      await user.type(adjustInput, "-5000");
      await user.click(screen.getByRole("button", { name: /apply adjustment/i }));
      expect(screen.getByText("A note is required for adjustments.")).toBeInTheDocument();
      expect(adjustFn).not.toHaveBeenCalled();

      await user.type(screen.getByLabelText("Note (required)"), "Correction");
      await user.click(screen.getByRole("button", { name: /apply adjustment/i }));
      expect(adjustFn).toHaveBeenCalledWith({
        companyId: "company-1",
        amount_xaf: "-5000",
        note: "Correction",
      });
    });

    it("shows the created admin email on success", async () => {
      const user = userEvent.setup();
      useCreateCompanyAdmin.mockReturnValue({
        mutateAsync: jest.fn().mockResolvedValue({ email: "admin@acme.com" }),
        isPending: false,
      });

      await openDetail(user);

      await user.type(screen.getByLabelText("Email"), "admin@acme.com");
      await user.type(screen.getByLabelText("Password"), "hunter2");
      await user.click(screen.getByRole("button", { name: /create admin/i }));

      expect(
        await screen.findByText(
          "Admin admin@acme.com created — share the password with them securely."
        )
      ).toBeInTheDocument();
    });

    it("shows an inline error for a duplicate admin email", async () => {
      const user = userEvent.setup();
      useCreateCompanyAdmin.mockReturnValue({
        mutateAsync: jest.fn().mockRejectedValue({
          response: { data: { code: "create_failed" } },
        }),
        isPending: false,
      });

      await openDetail(user);

      await user.type(screen.getByLabelText("Email"), "admin@acme.com");
      await user.type(screen.getByLabelText("Password"), "hunter2");
      await user.click(screen.getByRole("button", { name: /create admin/i }));

      expect(
        await screen.findByText("That email already has an admin account.")
      ).toBeInTheDocument();
    });

    it("resets draft form state when Manage is reopened for a different company", async () => {
      const mockCompany2 = { ...mockCompany, id: "company-2", slug: "globex", name: "Globex" };
      useCompanies.mockReturnValue({
        data: [mockCompany, mockCompany2],
        isLoading: false,
        refetch: jest.fn(),
        isRefetching: false,
      });
      useCompany.mockImplementation((id: string) => ({
        data: id === "company-2" ? mockCompany2 : mockCompany,
        isLoading: false,
      }));

      const user = userEvent.setup();
      renderWithProviders(<PlatformConsolePage />);

      const manageButtons = screen.getAllByRole("button", { name: /manage/i });
      await user.click(manageButtons[0]);
      await screen.findByRole("heading", { name: /top up/i });

      const topUpInput = screen.getAllByLabelText("Amount (XAF)")[0];
      await user.type(topUpInput, "12345");
      expect(topUpInput).toHaveValue("12345");

      await user.click(screen.getByRole("button", { name: "Close" }));
      await user.click(manageButtons[1]);

      expect(await screen.findByRole("heading", { name: "Globex" })).toBeInTheDocument();
      expect(screen.getAllByLabelText("Amount (XAF)")[0]).toHaveValue("");
    });
  });

  describe("reconciliation health", () => {
    it("shows a positive empty state", () => {
      renderWithProviders(<PlatformConsolePage />);
      expect(
        screen.getByText("All companies healthy — no requests need review.")
      ).toBeInTheDocument();
    });

    it("renders the health table and highlights non-zero needs-review counts", () => {
      useRequestsHealth.mockReturnValue({
        data: [
          {
            company_id: "c1",
            company_slug: "acme",
            company_name: "Acme Corp",
            processing_count: 2,
            pending_count: 1,
            needs_review_count: 3,
          },
          {
            company_id: "c2",
            company_slug: "globex",
            company_name: "Globex",
            processing_count: 0,
            pending_count: 0,
            needs_review_count: 0,
          },
        ],
        isLoading: false,
      });

      renderWithProviders(<PlatformConsolePage />);

      expect(screen.getByText("Acme Corp (acme)")).toBeInTheDocument();
      expect(screen.getByText("Globex (globex)")).toBeInTheDocument();
      expect(screen.getByText("2")).toBeInTheDocument();
      expect(screen.getByText("1")).toBeInTheDocument();
      expect(screen.getAllByText("3").length).toBeGreaterThan(0);
    });
  });

  describe("needs-review queue", () => {
    it("shows a positive empty state", () => {
      renderWithProviders(<PlatformConsolePage />);
      expect(screen.getByText("Nothing flagged for review.")).toBeInTheDocument();
    });

    it("renders flagged requests with a jump link to the company admin page", () => {
      useRequestsNeedingReview.mockReturnValue({
        data: [
          {
            id: "r1",
            user_id: "u1",
            user_email: "employee@acme.com",
            amount_xaf: "10000",
            status: "pending",
            created_at: "2026-05-20T12:00:00Z",
            updated_at: "2026-05-20T12:00:00Z",
            company_slug: "acme",
            company_name: "Acme Corp",
          },
        ],
        isLoading: false,
      });

      renderWithProviders(<PlatformConsolePage />);

      expect(screen.getByText("Acme Corp (acme)")).toBeInTheDocument();
      expect(screen.getByText("employee@acme.com")).toBeInTheDocument();
      expect(screen.getByText("10000 XAF")).toBeInTheDocument();
      expect(screen.getByRole("link", { name: /open company requests/i })).toHaveAttribute(
        "href",
        "/acme/admin/requests"
      );
    });
  });
});
