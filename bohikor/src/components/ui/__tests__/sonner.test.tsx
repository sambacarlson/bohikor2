import { render } from "@testing-library/react";
import { Toaster } from "../sonner";

describe("Toaster", () => {
  it("mounts without throwing", () => {
    const { container } = render(<Toaster />);
    expect(container).toBeInTheDocument();
  });
});
