import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { CompanyRequestHealth, RequestNeedingReview } from "@/types";

// GET /api/platform/requests/needs-review -> { data: RequestNeedingReview[] }
export function useRequestsNeedingReview() {
  return useQuery({
    queryKey: ["platform-requests-needs-review"],
    queryFn: async () => {
      const { data } = await api.get<{ data: RequestNeedingReview[] }>(
        "/api/platform/requests/needs-review"
      );
      return data.data;
    },
  });
}

// GET /api/platform/requests/health -> { data: CompanyRequestHealth[] }
export function useRequestsHealth() {
  return useQuery({
    queryKey: ["platform-requests-health"],
    queryFn: async () => {
      const { data } = await api.get<{ data: CompanyRequestHealth[] }>(
        "/api/platform/requests/health"
      );
      return data.data;
    },
  });
}
