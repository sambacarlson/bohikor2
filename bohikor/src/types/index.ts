export type UserStatus = "active" | "suspended" | "locked";
export type InvitationStatus = "pending" | "sent" | "accepted" | "revoked" | "failed";
export type RequestStatus = "initiated" | "processing" | "pending" | "success" | "failed";

// The employee record, shaped exactly like backend/internal/handler/auth.go's
// sanitizeUser() allowlist (does NOT include pin_hash or lockout fields).
export interface User {
  id: string;
  email: string;
  email_verified: boolean;
  full_name: string | null;
  phone_number: string | null;
  phone_verified: boolean;
  status: UserStatus;
  is_terms_accepted: boolean;
  // Deliberately `unknown`, not `string | null`: the backend's `sql.NullTime` wire shape is
  // `{ Time: string; Valid: boolean }`, never a plain string. Don't widen this and don't render
  // it; use `is_terms_accepted` for any accepted/not-accepted UI instead.
  terms_accepted_at: unknown;
  terms_version: string | null;
  created_at: string;
  updated_at: string;
}

// The raw user row as returned by the company-admin endpoints (GET /api/admin/users and the
// suspend/activate/unlock writes), which serialize the full db.User struct including lockout
// fields — unlike the employee-facing sanitizeUser() shape above.
export interface AdminUser {
  id: string;
  company_id?: string;
  email: string;
  email_verified: boolean;
  full_name: string | null;
  phone_number: string | null;
  phone_verified: boolean;
  pin_hash: string | null;
  failed_login_attempts: number;
  locked_until: string | null;
  status: UserStatus;
  is_terms_accepted: boolean;
  terms_accepted_at: unknown;
  terms_version: string | null;
  user_ip_at_consent: string | null;
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

export interface AuthResponse {
  user: User;
  company_slug?: string; // present on POST /api/auth/login only; absent elsewhere
  access_token: string;
  refresh_token: string;
  expires_in: number;
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

export interface PhoneVerificationStatus {
  phone_number: string | null;
  phone_verified: boolean;
  verification: {
    id: string;
    status: RequestStatus;
    created_at: string;
    ussd_code?: string;
  } | null;
}

export interface AdvanceRequest {
  id: string;
  company_id?: string;
  user_id: string;
  user_email?: string;
  amount_xaf: string;
  status: RequestStatus;
  campay_payout_ref: string | null;
  failure_reason: string | null;
  payout_duration_seconds: number | null;
  attempt_count?: number;
  // Both deliberately `unknown`, not `string | null` — same sql.NullTime wire-shape gotcha as
  // User.terms_accepted_at above. Not rendered anywhere in this plan.
  last_reconciled_at?: unknown;
  next_retry_at?: unknown;
  needs_admin_review?: boolean;
  reissued_from_id?: string | null;
  created_at: string;
  updated_at: string;
}

// Company, as returned by every /api/platform/companies* endpoint.
export type CompanyStatus = "active" | "suspended";

export interface Company {
  id: string;
  slug: string;
  name: string;
  status: CompanyStatus;
  balance_xaf: string;
  created_at: string;
  updated_at: string;
}

// One row from GET /api/admin/ledger's "entries" array.
export type LedgerEntryType = "topup" | "payout_debit" | "reversal" | "adjustment";

export interface LedgerEntry {
  id: string;
  company_id: string;
  entry_type: LedgerEntryType;
  amount_xaf: string; // signed: topup/reversal positive, payout_debit negative
  advance_request_id: string | null;
  created_by: string | null;
  note: string | null;
  created_at: string;
}

// GET /api/admin/ledger response shape
export interface LedgerResponse {
  balance_xaf: string;
  entries: LedgerEntry[];
}

// One row from GET /api/platform/requests/needs-review
export interface RequestNeedingReview extends AdvanceRequest {
  company_slug: string;
  company_name: string;
}

// One row from GET /api/platform/requests/health
export interface CompanyRequestHealth {
  company_id: string;
  company_slug: string;
  company_name: string;
  processing_count: number;
  pending_count: number;
  needs_review_count: number;
}
