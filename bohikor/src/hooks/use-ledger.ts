import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { LedgerResponse } from "@/types";

// GET /api/admin/ledger?page=&per_page= -> { data: LedgerResponse }
export function useLedger(page = 1, perPage = 50) {
  return useQuery({
    queryKey: ["ledger", page, perPage],
    queryFn: async () => {
      const { data } = await api.get<{ data: LedgerResponse }>("/api/admin/ledger", {
        params: { page, per_page: perPage },
      });
      return data.data;
    },
  });
}
