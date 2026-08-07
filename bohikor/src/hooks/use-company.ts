"use client";

import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";

// GET /api/companies/by-slug/:slug (public, unauthenticated)
// 200 { data: { slug, name } } if the company exists, 404 otherwise — used to
// confirm a {company} URL segment is real before rendering a login form for it.
export function useCompanyBySlug(slug: string) {
  return useQuery({
    queryKey: ["company-by-slug", slug],
    queryFn: async () => {
      const { data } = await api.get<{ data: { slug: string; name: string } }>(
        `/api/companies/by-slug/${encodeURIComponent(slug)}`
      );
      return data.data;
    },
    enabled: !!slug,
    retry: false,
  });
}
