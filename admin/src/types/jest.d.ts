/* eslint-disable @typescript-eslint/no-explicit-any */
declare namespace jest {
  function fn<T extends (...args: any[]) => any = (...args: any[]) => any>(
    implementation?: T,
  ): Mock<T>;
  function clearAllMocks(): void;
  function resetAllMocks(): void;
  function restoreAllMocks(): void;
  function mock<T extends object>(
    moduleName: string,
    factory?: () => T,
  ): jest;
  function requireMock<T>(moduleName: string): T;
  function unmock(moduleName: string): void;
  function useFakeTimers(config?: Record<string, unknown>): void;
  function useRealTimers(): void;

  interface Mock<
    TArgs extends any[] = any[],
    TReturn = any,
  > {
    (...args: TArgs): TReturn;
    mock: {
      calls: TArgs[];
      contexts: unknown[];
      instances: TReturn[];
      invocationCallOrder: number[];
      results: Array<{ type: string; value: TReturn }>;
      lastCall: TArgs;
      clear(): this;
      reset(): this;
      restore(): this;
    };
    getMockName(): string;
    mockClear(): this;
    mockReset(): this;
    mockRestore(): void;
    mockImplementation(fn: (...args: TArgs) => TReturn): this;
    mockImplementationOnce(fn: (...args: TArgs) => TReturn): this;
    mockResolvedValue(value: TReturn extends Promise<infer U> ? U : never): this;
    mockResolvedValueOnce(value: TReturn extends Promise<infer U> ? U : never): this;
    mockRejectedValue(value: unknown): this;
    mockRejectedValueOnce(value: unknown): this;
    mockReturnValue(value: TReturn): this;
    mockReturnValueOnce(value: TReturn): this;
  }
}

declare function describe(
  name: string,
  fn: () => void | Promise<void>,
): void;
declare function xdescribe(
  name: string,
  fn: () => void | Promise<void>,
): void;
declare function fdescribe(
  name: string,
  fn: () => void | Promise<void>,
): void;
declare function it(
  name: string,
  fn?: () => void | Promise<void>,
  timeout?: number,
): void;
declare function xit(
  name: string,
  fn?: () => void | Promise<void>,
  timeout?: number,
): void;
declare function fit(
  name: string,
  fn?: () => void | Promise<void>,
  timeout?: number,
): void;
declare function test(
  name: string,
  fn?: () => void | Promise<void>,
  timeout?: number,
): void;
declare function xtest(
  name: string,
  fn?: () => void | Promise<void>,
  timeout?: number,
): void;
declare function beforeAll(fn: () => void | Promise<void>, timeout?: number): void;
declare function beforeEach(fn: () => void | Promise<void>, timeout?: number): void;
declare function afterEach(fn: () => void | Promise<void>, timeout?: number): void;
declare function afterAll(fn: () => void | Promise<void>, timeout?: number): void;
declare function expect<T = unknown>(actual: T): {
  toBe(expected: T): void;
  toEqual(expected: T): void;
  toBeDefined(): void;
  toBeNull(): void;
  toBeTruthy(): void;
  toBeFalsy(): void;
  toBeGreaterThan(expected: number): void;
  toBeGreaterThanOrEqual(expected: number): void;
  toBeLessThan(expected: number): void;
  toBeLessThanOrEqual(expected: number): void;
  toContain(expected: unknown): void;
  toHaveLength(expected: number): void;
  toHaveBeenCalled(): void;
  toHaveBeenCalledWith(...args: unknown[]): void;
  toHaveBeenCalledTimes(expected: number): void;
  toThrow(expected?: string | Error): void;
  toBeInTheDocument(): void;
  toBeDisabled(): void;
  toHaveTextContent(expected: string): void;
  toHaveClass(expected: string): void;
  toHaveAttribute(attr: string, value?: string): void;
  toHaveStyle(style: Record<string, string>): void;
  toHaveValue(value: unknown): void;
  toBeChecked(): void;
  toBeVisible(): void;
  toBeEmptyDOMElement(): void;
  toHaveFocus(): void;
  toContainElement(element: unknown): void;
  toContainHTML(html: string): void;
  not: Record<string, (...args: unknown[]) => unknown>;
};
