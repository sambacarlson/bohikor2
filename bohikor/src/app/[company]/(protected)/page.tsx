"use client";

import { useState } from "react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { motion } from "motion/react";
import { Check, RefreshCw } from "lucide-react";
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
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { extractApiError } from "@/lib/errors";

const fadeUp = {
  initial: { opacity: 0, y: 8 },
  animate: { opacity: 1, y: 0 },
};

export default function EmployeeHomePage() {
  const router = useRouter();
  const { company } = useParams<{ company: string }>();
  const { user } = useAuth();
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

  return (
    <div className="mx-auto max-w-[680px] space-y-5 lg:mx-0">
      <div>
        <p className="text-sm text-muted-foreground">Good to see you,</p>
        <h1 className="font-heading text-2xl font-bold">{displayName}</h1>
      </div>

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

      <div className="grid gap-4 lg:grid-cols-[300px_1fr] lg:items-start">
        <motion.div
          {...fadeUp}
          transition={{ duration: 0.4, ease: "easeOut" }}
          className="rounded-2xl border border-border-strong p-6"
          style={{
            background:
              "linear-gradient(160deg, var(--card-elevated), var(--card))",
          }}
        >
          <div className="mb-4 flex items-center justify-between">
            <span className="text-xs font-medium tracking-wide text-muted-foreground">
              AVAILABLE TO REQUEST
            </span>
            {eligibility && (
              <Badge variant={eligibility.eligible ? "default" : "destructive"}>
                {eligibility.eligible ? "Eligible" : "Not eligible"}
              </Badge>
            )}
          </div>

          <div className="font-heading mb-5 text-[2rem] leading-none font-extrabold tabular-nums">
            {advanceAmount}
          </div>

          <motion.button
            onClick={handleRequestAdvance}
            animate={
              readyToRequest
                ? {
                    boxShadow: [
                      "0 4px 20px -4px rgba(34,197,94,0.35)",
                      "0 4px 28px -2px rgba(34,197,94,0.55)",
                      "0 4px 20px -4px rgba(34,197,94,0.35)",
                    ],
                  }
                : undefined
            }
            whileHover={{ y: -2 }}
            transition={
              readyToRequest
                ? { duration: 2.8, repeat: Infinity, ease: "easeInOut" }
                : undefined
            }
            className={`w-full rounded-xl py-3.5 text-sm font-semibold text-primary-foreground ${
              readyToRequest ? "opacity-100" : "opacity-70"
            }`}
            style={{
              background: "linear-gradient(135deg, #22c55e, #15803d)",
            }}
          >
            Request Advance
          </motion.button>
        </motion.div>

        <div className="space-y-4">
          <motion.div {...fadeUp} transition={{ duration: 0.4, delay: 0.08, ease: "easeOut" }}>
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
                      <span className="text-muted-foreground">Remaining</span>
                      <span className="tabular-nums">
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
          </motion.div>

          <motion.div {...fadeUp} transition={{ duration: 0.4, delay: 0.16, ease: "easeOut" }}>
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
          </motion.div>

          <motion.div {...fadeUp} transition={{ duration: 0.4, delay: 0.24, ease: "easeOut" }}>
            <Link
              href={`/${company}/history`}
              className="flex items-center justify-between rounded-2xl border border-border bg-card p-5 text-card-foreground transition-colors hover:bg-accent"
            >
              <span className="font-semibold">View Transaction History</span>
              <span className="text-muted-foreground">→</span>
            </Link>
          </motion.div>
        </div>
      </div>

      <Dialog open={modalVisible} onOpenChange={setModalVisible}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Confirm Advance Request</DialogTitle>
          </DialogHeader>

          <DialogDescription>
            You are about to request a salary advance of{" "}
            <span className="font-semibold text-foreground">{advanceAmount}</span>.
            This amount plus any applicable charges will be deducted from your
            upcoming salary payment. You can only have one active advance request
            at a time.
          </DialogDescription>

          {modalError && <p className="mt-3 text-sm text-destructive">{modalError}</p>}

          <DialogFooter>
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
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
