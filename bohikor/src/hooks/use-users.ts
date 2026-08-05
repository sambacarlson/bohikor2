import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { AdminUser } from "@/types";

export function useUsers(page = 1, perPage = 20) {
  return useQuery({
    queryKey: ["users", page, perPage],
    queryFn: async () => {
      const { data } = await api.get<{ data: AdminUser[] }>("/api/admin/users", {
        params: { page, per_page: perPage },
      });
      return data.data;
    },
  });
}

export function useSuspendUser() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (userId: string) => {
      const { data } = await api.put<AdminUser>(`/api/admin/users/${userId}/suspend`);
      return data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users"] });
    },
  });
}

export function useActivateUser() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (userId: string) => {
      const { data } = await api.put<AdminUser>(`/api/admin/users/${userId}/activate`);
      return data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users"] });
    },
  });
}

export function useUnlockUser() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (userId: string) => {
      const { data } = await api.put<AdminUser>(`/api/admin/users/${userId}/unlock`);
      return data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users"] });
    },
  });
}
