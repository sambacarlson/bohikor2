import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { AdvanceRequest } from "@/types";

// GET /api/advance-requests -> { data: AdvanceRequest[] }
export function useMyAdvanceRequests() {
  return useQuery({
    queryKey: ["my-advance-requests"],
    queryFn: async () => {
      const { data } = await api.get<{ data: AdvanceRequest[] }>("/api/advance-requests");
      return data.data;
    },
    // Poll every 10s only while something is still in flight, matching mobile's history screen.
    refetchInterval: (query) => {
      const requests = query.state.data;
      const hasInFlight = requests?.some(
        (r) => r.status === "initiated" || r.status === "processing" || r.status === "pending"
      );
      return hasInFlight ? 10_000 : false;
    },
  });
}

// POST /api/advance-requests (no body) -> 201/202 { data: AdvanceRequest } on success;
// 4xx { error, code } on an eligibility failure (e.g. insufficient_employer_float).
export function useCreateAdvanceRequest() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      const { data } = await api.post<{ data: AdvanceRequest }>("/api/advance-requests");
      return data.data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["my-advance-requests"] });
      queryClient.invalidateQueries({ queryKey: ["eligibility"] });
    },
  });
}

// POST /api/advance-requests/:id/retry -> same response shape as create; only legal on a
// terminally "failed" request.
export function useRetryAdvanceRequest() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => {
      const { data } = await api.post<{ data: AdvanceRequest }>(
        `/api/advance-requests/${id}/retry`
      );
      return data.data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["my-advance-requests"] });
    },
  });
}
