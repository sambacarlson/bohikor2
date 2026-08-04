import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Sidebar } from "../sidebar";

const mockSignOut = jest.fn();
const mockPush = jest.fn();

jest.mock("next/navigation", () => ({
  usePathname: jest.fn(),
  useParams: () => ({ company: "acme" }),
  useRouter: () => ({ push: mockPush }),
}));

jest.mock("@/components/providers", () => ({
  useAuth: () => ({ signOut: mockSignOut }),
}));

const { usePathname } = jest.requireMock("next/navigation");

describe("Sidebar", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    usePathname.mockReturnValue("/acme/admin");
  });

  it("renders all 7 nav links with slug-prefixed hrefs", () => {
    render(<Sidebar />);

    const expected: [string, string][] = [
      ["Dashboard", "/acme/admin"],
      ["Invite", "/acme/admin/invite"],
      ["Users", "/acme/admin/users"],
      ["Requests", "/acme/admin/requests"],
      ["Events", "/acme/admin/events"],
      ["Balance", "/acme/admin/balance"],
      ["Settings", "/acme/admin/settings"],
    ];

    for (const [label, href] of expected) {
      const link = screen.getByText(label).closest("a");
      expect(link).toHaveAttribute("href", href);
    }
  });

  it("applies the active styling class to the current route's link", () => {
    usePathname.mockReturnValue("/acme/admin/users");
    render(<Sidebar />);

    const usersLink = screen.getByText("Users").closest("a");
    const dashboardLink = screen.getByText("Dashboard").closest("a");

    expect(usersLink?.className).toContain("bg-sidebar-primary");
    expect(dashboardLink?.className).not.toContain("bg-sidebar-primary");
  });

  it("calls signOut and redirects to the company login page when the sign out button is clicked", async () => {
    const user = userEvent.setup();
    render(<Sidebar />);

    await user.click(screen.getByText("Sign Out"));

    expect(mockSignOut).toHaveBeenCalled();
    expect(mockPush).toHaveBeenCalledWith("/acme/admin/login");
  });
});
