import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import AccountPage from "../page";

const mockPush = jest.fn();
const mockRefreshSubject = jest.fn();
const mockAddPhone = jest.fn();
const mockChangePin = jest.fn();
const mockAcceptTerms = jest.fn();
const mockUseSearchParams = jest.fn(() => new URLSearchParams());

jest.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, replace: jest.fn() }),
  useParams: () => ({ company: "acme" }),
  useSearchParams: () => mockUseSearchParams(),
}));

jest.mock("@/components/providers", () => ({
  useAuth: jest.fn(),
}));

jest.mock("@/hooks/use-user", () => ({
  useAcceptTerms: jest.fn(),
  useAddPhone: jest.fn(),
  useChangePin: jest.fn(),
  usePhoneVerificationStatus: jest.fn(),
}));

jest.mock("sonner", () => ({
  toast: { success: jest.fn(), error: jest.fn() },
}));

jest.mock("@/hooks/use-media-query", () => ({
  useMediaQuery: jest.fn(),
}));

const { useAuth } = jest.requireMock("@/components/providers");
const { useAcceptTerms, useAddPhone, useChangePin, usePhoneVerificationStatus } =
  jest.requireMock("@/hooks/use-user");
const { toast } = jest.requireMock("sonner");
const { useMediaQuery } = jest.requireMock("@/hooks/use-media-query");

const baseUser = {
  id: "1",
  email: "sarah@acme.com",
  email_verified: true,
  full_name: "Sarah Doe",
  phone_number: null,
  phone_verified: false,
  status: "active",
  is_terms_accepted: false,
  terms_accepted_at: null,
  terms_version: null,
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
};

const baseVerif = {
  phone_number: null,
  phone_verified: false,
  verification: null,
};

function mockHooks({ user = baseUser, verif = baseVerif } = {}) {
  useAuth.mockReturnValue({
    user,
    signOut: jest.fn(),
    refreshSubject: mockRefreshSubject,
  });
  usePhoneVerificationStatus.mockReturnValue({ data: verif, isLoading: false, isError: false });
  useAddPhone.mockReturnValue({ mutateAsync: mockAddPhone, isPending: false });
  useChangePin.mockReturnValue({ mutateAsync: mockChangePin, isPending: false });
  useAcceptTerms.mockReturnValue({
    mutateAsync: mockAcceptTerms,
    isPending: false,
    isError: false,
  });
}

