// Shared helpers for unwrapping the `{ error, code }` shape returned by the backend's
// JSONError response on a failed axios request (see internal/handler/handler.go).

export function extractApiError(err: unknown): string {
  return (err as { response?: { data?: { error?: string } } }).response?.data?.error ?? "";
}

export function extractApiErrorCode(err: unknown): string {
  return (err as { response?: { data?: { code?: string } } }).response?.data?.code ?? "";
}
