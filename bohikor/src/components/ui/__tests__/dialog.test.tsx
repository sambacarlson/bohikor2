import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "../dialog";

jest.mock("@/hooks/use-media-query", () => ({
  useMediaQuery: jest.fn(),
}));

const { useMediaQuery } = jest.requireMock("@/hooks/use-media-query");

function renderDialog() {
  render(
    <Dialog defaultOpen>
      <DialogContent>
        <DialogTitle>Reissue payout?</DialogTitle>
        <DialogDescription>Re-debits float and re-sends via Campay.</DialogDescription>
        <DialogFooter>
          <button>Confirm</button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

describe("DialogContent", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("renders bottom-sheet styling and a drag handle when isDesktop is false", () => {
    useMediaQuery.mockReturnValue(false);
    renderDialog();

    const content = screen.getByRole("dialog");
    expect(content.className).toContain("rounded-t-2xl");
    expect(content.className).toContain("bottom-0");
    expect(screen.getByTestId("dialog-sheet-handle")).toBeInTheDocument();
  });

  it("renders centered-dialog styling and no drag handle when isDesktop is true", () => {
    useMediaQuery.mockReturnValue(true);
    renderDialog();

    const content = screen.getByRole("dialog");
    expect(content.className).toContain("top-1/2");
    expect(content.className).not.toContain("rounded-t-2xl");
    expect(screen.queryByTestId("dialog-sheet-handle")).not.toBeInTheDocument();
  });

  it("renders title, description, and footer content", () => {
    useMediaQuery.mockReturnValue(true);
    renderDialog();

    expect(screen.getByText("Reissue payout?")).toBeInTheDocument();
    expect(screen.getByText("Re-debits float and re-sends via Campay.")).toBeInTheDocument();
    expect(screen.getByText("Confirm")).toBeInTheDocument();
  });

  it("closes when the close button is clicked", async () => {
    useMediaQuery.mockReturnValue(true);
    const user = userEvent.setup();
    renderDialog();

    expect(screen.getByRole("dialog")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Close" }));

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
});
