import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EmployeeHeader } from "../employee-header";

const mockSignOut = jest.fn();
const mockRefreshSubject = jest.fn();
const mockPush = jest.fn();

jest.mock("next/navigation", () => ({
  useParams: () => ({ company: "acme" }),
  useRouter: () => ({ push: mockPush }),
}));

jest.mock("@/components/providers", () => ({
  useAuth: () => ({ signOut: mockSignOut, refreshSubject: mockRefreshSubject }),
}));

describe("EmployeeHeader", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("links the logo/wordmark to the company home page", () => {
    render(<EmployeeHeader />);
    expect(screen.getByRole("link", { name: /bohikor/i })).toHaveAttribute("href", "/acme");
  });

  it("navigates to Home, History, and Account from the menu", async () => {
    const user = userEvent.setup();
    render(<EmployeeHeader />);

    await user.click(screen.getByRole("button", { name: "Navigation menu" }));
    await user.click(screen.getByRole("menuitem", { name: /history/i }));
    expect(mockPush).toHaveBeenCalledWith("/acme/history");
  });

  it("calls signOut and redirects to the company login page when sign out is clicked", async () => {
    const user = userEvent.setup();
    render(<EmployeeHeader />);

    await user.click(screen.getByRole("button", { name: "Navigation menu" }));
    await user.click(screen.getByRole("menuitem", { name: /sign out/i }));

    expect(mockSignOut).toHaveBeenCalled();
    expect(mockRefreshSubject).toHaveBeenCalled();
    expect(mockPush).toHaveBeenCalledWith("/acme/login");
  });
});
