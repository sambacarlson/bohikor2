import { render, screen } from "@testing-library/react";
import DashboardPage from "../page";

jest.mock("@/hooks/use-users", () => ({
  useUsers: jest.fn(),
}));

jest.mock("@/hooks/use-invitations", () => ({
  useInvitations: jest.fn(),
}));

jest.mock("@/hooks/use-requests", () => ({
  useRequests: jest.fn(),
}));

jest.mock("@/hooks/use-events", () => ({
  useEvents: jest.fn(),
}));

const { useUsers } = jest.requireMock("@/hooks/use-users");
const { useInvitations } = jest.requireMock("@/hooks/use-invitations");
const { useRequests } = jest.requireMock("@/hooks/use-requests");
const { useEvents } = jest.requireMock("@/hooks/use-events");

describe("DashboardPage", () => {
  beforeEach(() => {
    jest.clearAllMocks();

    useUsers.mockReturnValue({ data: [] });
    useInvitations.mockReturnValue({ data: [] });
    useRequests.mockReturnValue({ data: [] });
    useEvents.mockReturnValue({ data: [], refetch: jest.fn(), isRefetching: false });
  });

  it("renders dashboard title and description", () => {
    render(<DashboardPage />);
    expect(screen.getByText("Dashboard")).toBeInTheDocument();
    expect(screen.getByText("Salary advance pilot overview")).toBeInTheDocument();
  });

  it("shows summary cards with zero counts", () => {
    render(<DashboardPage />);
    expect(screen.getByText("Total Users")).toBeInTheDocument();
    expect(screen.getByText("Total Invitations")).toBeInTheDocument();
    expect(screen.getByText("Advance Requests")).toBeInTheDocument();
    expect(screen.getByText("Activity")).toBeInTheDocument();
  });

  it("shows user count from data", () => {
    useUsers.mockReturnValue({ data: [{ id: "1" }, { id: "2" }, { id: "3" }] });
    render(<DashboardPage />);
    expect(screen.getByText("3")).toBeInTheDocument();
  });

  it("shows invitation count from data", () => {
    useInvitations.mockReturnValue({ data: [{ id: "1" }, { id: "2" }] });
    render(<DashboardPage />);
    expect(screen.getByText("2")).toBeInTheDocument();
  });

  it("shows request summary breakdown", () => {
    useRequests.mockReturnValue({
      data: [
        { id: "1", status: "success" },
        { id: "2", status: "success" },
        { id: "3", status: "failed" },
        { id: "4", status: "pending" },
      ],
    });
    render(<DashboardPage />);
    // Total = 4
    const totalTexts = screen.getAllByText("4");
    expect(totalTexts.length).toBeGreaterThanOrEqual(1);
    // Breakdown text shows counts
    expect(screen.getByText(/2 success/)).toBeInTheDocument();
    expect(screen.getByText(/1 failed/)).toBeInTheDocument();
    expect(screen.getByText(/1 pending/)).toBeInTheDocument();
  });

  it("shows events list when events exist", () => {
    useEvents.mockReturnValue({
      data: [
        { id: "e1", event_type: "payout_success", created_at: "2026-05-20T12:00:00Z" },
        { id: "e2", event_type: "payout_failed", created_at: "2026-05-20T13:00:00Z" },
      ],
    });
    render(<DashboardPage />);
    expect(screen.getByText("Recent Events")).toBeInTheDocument();
    expect(screen.getByText("Payout Successful")).toBeInTheDocument();
    expect(screen.getByText("Payout Failed")).toBeInTheDocument();
  });

  it("shows event count in activity card", () => {
    useEvents.mockReturnValue({
      data: Array.from({ length: 5 }, (_, i) => ({
        id: `e${i}`,
        event_type: "request_initiated",
        created_at: "2026-05-20T12:00:00Z",
      })),
    });
    render(<DashboardPage />);

    const activityCards = screen.getAllByText("5");
    expect(activityCards.length).toBeGreaterThanOrEqual(1);
  });

  it("shows no events message when empty", () => {
    useEvents.mockReturnValue({ data: [] });
    render(<DashboardPage />);
    expect(screen.getByText(/no events yet/i)).toBeInTheDocument();
  });
});
