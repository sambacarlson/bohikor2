import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { AdvanceRequest } from "@/types";

export function useRequests(page = 1, perPage = 20) {
  return useQuery({
    queryKey: ["requests", page, perPage],
    queryFn: async () => {
      const { data } = await api.get<{ data: AdvanceRequest[] }>("/api/admin/requests", {
        params: { page, per_page: perPage },
      });
      return data.data;
    },
  });
}

// POST /api/admin/requests/:id/reconcile -> { data: AdvanceRequest }
// 409 { code: "nothing_to_poll" } if the request has no campay_payout_ref to poll.
export function useReconcileRequest() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => {
      const { data } = await api.post<{ data: AdvanceRequest }>(
        `/api/admin/requests/${id}/reconcile`
      );
      return data.data;
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["requests"] }),
  });
}

// POST /api/admin/requests/:id/resolve  { note: string }  -> { data: AdvanceRequest }
// 409 { code: "not_flagged_for_review" } if it's not currently flagged.
export function useResolveRequest() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (input: { id: string; note: string }) => {
      const { data } = await api.post<{ data: AdvanceRequest }>(
        `/api/admin/requests/${input.id}/resolve`,
        { note: input.note }
      );
      return data.data;
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["requests"] }),
  });
}

// POST /api/admin/requests/:id/reissue (no body) -> 201/202 { data: AdvanceRequest } on success.
// Only legal on a terminally "failed" request. Possible failures: 403 insufficient_employer_float,
// 409 already_reissued, 502 transfer_failed (still creates a row, just failed again — refetch).
export function useReissueRequest() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => {
      const { data } = await api.post<{ data: AdvanceRequest }>(
        `/api/admin/requests/${id}/reissue`
      );
      return data.data;
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["requests"] }),
  });
}
