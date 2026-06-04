import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";

interface Setting {
  key: string;
  value: string | number | boolean;
  updated_by: string | null;
  updated_at: string | null;
}

export function useSettings() {
  return useQuery({
    queryKey: ["settings"],
    queryFn: async () => {
      const { data } = await api.get<{ data: Setting[] }>("/api/admin/settings");
      const settings: Record<string, string> = {};
      for (const s of data.data) {
        settings[s.key] = String(s.value);
      }
      return settings;
    },
  });
}

export function useUpdateSetting() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({ key, value }: { key: string; value: string }) => {
      const { data } = await api.put<{ data: Record<string, string> }>(
        "/api/admin/settings",
        { [key]: value }
      );
      return data.data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["settings"] });
    },
  });
}
