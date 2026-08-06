import { ThemeProvider } from "../index";

describe("providers barrel", () => {
  it("re-exports ThemeProvider from next-themes", () => {
    expect(typeof ThemeProvider).toBe("function");
  });
});
