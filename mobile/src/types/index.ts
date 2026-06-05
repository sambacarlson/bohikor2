export type UserStatus = "active" | "suspended" | "locked";

export type InvitationStatus = "pending" | "sent" | "accepted" | "revoked" | "failed";

export type RequestStatus = "initiated" | "pending" | "success" | "failed";

export interface User {
  id: string;
  email: string;
  email_verified: boolean;
  full_name: string | null;
  phone_number: string | null;
  phone_verified: boolean;
  has_pin?: boolean;
  status: UserStatus;
  is_terms_accepted: boolean;
  terms_accepted_at: string | null;
  terms_version: string | null;
  locked_until: string | null;
  created_at: string;
  updated_at: string;
}

export interface Invitation {
  id: string;
  email: string;
  status: InvitationStatus;
  invited_by: string | null;
  sent_at: string;
  accepted_at: string | null;
}

export interface AdvanceRequest {
  id: string;
  user_id: string;
  amount_xaf: string;
  status: RequestStatus;
  campay_payout_ref: string | null;
  failure_reason: string | null;
  payout_duration_seconds: number | null;
  created_at: string;
  updated_at: string;
}

export interface AuthResponse {
  user: User;
  access_token: string;
  refresh_token: string;
  expires_in: number;
}

export interface AdminAuthResponse {
  admin: Admin;
  access_token: string;
  refresh_token: string;
  expires_in: number;
}

export interface PhoneVerificationStatus {
  phone_number: string;
  phone_verified: boolean;
  verification: {
    id: string;
    status: RequestStatus;
    created_at: string;
    ussd_code?: string;
  } | null;
}

export interface Admin {
  id: string;
  email: string;
  password_hash: string;
  created_at: string;
}

export interface RequestWindow {
  start_day: number;
  end_day: number;
  in_window: boolean;
}

export interface EligibilityResponse {
  eligible: boolean;
  reasons: string[];
  kill_switch_active: boolean;
  request_window: RequestWindow;
  daily_requests_remaining: number;
  monthly_requests_remaining: number;
  advance_amount_xaf: string;
  phone_verified: boolean;
  terms_accepted: boolean;
}