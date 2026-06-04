import { useQuery } from "@tanstack/react-query";
import { api } from "@/src/lib/api";
import type { EligibilityResponse } from "@/src/types";

export function useEligibility() {
  return useQuery({
    queryKey: ["eligibility"],
    queryFn: async () => {
      const { data } = await api.get<{ data: EligibilityResponse }>(
        "/api/advance-requests/eligibility"
      );
      return data.data;
    },
    refetchInterval: 30_000,
  });
}

export type { EligibilityResponse };
