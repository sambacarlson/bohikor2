"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { AuthResponse } from "@/types";

// GET /api/auth/check-invite?email=<email>
// Success (200): { data: { has_invitation: true, status: "pending"|"sent"|"accepted"|"revoked"|"failed" } }
// No active invitation (404): { error: "no_invitation", ... } — treat any error response as "not invited".
export function useCheckInvite(email: string, enabled: boolean) {
  return useQuery({
    queryKey: ["check-invite", email],
    queryFn: async () => {
      const { data } = await api.get<{ data: { has_invitation: true; status: string } }>(
        "/api/auth/check-invite",
        { params: { email } }
      );
      return data.data;
    },
    enabled,
    retry: false,
  });
}

// POST /api/auth/send-email-otp  { email }  -> 200 { status: "ok" } (no data payload)
export function useSendEmailOtp() {
  return useMutation({
    mutationFn: async (email: string) => {
      await api.post("/api/auth/send-email-otp", { email });
    },
  });
}

// POST /api/auth/verify-email-otp  { email, code, purpose: "signup" | "pin_reset" }
// purpose="signup"    -> 200 { status: "ok" } (no tokens yet — proceed to create-pin)
// purpose="pin_reset" -> 200 { data: AuthResponse } (tokens issued immediately — proceed to reset-pin, already authenticated)
export function useVerifyEmailOtp() {
  return useMutation({
    mutationFn: async (input: { email: string; code: string; purpose: "signup" | "pin_reset" }) => {
      const { data } = await api.post<{ data?: AuthResponse }>("/api/auth/verify-email-otp", input);
      return data.data ?? null;
    },
  });
}

// POST /api/auth/login  { email, pin, company_slug }  -> 200 { data: AuthResponse }
// company_slug must match the resolved user's actual company (server-verified) or the
// request is rejected the same as a wrong PIN — see backend/internal/handler/auth.go's Login().
export function useLogin() {
  return useMutation({
    mutationFn: async (input: { email: string; pin: string; company_slug: string }) => {
      const { data } = await api.post<{ data: AuthResponse }>("/api/auth/login", input);
      return data.data;
    },
  });
}

// POST /api/auth/create-pin  { email, pin }  -> 201 { data: AuthResponse } (no company_slug)
export function useCreatePin() {
  return useMutation({
    mutationFn: async (input: { email: string; pin: string }) => {
      const { data } = await api.post<{ data: AuthResponse }>("/api/auth/create-pin", input);
      return data.data;
    },
  });
}

// POST /api/auth/forgot-pin  { email }  -> 200 { status: "ok" }
export function useForgotPin() {
  return useMutation({
    mutationFn: async (email: string) => {
      await api.post("/api/auth/forgot-pin", { email });
    },
  });
}
