import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ForbiddenPage } from "../forbidden";

const mockPush = jest.fn();
const mockBack = jest.fn();
const mockSignOut = jest.fn().mockResolvedValue(undefined);

jest.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, back: mockBack }),
}));

jest.mock("@/components/providers", () => ({
  useAuth: () => ({ signOut: mockSignOut }),
}));

describe("ForbiddenPage", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("renders the default 403 content and label", () => {
    render(<ForbiddenPage />);

    expect(screen.getByText("403")).toBeInTheDocument();
    expect(screen.getByText("Access Denied")).toBeInTheDocument();
    expect(screen.getByText("Go to Login")).toBeInTheDocument();
  });

  it("renders a custom backLabel when provided", () => {
    render(<ForbiddenPage backHref="/acme/admin/login" backLabel="Back to sign in" />);

    expect(screen.getByText("Back to sign in")).toBeInTheDocument();
  });

  it("calls signOut then pushes backHref when the primary button is clicked", async () => {
    const user = userEvent.setup();
    render(<ForbiddenPage backHref="/acme/admin/login" backLabel="Back to sign in" />);

    await user.click(screen.getByText("Back to sign in"));

    expect(mockSignOut).toHaveBeenCalled();
    expect(mockPush).toHaveBeenCalledWith("/acme/admin/login");
  });

  it("calls router.back when Go Back is clicked", async () => {
    const user = userEvent.setup();
    render(<ForbiddenPage />);

    await user.click(screen.getByText("Go Back"));

    expect(mockBack).toHaveBeenCalled();
  });
});
