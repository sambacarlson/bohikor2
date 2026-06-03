import { useMutation, useQuery } from "@tanstack/react-query";
import { api } from "@/src/lib/api";
import type { AuthResponse, PhoneVerificationStatus } from "@/src/types";

export interface CheckInvitationResponse {
  has_invitation: boolean;
  status: string;
}

export function useCheckInvitation() {
  return useMutation({
    mutationFn: async (email: string) => {
      const { data } = await api.get<{ data: CheckInvitationResponse }>(
        `/api/auth/check-invite?email=${encodeURIComponent(email)}`
      );
      return data.data;
    },
  });
}

export function useSendEmailOTP() {
  return useMutation({
    mutationFn: async (email: string) => {
      await api.post("/api/auth/send-email-otp", { email });
    },
  });
}

export function useVerifyEmailOTP() {
  return useMutation({
    mutationFn: async ({
      email,
      code,
      purpose,
    }: {
      email: string;
      code: string;
      purpose?: "signup" | "pin_reset";
    }) => {
      const { data } = await api.post<{ data: AuthResponse }>(
        "/api/auth/verify-email-otp",
        { email, code, purpose }
      );
      return data.data;
    },
  });
}

export function useLogin() {
  return useMutation({
    mutationFn: async ({
      email,
      pin,
    }: {
      email: string;
      pin: string;
    }) => {
      const { data } = await api.post<{ data: AuthResponse }>(
        "/api/auth/login",
        { email, pin }
      );
      return data.data;
    },
  });
}

export function useCreatePin() {
  return useMutation({
    mutationFn: async ({
      email,
      pin,
    }: {
      email: string;
      pin: string;
    }) => {
      const { data } = await api.post<{ data: AuthResponse }>(
        "/api/auth/create-pin",
        { email, pin }
      );
      return data.data;
    },
  });
}

export function useForgotPin() {
  return useMutation({
    mutationFn: async (email: string) => {
      await api.post("/api/auth/forgot-pin", { email });
    },
  });
}

export function useChangePin() {
  return useMutation({
    mutationFn: async ({
      current_pin,
      new_pin,
    }: {
      current_pin: string;
      new_pin: string;
    }) => {
      const { data } = await api.put<{ data: { message: string } }>(
        "/api/users/me/pin",
        { current_pin, new_pin }
      );
      return data.data;
    },
  });
}

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

export function useAddPhone() {
  return useMutation({
    mutationFn: async (phone_number: string) => {
      const { data } = await api.post("/api/users/phone", { phone_number });
      return data.data;
    },
  });
}

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
      const verif = query.state.data?.verification;
      if (verif && (verif.status === "initiated" || verif.status === "pending")) {
        return 5000;
      }
      return false;
    },
  });
}
