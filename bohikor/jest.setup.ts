import "@testing-library/jest-dom";

// Radix UI (dropdown-menu) needs these in jsdom. `jest.fn()`'s Mock<> type
// isn't structurally assignable to the plain DOM prototype method types
// (params/return don't line up through the mock wrapper), so route through
// `unknown` rather than `any` to keep eslint's no-explicit-any rule happy.
Element.prototype.hasPointerCapture = jest.fn(() => false) as unknown as (
  pointerId: number
) => boolean;
Element.prototype.releasePointerCapture = jest.fn() as unknown as (
  pointerId: number
) => void;
Element.prototype.scrollIntoView = jest.fn() as unknown as (
  arg?: boolean | ScrollIntoViewOptions
) => void;

// sonner/Toaster needs matchMedia in jsdom.
Object.defineProperty(window, "matchMedia", {
  writable: true,
  configurable: true,
  value: jest.fn((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: jest.fn(),
    removeListener: jest.fn(),
    addEventListener: jest.fn(),
    removeEventListener: jest.fn(),
    dispatchEvent: jest.fn(),
  })),
});
