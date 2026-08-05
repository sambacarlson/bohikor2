import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { EligibilityResponse } from "@/types";

// GET /api/advance-requests/eligibility -> { data: EligibilityResponse }
export function useEligibility() {
  return useQuery({
    queryKey: ["eligibility"],
    queryFn: async () => {
      const { data } = await api.get<{ data: EligibilityResponse }>(
        "/api/advance-requests/eligibility"
      );
      return data.data;
    },
    refetchInterval: 30_000, // matches mobile's home-screen polling cadence
  });
}