describe("AccountPage", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockUseSearchParams.mockReturnValue(new URLSearchParams());
    useMediaQuery.mockReturnValue(false);
    mockHooks();
  });

  it("renders all three sections regardless of the section param", () => {
    mockUseSearchParams.mockReturnValue(new URLSearchParams({ section: "terms" }));
    render(<AccountPage />);
    expect(screen.getByText("Phone Number")).toBeInTheDocument();
    expect(screen.getAllByText("Change PIN").length).toBeGreaterThan(0);
    expect(screen.getByText("Terms & Conditions")).toBeInTheDocument();
  });

  describe("phone section", () => {
    it("validates a required and well-formed phone number", async () => {
      const user = userEvent.setup();
      render(<AccountPage />);

      await user.click(screen.getByRole("button", { name: /verify phone number/i }));
      expect(screen.getByText("Phone number is required")).toBeInTheDocument();

      await user.type(screen.getByLabelText("Phone number"), "699");
      await user.click(screen.getByRole("button", { name: /verify phone number/i }));
      expect(
        screen.getByText("Enter a valid phone number (e.g., +237 6XXXXXXXX)")
      ).toBeInTheDocument();
    });

    it("submits the combined full phone and switches to the status view", async () => {
      mockAddPhone.mockResolvedValue({});
      const user = userEvent.setup();
      render(<AccountPage />);

      await user.type(screen.getByLabelText("Phone number"), "699000000");
      await user.click(screen.getByRole("button", { name: /verify phone number/i }));

      expect(mockAddPhone).toHaveBeenCalledWith("+237699000000");
      expect(await screen.findByText("Verified")).toBeInTheDocument();
      expect(screen.getByText("+237699000000")).toBeInTheDocument();
    });

    it("shows the USSD code box and a pending status", () => {
      mockHooks({
        verif: {
          phone_number: "+237699000000",
          phone_verified: false,
          verification: {
            id: "v1",
            status: "pending",
            created_at: new Date().toISOString(),
            ussd_code: "*126#",
          },
        },
      });
      render(<AccountPage />);

      expect(screen.getByText("Dial the USSD code on your phone")).toBeInTheDocument();
      expect(screen.getByText("*126#")).toBeInTheDocument();
      expect(screen.getByText("Processing…")).toBeInTheDocument();
    });

    it("shows Try Again immediately when verification failed", () => {
      mockHooks({
        verif: {
          phone_number: "+237699000000",
          phone_verified: false,
          verification: {
            id: "v1",
            status: "failed",
            created_at: new Date().toISOString(),
          },
        },
      });
      render(<AccountPage />);
      expect(screen.getByRole("button", { name: /try again/i })).toBeInTheDocument();
    });

    it("does not show Try Again for a recent pending verification", () => {
      mockHooks({
        verif: {
          phone_number: "+237699000000",
          phone_verified: false,
          verification: {
            id: "v1",
            status: "pending",
            created_at: new Date().toISOString(),
          },
        },
      });
      render(<AccountPage />);
      expect(screen.queryByRole("button", { name: /try again/i })).not.toBeInTheDocument();
    });

    it("does not show Try Again once verified", () => {
      mockHooks({
        verif: {
          phone_number: "+237699000000",
          phone_verified: true,
          verification: {
            id: "v1",
            status: "success",
            created_at: new Date().toISOString(),
          },
        },
      });
      render(<AccountPage />);
      expect(screen.getByText("Yes")).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: /try again/i })).not.toBeInTheDocument();
    });

    it("shows an inline error when adding the phone fails", async () => {
      mockAddPhone.mockRejectedValue({
        response: { data: { error: "Campay unavailable" } },
      });
      const user = userEvent.setup();
      render(<AccountPage />);

      await user.type(screen.getByLabelText("Phone number"), "699000000");
      await user.click(screen.getByRole("button", { name: /verify phone number/i }));

      expect(await screen.findByText("Campay unavailable")).toBeInTheDocument();
    });
  });

  describe("change PIN section", () => {
    it("blocks a submit when the new PINs do not match", async () => {
      const user = userEvent.setup();
      render(<AccountPage />);

      await user.type(screen.getByLabelText("Current PIN"), "11111");
      await user.type(screen.getByLabelText("New PIN"), "22222");
      await user.type(screen.getByLabelText("Confirm New PIN"), "33333");
      await user.click(screen.getByRole("button", { name: /change pin/i }));

      expect(screen.getByText("New PINs do not match")).toBeInTheDocument();
      expect(mockChangePin).not.toHaveBeenCalled();
    });

    it("submits current and new PINs and clears the form on success", async () => {
      mockChangePin.mockResolvedValue({ message: "ok" });
      const user = userEvent.setup();
      render(<AccountPage />);

      await user.type(screen.getByLabelText("Current PIN"), "11111");
      await user.type(screen.getByLabelText("New PIN"), "22222");
      await user.type(screen.getByLabelText("Confirm New PIN"), "22222");
      await user.click(screen.getByRole("button", { name: /change pin/i }));

      expect(mockChangePin).toHaveBeenCalledWith({ current_pin: "11111", new_pin: "22222" });
      await waitFor(() => expect(toast.success).toHaveBeenCalledWith("PIN changed"));
      expect(screen.getByLabelText("Current PIN")).toHaveValue("");
      expect(screen.getByLabelText("New PIN")).toHaveValue("");
    });

    it("shows the server error when the current PIN is wrong", async () => {
      mockChangePin.mockRejectedValue({
        response: { data: { error: "Invalid current PIN", code: "invalid_pin" } },
      });
      const user = userEvent.setup();
      render(<AccountPage />);

      await user.type(screen.getByLabelText("Current PIN"), "00000");
      await user.type(screen.getByLabelText("New PIN"), "22222");
      await user.type(screen.getByLabelText("Confirm New PIN"), "22222");
      await user.click(screen.getByRole("button", { name: /change pin/i }));

      expect(await screen.findByText("Invalid current PIN")).toBeInTheDocument();
    });
  });

  describe("terms section", () => {
    it("keeps Accept disabled until the checkbox is checked, then submits", async () => {
      mockAcceptTerms.mockResolvedValue({});
      const user = userEvent.setup();
      render(<AccountPage />);

      const acceptButton = screen.getByRole("button", { name: /accept terms/i });
      expect(acceptButton).toBeDisabled();

      await user.click(screen.getByRole("checkbox"));
      expect(acceptButton).toBeEnabled();

      await user.click(acceptButton);
      expect(mockAcceptTerms).toHaveBeenCalled();
      await waitFor(() => expect(mockRefreshSubject).toHaveBeenCalled());
      await waitFor(() => expect(toast.success).toHaveBeenCalledWith("Terms accepted"));
    });

    it("shows the already-accepted state when terms are accepted", () => {
      mockHooks({ user: { ...baseUser, is_terms_accepted: true } });
      render(<AccountPage />);
      expect(screen.getByText("Terms Already Accepted")).toBeInTheDocument();
      expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
    });
  });

  it("scrolls to the section named in the URL", () => {
    const scrollIntoView = jest.fn();
    Element.prototype.scrollIntoView = scrollIntoView;

    mockUseSearchParams.mockReturnValue(new URLSearchParams({ section: "pin" }));
    render(<AccountPage />);

    expect(scrollIntoView).toHaveBeenCalled();
  });

  describe("desktop layout (two-column settings)", () => {
    beforeEach(() => {
      useMediaQuery.mockReturnValue(true);
    });

    it("defaults to the Phone Number panel and switches panels on nav click", async () => {
      const user = userEvent.setup();
      render(<AccountPage />);

      expect(screen.getAllByText("Phone Number").length).toBeGreaterThan(0);
      expect(screen.queryByText("Choose a 5-digit PIN", { exact: false })).not.toBeInTheDocument();

      await user.click(screen.getByRole("button", { name: "Change PIN" }));
      expect(screen.getByLabelText("Current PIN")).toBeInTheDocument();
      expect(screen.queryByLabelText("Phone number")).not.toBeInTheDocument();
    });

    it("does not scroll into view (no anchor sections to scroll to)", () => {
      const scrollIntoView = jest.fn();
      Element.prototype.scrollIntoView = scrollIntoView;

      mockUseSearchParams.mockReturnValue(new URLSearchParams({ section: "pin" }));
      render(<AccountPage />);

      expect(scrollIntoView).not.toHaveBeenCalled();
      expect(screen.getByLabelText("Current PIN")).toBeInTheDocument();
    });
  });
});
