import { render, screen } from "@testing-library/react";
import LandingPage from "../page";

describe("LandingPage", () => {
  it("renders the heading and tagline", () => {
    render(<LandingPage />);
    expect(screen.getByRole("heading", { name: "Bohikor" })).toBeInTheDocument();
    expect(screen.getByText("Salary advances, made simple.")).toBeInTheDocument();
  });

  it("links to the platform admin login", () => {
    render(<LandingPage />);
    const link = screen.getByRole("link", { name: /platform administrator/i });
    expect(link).toHaveAttribute("href", "/platform/login");
  });
});
