import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/src/lib/api";
import type { AdvanceRequest, User } from "@/src/types";

export function useAcceptTerms() {
  return useMutation({
    mutationFn: async ({ version }: { version: string }) => {
      const { data } = await api.put<{ data: User }>("/api/users/terms", { version });
      return data.data;
    },
  });
}

export function useCreateAdvanceRequest() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async () => {
      const { data } = await api.post<{ data: AdvanceRequest }>("/api/advance-requests");
      return data.data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["advance-requests"] });
    },
  });
}

export function useAdvanceRequests() {
  return useQuery({
    queryKey: ["advance-requests"],
    queryFn: async () => {
      const { data } = await api.get<{ data: AdvanceRequest[] }>("/api/advance-requests");
      return data.data;
    },
    refetchInterval: (query) => {
      const requests = query.state.data;
      if (!requests || requests.length === 0) return false;
      const hasActive = requests.some(
        (r) => r.status === "initiated" || r.status === "pending"
      );
      return hasActive ? 10000 : false;
    },
  });
}
