"use client";

import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { Admin } from "@/types";

export function useAdmin(enabled = true) {
  return useQuery({
    queryKey: ["admin"],
    queryFn: async () => {
      const { data } = await api.get<{ data: Admin }>("/api/admin/me");
      return data.data;
    },
    retry: false,
    enabled,
  });
}
