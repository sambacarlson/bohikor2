"use client";

import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { User } from "@/types";

export function useUser(enabled = true) {
  return useQuery({
    queryKey: ["user"],
    queryFn: async () => {
      const { data } = await api.get<{ data: User }>("/api/users/me");
      return data.data;
    },
    retry: false,
    enabled,
  });
}
