import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { PhoneVerificationStatus, User } from "@/types";

// PUT /api/users/terms  { version: "v1" }  -> { data: User }
// After success, call useAuth()'s refreshSubject() to sync the user in context.
export function useAcceptTerms() {
  return useMutation({
    mutationFn: async () => {
      const { data } = await api.put<{ data: User }>("/api/users/terms", { version: "v1" });
      return data.data;
    },
  });
}

// PUT /api/users/me/pin  { current_pin, new_pin }  -> { data: { message: string } }
export function useChangePin() {
  return useMutation({
    mutationFn: async (input: { current_pin: string; new_pin: string }) => {
      const { data } = await api.put<{ data: { message: string } }>("/api/users/me/pin", input);
      return data.data;
    },
  });
}

// POST /api/users/phone  { phone_number }  -> { data: PhoneVerificationStatus }
export function useAddPhone() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (phone_number: string) => {
      const { data } = await api.post<{ data: PhoneVerificationStatus }>("/api/users/phone", {
        phone_number,
      });
      return data.data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["phone-verification"] });
    },
  });
}

// PUT /api/users/me/pin/reset  { new_pin }  -> { data: { message: string } }
export function useResetPin() {
  return useMutation({
    mutationFn: async (new_pin: string) => {
      const { data } = await api.put<{ data: { message: string } }>(
        "/api/users/me/pin/reset",
        { new_pin }
      );
      return data.data;
    },
  });
}

// GET /api/users/phone-verification -> { data: PhoneVerificationStatus }
export function usePhoneVerificationStatus() {
  return useQuery({
    queryKey: ["phone-verification"],
    queryFn: async () => {
      const { data } = await api.get<{ data: PhoneVerificationStatus }>(
        "/api/users/phone-verification"
      );
      return data.data;
    },
    refetchInterval: (query) => {
      const status = query.state.data?.verification?.status;
      return status === "initiated" || status === "pending" ? 5_000 : false;
    },
  });
}
