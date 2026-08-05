import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { Company, LedgerEntry } from "@/types";

// GET /api/platform/companies -> { data: Company[] }
export function useCompanies() {
  return useQuery({
    queryKey: ["companies"],
    queryFn: async () => {
      const { data } = await api.get<{ data: Company[] }>("/api/platform/companies");
      return data.data;
    },
  });
}

// GET /api/platform/companies/:id -> { data: Company }
export function useCompany(id: string, enabled = true) {
  return useQuery({
    queryKey: ["company", id],
    queryFn: async () => {
      const { data } = await api.get<{ data: Company }>(`/api/platform/companies/${id}`);
      return data.data;
    },
    enabled,
  });
}

// POST /api/platform/companies  { slug, name }  -> 201 { data: Company }
// slug must match /^[a-z0-9]+(-[a-z0-9]+)*$/ (lowercase words separated by single hyphens) — this
// is enforced server-side (400 invalid_slug) and should also be validated client-side before submit.
export function useCreateCompany() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (input: { slug: string; name: string }) => {
      const { data } = await api.post<{ data: Company }>("/api/platform/companies", input);
      return data.data;
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["companies"] }),
  });
}

// PUT /api/platform/companies/:id/status  { status: "active" | "suspended" }  -> { data: Company }
export function useUpdateCompanyStatus() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (input: { id: string; status: "active" | "suspended" }) => {
      const { data } = await api.put<{ data: Company }>(
        `/api/platform/companies/${input.id}/status`,
        { status: input.status }
      );
      return data.data;
    },
    onSuccess: (_data, vars) => {
      queryClient.invalidateQueries({ queryKey: ["companies"] });
      queryClient.invalidateQueries({ queryKey: ["company", vars.id] });
    },
  });
}

// POST /api/platform/companies/:id/admins  { email, password }  -> 201 { data: { id, company_id, email, created_at } }
export function useCreateCompanyAdmin() {
  return useMutation({
    mutationFn: async (input: { companyId: string; email: string; password: string }) => {
      const { data } = await api.post<{
        data: { id: string; company_id: string; email: string; created_at: string };
      }>(`/api/platform/companies/${input.companyId}/admins`, {
        email: input.email,
        password: input.password,
      });
      return data.data;
    },
  });
}

// POST /api/platform/companies/:id/ledger/topup  { amount_xaf: string, note?: string }
//   -> 201 { data: { entry: LedgerEntry, balance_xaf: string } }
export function useTopUpCompany() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (input: { companyId: string; amount_xaf: string; note?: string }) => {
      const { data } = await api.post<{ data: { entry: LedgerEntry; balance_xaf: string } }>(
        `/api/platform/companies/${input.companyId}/ledger/topup`,
        { amount_xaf: input.amount_xaf, note: input.note }
      );
      return data.data;
    },
    onSuccess: (_data, vars) => {
      queryClient.invalidateQueries({ queryKey: ["companies"] });
      queryClient.invalidateQueries({ queryKey: ["company", vars.companyId] });
    },
  });
}

// POST /api/platform/companies/:id/ledger/adjustment  { amount_xaf: string (signed, non-zero), note: string (required) }
//   -> same response shape as topup.
export function useAdjustCompanyLedger() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (input: { companyId: string; amount_xaf: string; note: string }) => {
      const { data } = await api.post<{ data: { entry: LedgerEntry; balance_xaf: string } }>(
        `/api/platform/companies/${input.companyId}/ledger/adjustment`,
        { amount_xaf: input.amount_xaf, note: input.note }
      );
      return data.data;
    },
    onSuccess: (_data, vars) => {
      queryClient.invalidateQueries({ queryKey: ["companies"] });
      queryClient.invalidateQueries({ queryKey: ["company", vars.companyId] });
    },
  });
}
