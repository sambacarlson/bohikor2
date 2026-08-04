import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { Invitation } from "@/types";

export function useInvitations() {
  return useQuery({
    queryKey: ["invitations"],
    queryFn: async () => {
      const { data } = await api.get<{ data: Invitation[] }>("/api/admin/invitations");
      return data.data;
    },
    staleTime: Infinity,
  });
}

export function useSendInvite() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (email: string) => {
      const { data } = await api.post<{ data: Invitation }>("/api/admin/invite", {
        email,
      });
      return data.data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["invitations"] });
    },
  });
}
