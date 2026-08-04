import { useQuery } from "@tanstack/react-query";
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
