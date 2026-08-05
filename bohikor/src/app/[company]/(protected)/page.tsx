"use client";

import { useState } from "react";
import Link from "next/link";
import Image from "next/image";
import { useParams, useRouter } from "next/navigation";
import { Check, LogOut, MoreVertical, RefreshCw, User, X } from "lucide-react";
import { useAuth } from "@/components/providers";
import { useEligibility } from "@/hooks/use-eligibility";
import { useCreateAdvanceRequest } from "@/hooks/use-advance-requests";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Alert, AlertDescription } from "@/components/ui/alert";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { extractApiError } from "@/lib/errors";

export default function EmployeeHomePage() {
  const router = useRouter();
  const { company } = useParams<{ company: string }>();
  const { user, signOut, refreshSubject } = useAuth();
  const [modalVisible, setModalVisible] = useState(false);
  const [modalError, setModalError] = useState("");

  const createRequest = useCreateAdvanceRequest();
  const {
    data: eligibility,
    isLoading: eligibilityLoading,
    isError: eligibilityError,
    refetch: refetchEligibility,
    isRefetching: isRefetchingEligibility,
  } = useEligibility();

  if (!user) return null;

  const displayName = user.full_name || user.email;
  const profileIncomplete = !user.phone_verified || !user.phone_number;
  const readyToRequest = user.is_terms_accepted && !profileIncomplete;

  const advanceAmount = eligibility?.advance_amount_xaf
    ? eligibility.advance_amount_xaf.includes("XAF")
      ? eligibility.advance_amount_xaf
      : `${eligibility.advance_amount_xaf} XAF`
    : "10,000 XAF";

  const handleRequestAdvance = () => {
    if (!user.is_terms_accepted) {
      router.push(`/${company}/account?section=terms`);
      return;
    }
    if (profileIncomplete) {
      router.push(`/${company}/account?section=phone`);
      return;
    }
    setModalVisible(true);
  };

  const handleConfirmRequest = async () => {
    setModalError("");
    try {
      await createRequest.mutateAsync();
      setModalVisible(false);
      router.push(`/${company}/history`);
    } catch (err) {
      setModalError(
        extractApiError(err) || "Failed to create request. Please try again."
      );
    }
  };

  const handleSignOut = async () => {
    await signOut();
    await refreshSubject();
    router.push(`/${company}/login`);
  };

  return (
    <div className="min-h-screen bg-muted/50">
      <header className="border-b bg-background">
        <div className="mx-auto flex max-w-3xl items-center justify-between px-6 py-4">
          <div className="flex items-center gap-3">
            <div className="flex items-center gap-2">
              <Image src="/logo.png" alt="" width={24} height={24} />
              <span className="text-xl font-bold">Bohikor</span>
            </div>
            <span className="text-sm text-muted-foreground">{displayName}</span>
          </div>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" aria-label="Account menu">
                <MoreVertical className="h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onClick={() => router.push(`/${company}/account`)}>
                <User className="mr-2 h-4 w-4" />
                Account
              </DropdownMenuItem>
              <DropdownMenuItem onClick={handleSignOut}>
                <LogOut className="mr-2 h-4 w-4" />
                Sign Out
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </header>

      <main className="mx-auto max-w-3xl space-y-6 px-6 py-6">
        <h1 className="text-2xl font-bold">{displayName}</h1>

        {!user.is_terms_accepted && (
          <Alert>
            <AlertDescription className="flex flex-wrap items-center justify-between gap-3">
              <span>
                <strong>You haven&apos;t accepted the terms yet.</strong>{" "}
                <span className="text-muted-foreground">
                  Accept them before requesting an advance.
                </span>
              </span>
              <Button
                size="sm"
                onClick={() => router.push(`/${company}/account?section=terms`)}
              >
                Accept Terms
              </Button>
            </AlertDescription>
          </Alert>
        )}

        {user.is_terms_accepted && profileIncomplete && (
          <Alert>
            <AlertDescription className="flex flex-wrap items-center justify-between gap-3">
              <span>
                {!user.phone_number ? (
                  "Add a phone number to request an advance."
                ) : (
                  "Verify your phone number to request an advance."
                )}
              </span>
              <Button
                size="sm"
                onClick={() => router.push(`/${company}/account?section=phone`)}
              >
                {!user.phone_number ? "Add Phone Number" : "Verify Phone"}
              </Button>
            </AlertDescription>
          </Alert>
        )}

        <Button
          onClick={handleRequestAdvance}
          className={`w-full py-8 text-lg ${readyToRequest ? "" : "opacity-70"}`}
        >
          Request Advance
          <span className="block text-sm font-normal opacity-80">{advanceAmount}</span>
        </Button>

        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle>Eligibility</CardTitle>
              <Button
                variant="ghost"
                size="icon"
                onClick={() => refetchEligibility()}
                disabled={isRefetchingEligibility}
                aria-label="Refresh eligibility"
              >
                <RefreshCw
                  className={`h-4 w-4 ${isRefetchingEligibility ? "animate-spin" : ""}`}
                />
              </Button>
            </div>
          </CardHeader>
          <CardContent className="space-y-4">
            {eligibilityLoading ? (
              <p className="text-muted-foreground">Loading eligibility...</p>
            ) : eligibilityError ? (
              <div className="space-y-3">
                <p className="text-destructive">Failed to load eligibility.</p>
                <Button variant="outline" size="sm" onClick={() => refetchEligibility()}>
                  Retry
                </Button>
              </div>
            ) : eligibility ? (
              <>
                <div className="flex items-center justify-between">
                  <span className="text-muted-foreground">Advance amount</span>
                  <span className="font-semibold">{advanceAmount}</span>
                </div>

                <div className="flex items-center justify-between">
                  <span className="text-muted-foreground">Status</span>
                  {eligibility.eligible ? (
                    <Badge variant="default">Eligible</Badge>
                  ) : (
                    <Badge variant="destructive">Not eligible</Badge>
                  )}
                </div>

                <div className="flex items-center justify-between">
                  <span className="text-muted-foreground">Remaining</span>
                  <span>
                    {eligibility.daily_requests_remaining} daily /{" "}
                    {eligibility.monthly_requests_remaining} monthly
                  </span>
                </div>

                <div className="flex items-center justify-between">
                  <span className="text-muted-foreground">Request window</span>
                  <span>
                    Available days {eligibility.request_window.start_day}–
                    {eligibility.request_window.end_day} of the month
                  </span>
                </div>

                {eligibility.kill_switch_active && (
                  <Alert variant="destructive">
                    <AlertDescription>
                      Advances are temporarily disabled
                    </AlertDescription>
                  </Alert>
                )}

                {!eligibility.eligible && eligibility.reasons.length > 0 && (
                  <ul className="list-disc space-y-1 pl-5 text-sm text-destructive">
                    {eligibility.reasons.map((reason) => (
                      <li key={reason}>{reason}</li>
                    ))}
                  </ul>
                )}
              </>
            ) : null}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Your Information</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3 text-sm">
            <div className="flex items-center justify-between">
              <span className="text-muted-foreground">Email</span>
              <span className="flex items-center gap-2">
                {user.email}
                {user.email_verified ? (
                  <Check className="h-4 w-4 text-green-600" />
                ) : (
                  <span className="text-muted-foreground">—</span>
                )}
              </span>
            </div>

            <div className="flex items-center justify-between">
              <span className="text-muted-foreground">Phone</span>
              <span className="flex items-center gap-2">
                {user.phone_number
                  ? user.phone_number
                  : "Not set"}
                {user.phone_number && user.phone_verified ? (
                  <Check className="h-4 w-4 text-green-600" />
                ) : user.phone_number ? (
                  <span className="text-muted-foreground">Not verified</span>
                ) : null}
              </span>
            </div>

            {user.full_name && (
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">Name</span>
                <span>{user.full_name}</span>
              </div>
            )}

            <div className="flex items-center justify-between">
              <span className="text-muted-foreground">Status</span>
              <span>{user.status}</span>
            </div>

            <div className="flex items-center justify-between">
              <span className="text-muted-foreground">Terms</span>
              <span>{user.is_terms_accepted ? "Accepted" : "Not accepted"}</span>
            </div>
          </CardContent>
        </Card>

        <Link
          href={`/${company}/history`}
          className="flex items-center justify-between rounded-lg border bg-card p-5 text-card-foreground transition-colors hover:bg-accent"
        >
          <span className="font-semibold">View Transaction History</span>
          <span className="text-muted-foreground">→</span>
        </Link>
      </main>

      {modalVisible && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 px-6"
          onClick={() => setModalVisible(false)}
        >
          <div
            className="w-full max-w-sm rounded-xl bg-white p-6"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="mb-2 flex items-center justify-between">
              <h2 className="text-xl font-bold">Confirm Advance Request</h2>
              <button
                onClick={() => setModalVisible(false)}
                aria-label="Close"
                className="text-muted-foreground hover:text-foreground"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            <p className="mb-4 text-sm text-muted-foreground">
              You are about to request a salary advance of{" "}
              <span className="font-semibold text-foreground">{advanceAmount}</span>.
              This amount plus any applicable charges will be deducted from your
              upcoming salary payment. You can only have one active advance request
              at a time.
            </p>

            {modalError && (
              <p className="mb-4 text-sm text-destructive">{modalError}</p>
            )}

            <div className="flex justify-end gap-3">
              <Button
                variant="outline"
                onClick={() => setModalVisible(false)}
                disabled={createRequest.isPending}
              >
                Cancel
              </Button>
              <Button onClick={handleConfirmRequest} disabled={createRequest.isPending}>
                {createRequest.isPending ? "Requesting..." : "Confirm"}
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
