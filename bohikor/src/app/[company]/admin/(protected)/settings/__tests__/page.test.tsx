import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { Toaster } from "sonner";
import SettingsPage from "../page";
import { renderWithProviders, makeTestQueryClient } from "@/test-utils";

jest.mock("@/hooks/use-settings", () => ({
  useSettings: jest.fn(),
  useUpdateSetting: jest.fn(),
}));

jest.mock("sonner", () => ({
  ...jest.requireActual("sonner"),
  toast: { success: jest.fn(), error: jest.fn() },
}));

const { useSettings, useUpdateSetting } = jest.requireMock("@/hooks/use-settings");
const { toast } = jest.requireMock("sonner");

const settingsFixture = {
  advance_amount_xaf: "5000",
  kill_switch_enabled: "false",
  request_window_start_day: "1",
  request_window_end_day: "25",
  daily_request_limit: "3",
  monthly_request_limit: "10",
};

describe("SettingsPage", () => {
  let mutateFn: jest.Mock;

  beforeEach(() => {
    jest.clearAllMocks();
    mutateFn = jest.fn();
    useUpdateSetting.mockReturnValue({ mutate: mutateFn, isPending: false });
  });

  it("shows only the loading text while settings are loading", () => {
    useSettings.mockReturnValue({
      data: undefined,
      isLoading: true,
      refetch: jest.fn(),
      isRefetching: false,
    });
    renderWithProviders(<SettingsPage />);

    expect(screen.getByText("Loading settings...")).toBeInTheDocument();
    expect(screen.queryByText("Settings")).not.toBeInTheDocument();
  });

  it("populates all four cards from fetched settings", () => {
    useSettings.mockReturnValue({
      data: settingsFixture,
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });
    renderWithProviders(<SettingsPage />);

    expect(screen.getByText("Advance Amount")).toBeInTheDocument();
    expect(screen.getByDisplayValue("5000")).toBeInTheDocument();
    expect(screen.getByText("Advance requests allowed")).toBeInTheDocument();
    expect(screen.getByDisplayValue("1")).toBeInTheDocument();
    expect(screen.getByDisplayValue("25")).toBeInTheDocument();
    expect(screen.getByDisplayValue("3")).toBeInTheDocument();
    expect(screen.getByDisplayValue("10")).toBeInTheDocument();
  });

  it("shows the blocked label when kill_switch_enabled is true", () => {
    useSettings.mockReturnValue({
      data: { ...settingsFixture, kill_switch_enabled: "true" },
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });
    renderWithProviders(<SettingsPage />);

    expect(screen.getByText("Advance requests blocked")).toBeInTheDocument();
  });

  it("refetches when the refresh button is clicked", async () => {
    const refetchFn = jest.fn();
    useSettings.mockReturnValue({
      data: settingsFixture,
      isLoading: false,
      refetch: refetchFn,
      isRefetching: false,
    });
    const user = userEvent.setup();
    renderWithProviders(<SettingsPage />);

    await user.click(screen.getByText("Refresh"));

    expect(refetchFn).toHaveBeenCalled();
  });

  it("edits and saves the Advance Amount card, calling mutate with the right key/value", async () => {
    useSettings.mockReturnValue({
      data: settingsFixture,
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });
    const user = userEvent.setup();
    renderWithProviders(<SettingsPage />);

    const editButtons = screen.getAllByText("Edit");
    await user.click(editButtons[0]);

    const amountInput = screen.getByDisplayValue("5000");
    await user.clear(amountInput);
    await user.type(amountInput, "7000");

    await user.click(screen.getByText("Save"));

    expect(mutateFn).toHaveBeenCalledWith(
      { key: "advance_amount_xaf", value: "7000" },
      expect.objectContaining({ onSuccess: expect.any(Function), onError: expect.any(Function) })
    );
  });

  it("shows a success toast and exits edit mode on save success", async () => {
    useSettings.mockReturnValue({
      data: settingsFixture,
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });
    mutateFn.mockImplementation((_vars, callbacks) => callbacks.onSuccess());
    const user = userEvent.setup();
    renderWithProviders(<SettingsPage />);

    await user.click(screen.getAllByText("Edit")[0]);
    await user.click(screen.getByText("Save"));

    await waitFor(() => expect(toast.success).toHaveBeenCalled());
    expect(screen.getAllByText("Edit").length).toBeGreaterThan(0);
  });

  it("shows an error toast and stays in edit mode on save failure", async () => {
    useSettings.mockReturnValue({
      data: settingsFixture,
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });
    mutateFn.mockImplementation((_vars, callbacks) => callbacks.onError());
    const user = userEvent.setup();
    renderWithProviders(<SettingsPage />);

    await user.click(screen.getAllByText("Edit")[0]);
    await user.click(screen.getByText("Save"));

    await waitFor(() => expect(toast.error).toHaveBeenCalled());
    expect(screen.getByText("Save")).toBeInTheDocument();
  });

  it("saves the Request Window card by calling mutate twice, once per field", async () => {
    useSettings.mockReturnValue({
      data: settingsFixture,
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });
    const user = userEvent.setup();
    renderWithProviders(<SettingsPage />);

    // Third "Edit" button corresponds to the Request Window card
    // (Advance Amount, Kill Switch, Request Window, Rate Limits — in DOM order).
    const editButtons = screen.getAllByText("Edit");
    await user.click(editButtons[2]);

    const saveButtons = screen.getAllByText("Save");
    await user.click(saveButtons[0]);

    expect(mutateFn).toHaveBeenCalledWith(
      expect.objectContaining({ key: "request_window_start_day" }),
      expect.anything()
    );
    expect(mutateFn).toHaveBeenCalledWith(
      expect.objectContaining({ key: "request_window_end_day" }),
      expect.anything()
    );
    expect(mutateFn).toHaveBeenCalledTimes(2);
  });

  it("saves the Rate Limits card by calling mutate twice, once per field", async () => {
    useSettings.mockReturnValue({
      data: settingsFixture,
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });
    const user = userEvent.setup();
    renderWithProviders(<SettingsPage />);

    const editButtons = screen.getAllByText("Edit");
    await user.click(editButtons[3]);

    const saveButtons = screen.getAllByText("Save");
    await user.click(saveButtons[0]);

    expect(mutateFn).toHaveBeenCalledWith(
      expect.objectContaining({ key: "daily_request_limit" }),
      expect.anything()
    );
    expect(mutateFn).toHaveBeenCalledWith(
      expect.objectContaining({ key: "monthly_request_limit" }),
      expect.anything()
    );
    expect(mutateFn).toHaveBeenCalledTimes(2);
  });

  it("falls back to empty values when settings fields are missing from the payload", () => {
    useSettings.mockReturnValue({
      data: {},
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });
    renderWithProviders(<SettingsPage />);

    expect(screen.getByLabelText("Amount (XAF)")).toHaveValue(null);
    expect(screen.getByText("Advance requests allowed")).toBeInTheDocument();
  });

  it("shows a spinning refresh icon while refetching", () => {
    useSettings.mockReturnValue({
      data: settingsFixture,
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: true,
    });
    const { container } = renderWithProviders(<SettingsPage />);

    expect(container.querySelector(".animate-spin")).toBeInTheDocument();
  });

  it("edits and saves Kill Switch when toggled on, calling mutate with 'true'", async () => {
    useSettings.mockReturnValue({
      data: settingsFixture,
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });
    const user = userEvent.setup();
    renderWithProviders(<SettingsPage />);

    const editButtons = screen.getAllByText("Edit");
    await user.click(editButtons[1]); // Kill Switch card

    await user.click(screen.getByRole("switch"));
    await user.click(screen.getByText("Save"));

    expect(mutateFn).toHaveBeenCalledWith(
      { key: "kill_switch_enabled", value: "true" },
      expect.objectContaining({ onSuccess: expect.any(Function), onError: expect.any(Function) })
    );
  });

  it("edits and saves Kill Switch without toggling, calling mutate with 'false'", async () => {
    useSettings.mockReturnValue({
      data: settingsFixture,
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });
    const user = userEvent.setup();
    renderWithProviders(<SettingsPage />);

    const editButtons = screen.getAllByText("Edit");
    await user.click(editButtons[1]); // Kill Switch card

    await user.click(screen.getByText("Save"));

    expect(mutateFn).toHaveBeenCalledWith(
      { key: "kill_switch_enabled", value: "false" },
      expect.objectContaining({ onSuccess: expect.any(Function), onError: expect.any(Function) })
    );
  });

  it("does not re-populate fields from a later settings payload after initial mount", () => {
    useSettings.mockReturnValue({
      data: settingsFixture,
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });
    const { rerender } = renderWithProviders(<SettingsPage />);
    expect(screen.getByDisplayValue("5000")).toBeInTheDocument();

    useSettings.mockReturnValue({
      data: { ...settingsFixture, advance_amount_xaf: "9999" },
      isLoading: false,
      refetch: jest.fn(),
      isRefetching: false,
    });
    // Rerender through the same provider tree shape as the initial render
    // (QueryClientProvider > Toaster > SettingsPage) so React reconciles the
    // existing SettingsPage instance in place instead of unmounting and
    // remounting it — a bare `rerender(<SettingsPage />)` would change the
    // root element type and force a remount, resetting the `initialized`
    // ref and defeating the very guard this test is meant to exercise.
    rerender(
      <QueryClientProvider client={makeTestQueryClient()}>
        <Toaster />
        <SettingsPage />
      </QueryClientProvider>
    );

    expect(screen.getByDisplayValue("5000")).toBeInTheDocument();
    expect(screen.queryByDisplayValue("9999")).not.toBeInTheDocument();
  });
});
