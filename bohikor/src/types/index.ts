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
  status: UserStatus;
  is_terms_accepted: boolean;
  terms_accepted_at: string | null;
  terms_version: string | null;
  user_ip_at_consent: string | null;
  pin_hash: string | null;
  failed_login_attempts: number;
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

export interface Admin {
  id: string;
  email: string;
  company_slug: string;
  created_at: string;
}

export interface AdvanceRequest {
  id: string;
  user_id: string;
  user_email?: string;
  amount_xaf: string;
  status: RequestStatus;
  campay_payout_ref: string | null;
  failure_reason: string | null;
  payout_duration_seconds: number | null;
  created_at: string;
  updated_at: string;
}