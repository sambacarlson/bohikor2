import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ThemeToggle } from "../theme-toggle";

const mockSetTheme = jest.fn();
let mockResolvedTheme = "dark";

jest.mock("next-themes", () => ({
  useTheme: () => ({ resolvedTheme: mockResolvedTheme, setTheme: mockSetTheme }),
}));

describe("ThemeToggle", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockResolvedTheme = "dark";
  });

  it("shows a 'switch to light' control when the resolved theme is dark", async () => {
    render(<ThemeToggle />);
    expect(await screen.findByRole("button", { name: "Switch to light mode" })).toBeInTheDocument();
  });

  it("shows a 'switch to dark' control when the resolved theme is light", async () => {
    mockResolvedTheme = "light";
    render(<ThemeToggle />);
    expect(await screen.findByRole("button", { name: "Switch to dark mode" })).toBeInTheDocument();
  });

  it("calls setTheme with the opposite theme when clicked", async () => {
    const user = userEvent.setup();
    render(<ThemeToggle />);

    const button = await screen.findByRole("button", { name: "Switch to light mode" });
    await user.click(button);

    expect(mockSetTheme).toHaveBeenCalledWith("light");
  });
});
