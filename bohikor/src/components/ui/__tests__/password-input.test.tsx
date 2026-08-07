import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { PasswordInput } from "../password-input";

describe("PasswordInput", () => {
  it("renders masked by default and reveals on toggle click", async () => {
    const user = userEvent.setup();
    render(<PasswordInput aria-label="PIN" value="12345" onChange={() => {}} />);

    const input = screen.getByLabelText("PIN");
    expect(input).toHaveAttribute("type", "password");

    await user.click(screen.getByRole("button", { name: "Show" }));
    expect(input).toHaveAttribute("type", "text");

    await user.click(screen.getByRole("button", { name: "Hide" }));
    expect(input).toHaveAttribute("type", "password");
  });

  it("doesn't submit the enclosing form when the toggle is clicked", async () => {
    const handleSubmit = jest.fn((e) => e.preventDefault());
    const user = userEvent.setup();
    render(
      <form onSubmit={handleSubmit}>
        <PasswordInput aria-label="Password" value="hunter2" onChange={() => {}} />
      </form>
    );

    await user.click(screen.getByRole("button", { name: "Show" }));
    expect(handleSubmit).not.toHaveBeenCalled();
  });
});
