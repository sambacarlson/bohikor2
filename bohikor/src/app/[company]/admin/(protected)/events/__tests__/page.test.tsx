import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import EventsPage from "../page";
import { renderWithProviders as renderWithProvidersBase } from "@/test-utils";

jest.mock("@/hooks/use-events", () => ({
  useEvents: jest.fn(),
}));

const { useEvents } = jest.requireMock("@/hooks/use-events");

function renderWithProviders(ui: React.ReactElement) {
  return renderWithProvidersBase(ui, { withToaster: false });
}

const mockEvent = {
  id: "evt-1",
  user_id: "user-1",
  admin_id: null,
  event_type: "request_initiated",
  metadata: null,
  user_email: "employee@example.com",
  created_at: "2026-05-20T12:00:00Z",
};

describe("EventsPage", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("renders the page title and description", () => {
    useEvents.mockReturnValue({
      data: [],
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });

    renderWithProviders(<EventsPage />);

    expect(screen.getByRole("heading", { name: /events/i })).toBeInTheDocument();
    expect(screen.getByText(/full activity log/i)).toBeInTheDocument();
  });

  it("shows loading state", () => {
    useEvents.mockReturnValue({
      data: null,
      isLoading: true,
      refetch: jest.fn(),
      isRefetching: false,
    });

    renderWithProviders(<EventsPage />);

    expect(screen.getByText(/loading events/i)).toBeInTheDocument();
  });

  it("shows the empty state", () => {
    useEvents.mockReturnValue({
      data: [],
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });

    renderWithProviders(<EventsPage />);

    expect(screen.getByText(/no events yet/i)).toBeInTheDocument();
  });

  it("renders events with mapped labels and user emails", () => {
    useEvents.mockReturnValue({
      data: [mockEvent],
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });

    renderWithProviders(<EventsPage />);

    expect(screen.getByText("Request Initiated")).toBeInTheDocument();
    expect(screen.getByText("employee@example.com")).toBeInTheDocument();
  });

  it("renders multiple event types", () => {
    useEvents.mockReturnValue({
      data: [
        mockEvent,
        { ...mockEvent, id: "evt-2", event_type: "payout_success", user_email: null },
        { ...mockEvent, id: "evt-3", event_type: "payout_failed" },
      ],
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });

    renderWithProviders(<EventsPage />);

    expect(screen.getByText("Payout Successful")).toBeInTheDocument();
    expect(screen.getByText("Payout Failed")).toBeInTheDocument();
  });

  it("falls back to the raw event type for unmapped types", () => {
    useEvents.mockReturnValue({
      data: [{ ...mockEvent, id: "evt-x", event_type: "custom_event" }],
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });

    renderWithProviders(<EventsPage />);

    expect(screen.getByText("custom_event")).toBeInTheDocument();
  });

  it("does not render the full list capped at 20", () => {
    const events = Array.from({ length: 25 }, (_, i) => ({
      ...mockEvent,
      id: `evt-${i}`,
      event_type: "request_initiated",
    }));

    useEvents.mockReturnValue({
      data: events,
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });

    renderWithProviders(<EventsPage />);

    expect(screen.getAllByText("Request Initiated").length).toBe(25);
  });

  it("refetches when the refresh button is clicked", async () => {
    const user = userEvent.setup();
    const refetchFn = jest.fn();
    useEvents.mockReturnValue({
      data: [],
      isLoading: false,
      refetch: refetchFn,
      isRefetching: false,
    });

    renderWithProviders(<EventsPage />);

    const refreshButton = screen.getByRole("button", { name: /refresh/i });
    await user.click(refreshButton);

    expect(refetchFn).toHaveBeenCalled();
  });

  it("shows a spinning refresh icon while refetching", () => {
    useEvents.mockReturnValue({
      data: [],
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: true,
    });

    const { container } = renderWithProviders(<EventsPage />);

    expect(container.querySelector(".animate-spin")).toBeInTheDocument();
  });
});
