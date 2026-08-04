import { render } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Toaster } from "sonner";
import type { ReactElement, ReactNode } from "react";

export function makeTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
}

export function renderWithProviders(
  ui: ReactElement,
  { withToaster = true }: { withToaster?: boolean } = {}
) {
  const queryClient = makeTestQueryClient();
  return render(
    <QueryClientProvider client={queryClient}>
      {withToaster && <Toaster />}
      {ui}
    </QueryClientProvider>
  );
}

export function renderHookWithClient() {
  const queryClient = makeTestQueryClient();
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  return { wrapper, queryClient };
}
