import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Sidebar } from "../sidebar";

const mockSignOut = jest.fn();
const mockPush = jest.fn();
const mockToggle = jest.fn();
let mockCollapsed = false;

jest.mock("next/navigation", () => ({
  usePathname: jest.fn(),
  useParams: () => ({ company: "acme" }),
  useRouter: () => ({ push: mockPush }),
}));

jest.mock("@/components/providers", () => ({
  useAuth: () => ({ signOut: mockSignOut }),
}));

jest.mock("@/hooks/use-sidebar-collapse", () => ({
  useSidebarCollapse: () => ({ collapsed: mockCollapsed, toggle: mockToggle }),
}));

jest.mock("next-themes", () => ({
  useTheme: () => ({ resolvedTheme: "dark", setTheme: jest.fn() }),
}));

const { usePathname } = jest.requireMock("next/navigation");

describe("Sidebar", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockCollapsed = false;
    usePathname.mockReturnValue("/acme/admin");
  });

  it("renders all 7 nav links with slug-prefixed hrefs when expanded", () => {
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
      const link = screen.getByRole("link", { name: label });
      expect(link).toHaveAttribute("href", href);
    }
  });

  it("applies the primary-token active-state classes to the current route's link", () => {
    usePathname.mockReturnValue("/acme/admin/users");
    render(<Sidebar />);

    const usersLink = screen.getByRole("link", { name: "Users" });
    const dashboardLink = screen.getByRole("link", { name: "Dashboard" });

    expect(usersLink.className).toContain("bg-primary/10");
    expect(dashboardLink.className).not.toContain("bg-primary/10");
  });

  it("calls signOut and redirects to the company login page when sign out is clicked", async () => {
    const user = userEvent.setup();
    render(<Sidebar />);

    await user.click(screen.getByRole("button", { name: "Sign Out" }));

    expect(mockSignOut).toHaveBeenCalled();
    expect(mockPush).toHaveBeenCalledWith("/acme/admin/login");
  });

  it("hides nav labels and the brand name, and adds a title tooltip, when collapsed", () => {
    mockCollapsed = true;
    render(<Sidebar />);

    expect(screen.queryByText("Dashboard")).not.toBeInTheDocument();
    expect(screen.queryByText("Bohikor")).not.toBeInTheDocument();

    const dashboardLink = screen.getByRole("link", { name: "Dashboard" });
    expect(dashboardLink).toHaveAttribute("title", "Dashboard");
  });

  it("calls toggle when the collapse/expand control is clicked", async () => {
    const user = userEvent.setup();
    render(<Sidebar />);

    await user.click(screen.getByRole("button", { name: "Collapse sidebar" }));

    expect(mockToggle).toHaveBeenCalled();
  });
});
