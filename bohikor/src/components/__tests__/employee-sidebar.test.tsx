import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EmployeeSidebar } from "../employee-sidebar";

const mockSignOut = jest.fn();
const mockRefreshSubject = jest.fn();
const mockPush = jest.fn();
const mockToggle = jest.fn();
let mockCollapsed = false;

jest.mock("next/navigation", () => ({
  usePathname: jest.fn(),
  useParams: () => ({ company: "acme" }),
  useRouter: () => ({ push: mockPush }),
}));

jest.mock("@/components/providers", () => ({
  useAuth: () => ({ signOut: mockSignOut, refreshSubject: mockRefreshSubject }),
}));

jest.mock("@/hooks/use-sidebar-collapse", () => ({
  useSidebarCollapse: () => ({ collapsed: mockCollapsed, toggle: mockToggle }),
}));

jest.mock("next-themes", () => ({
  useTheme: () => ({ resolvedTheme: "dark", setTheme: jest.fn() }),
}));

const { usePathname } = jest.requireMock("next/navigation");

describe("EmployeeSidebar", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockCollapsed = false;
    usePathname.mockReturnValue("/acme");
  });

  it("renders Home, History, and Account nav links with slug-prefixed hrefs", () => {
    render(<EmployeeSidebar />);

    const expected: [string, string][] = [
      ["Home", "/acme"],
      ["History", "/acme/history"],
      ["Account", "/acme/account"],
    ];

    for (const [label, href] of expected) {
      const link = screen.getByRole("link", { name: label });
      expect(link).toHaveAttribute("href", href);
    }
  });

  it("applies the primary-token active-state classes to the current route's link", () => {
    usePathname.mockReturnValue("/acme/history");
    render(<EmployeeSidebar />);

    const historyLink = screen.getByRole("link", { name: "History" });
    const homeLink = screen.getByRole("link", { name: "Home" });

    expect(historyLink.className).toContain("bg-primary/10");
    expect(homeLink.className).not.toContain("bg-primary/10");
  });

  it("calls signOut and redirects to the company login page when sign out is clicked", async () => {
    const user = userEvent.setup();
    render(<EmployeeSidebar />);

    await user.click(screen.getByRole("button", { name: "Sign Out" }));

    expect(mockSignOut).toHaveBeenCalled();
    expect(mockRefreshSubject).toHaveBeenCalled();
    expect(mockPush).toHaveBeenCalledWith("/acme/login");
  });

  it("hides nav labels and the brand name, and adds a title tooltip, when collapsed", () => {
    mockCollapsed = true;
    render(<EmployeeSidebar />);

    expect(screen.queryByText("Home")).not.toBeInTheDocument();
    expect(screen.queryByText("Bohikor")).not.toBeInTheDocument();
    expect(screen.queryByText("Sign Out")).not.toBeInTheDocument();

    const homeLink = screen.getByRole("link", { name: "Home" });
    expect(homeLink).toHaveAttribute("title", "Home");
  });

  it("calls toggle when the collapse/expand control is clicked, without signing out", async () => {
    const user = userEvent.setup();
    render(<EmployeeSidebar />);

    await user.click(screen.getByRole("button", { name: "Collapse sidebar" }));

    expect(mockToggle).toHaveBeenCalled();
    expect(mockSignOut).not.toHaveBeenCalled();
  });

  it("shows an Expand control in place of Collapse when already collapsed", () => {
    mockCollapsed = true;
    render(<EmployeeSidebar />);

    expect(screen.getByRole("button", { name: "Expand sidebar" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Collapse sidebar" })).not.toBeInTheDocument();
  });
});
