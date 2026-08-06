import { renderHook, act } from "@testing-library/react";
import { useSidebarCollapse } from "../use-sidebar-collapse";

describe("useSidebarCollapse", () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  it("defaults to expanded when nothing is stored", () => {
    const { result } = renderHook(() => useSidebarCollapse());
    expect(result.current.collapsed).toBe(false);
  });

  it("reads a previously stored collapsed state on mount", () => {
    window.localStorage.setItem("bohikor:sidebar-collapsed", "true");
    const { result } = renderHook(() => useSidebarCollapse());
    expect(result.current.collapsed).toBe(true);
  });

  it("toggling flips the state and persists it to localStorage", () => {
    const { result } = renderHook(() => useSidebarCollapse());

    act(() => {
      result.current.toggle();
    });

    expect(result.current.collapsed).toBe(true);
    expect(window.localStorage.getItem("bohikor:sidebar-collapsed")).toBe("true");

    act(() => {
      result.current.toggle();
    });

    expect(result.current.collapsed).toBe(false);
    expect(window.localStorage.getItem("bohikor:sidebar-collapsed")).toBe("false");
  });
});
