import { useMutation } from "@tanstack/react-query";
import { api } from "@/src/lib/api";

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
    mutationFn: async ({ email, code }: { email: string; code: string }) => {
      await api.post("/api/auth/verify-email-otp", { email, code });
    },
  });
}

export function useSendPhoneOTP() {
  return useMutation({
    mutationFn: async (phoneNumber: string) => {
      await api.post("/api/auth/send-phone-otp", { phone_number: phoneNumber });
    },
  });
}