import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";

export interface Event {
  id: string;
  user_id: string | null;
  admin_id: string | null;
  event_type: string;
  metadata: string | null;
  user_email: string | null;
  created_at: string;
}

export function useEvents(page = 1, perPage = 50) {
  return useQuery({
    queryKey: ["events", page, perPage],
    queryFn: async () => {
      const { data } = await api.get<{ data: Event[] }>("/api/admin/events", {
        params: { page, per_page: perPage },
      });
      return data.data;
    },
  });
}
