import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { PlatformSidebar } from "../platform-sidebar";

const mockSignOut = jest.fn();
const mockPush = jest.fn();
const mockToggle = jest.fn();
let mockCollapsed = false;

jest.mock("next/navigation", () => ({
  usePathname: jest.fn(),
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

describe("PlatformSidebar", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockCollapsed = false;
    usePathname.mockReturnValue("/platform");
  });

  it("renders the Dashboard nav link", () => {
    render(<PlatformSidebar />);
    expect(screen.getByRole("link", { name: "Dashboard" })).toHaveAttribute("href", "/platform");
  });

  it("applies the primary-token active-state classes on the current route", () => {
    render(<PlatformSidebar />);
    expect(screen.getByRole("link", { name: "Dashboard" }).className).toContain(
      "bg-primary/10"
    );
  });

  it("calls signOut and redirects to the platform login page when sign out is clicked", async () => {
    const user = userEvent.setup();
    render(<PlatformSidebar />);

    await user.click(screen.getByRole("button", { name: "Sign Out" }));

    expect(mockSignOut).toHaveBeenCalled();
    expect(mockPush).toHaveBeenCalledWith("/platform/login");
  });

  it("hides labels and shows a title tooltip when collapsed", () => {
    mockCollapsed = true;
    render(<PlatformSidebar />);

    expect(screen.queryByText("Bohikor")).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Dashboard" })).toHaveAttribute("title", "Dashboard");
  });

  it("calls toggle when the collapse/expand control is clicked", async () => {
    const user = userEvent.setup();
    render(<PlatformSidebar />);

    await user.click(screen.getByRole("button", { name: "Collapse sidebar" }));

    expect(mockToggle).toHaveBeenCalled();
  });
});
