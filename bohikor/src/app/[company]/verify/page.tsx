"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import { useForgotPin, useSendEmailOtp, useVerifyEmailOtp } from "@/hooks/use-auth";
import { setSubjectHint, setTokens } from "@/lib/auth";
import { useAuth } from "@/components/providers";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Alert, AlertDescription } from "@/components/ui/alert";

const RESEND_SECONDS = 60;

function verifyError(err: unknown): string {
  const response = (err as { response?: { data?: { code?: string; error?: string } } })
    .response;
  const code = response?.data?.code;
  if (code === "invalid_otp") return "Invalid OTP code";
  if (code === "otp_permanently_blocked" || code === "otp_temporarily_blocked") {
    return response?.data?.error ?? "Verification failed. Please try again.";
  }
  return "Verification failed. Please try again.";
}

export default function VerifyPage() {
  const router = useRouter();
  const { company } = useParams<{ company: string }>();
  const searchParams = useSearchParams();
  const email = searchParams.get("email") ?? "";
  const purpose = searchParams.get("purpose") === "pin_reset" ? "pin_reset" : "signup";

  const [code, setCode] = useState("");
  const [error, setError] = useState("");
  const [resendIn, setResendIn] = useState(RESEND_SECONDS);

  const verifyOtp = useVerifyEmailOtp();
  const sendOtp = useSendEmailOtp();
  const forgotPin = useForgotPin();
  const { refreshSubject } = useAuth();

  useEffect(() => {
    if (resendIn === 0) return;
    const t = setTimeout(() => setResendIn(resendIn - 1), 1000);
    return () => clearTimeout(t);
  }, [resendIn]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");

    if (!/^\d{6}$/.test(code)) {
      setError("Enter the 6-digit code");
      return;
    }

    try {
      const result = await verifyOtp.mutateAsync({ email, code, purpose });
      if (purpose === "signup") {
        router.push(`/${company}/create-pin?email=${encodeURIComponent(email)}`);
      } else {
        if (result) {
          setTokens(result.access_token, result.refresh_token);
          setSubjectHint("user");
          await refreshSubject();
        }
        router.push(`/${company}/reset-pin`);
      }
    } catch (err) {
      setError(verifyError(err));
    }
  };

  const handleResend = async () => {
    setError("");
    try {
      if (purpose === "signup") {
        await sendOtp.mutateAsync(email);
      } else {
        await forgotPin.mutateAsync(email);
      }
      setResendIn(RESEND_SECONDS);
    } catch {
      setError("Couldn't resend the code. Please try again.");
    }
  };

  if (!email) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-muted/50 p-4">
        <Card className="w-full max-w-md">
          <CardHeader className="space-y-1">
            <CardTitle className="text-2xl font-bold">Verify your email</CardTitle>
            <CardDescription>
              We couldn&apos;t load your verification details. Please try again.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-2">
            <Link
              href={`/${company}/login`}
              className="text-sm text-muted-foreground underline-offset-4 hover:text-primary hover:underline"
            >
              Back to sign in
            </Link>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-muted/50 p-4">
      <Card className="w-full max-w-md">
        <CardHeader className="space-y-1">
          <CardTitle className="text-2xl font-bold">Verify your email</CardTitle>
          <CardDescription>
            Enter the 6-digit code we sent to <span className="font-medium">{email}</span>.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            {error && (
              <Alert variant="destructive">
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            )}

            <div className="space-y-2">
              <Label htmlFor="code">Verification code</Label>
              <Input
                id="code"
                type="text"
                inputMode="numeric"
                maxLength={6}
                placeholder="000000"
                value={code}
                onChange={(e) => setCode(e.target.value.replace(/\D/g, ""))}
                required
                autoComplete="one-time-code"
              />
            </div>

            <Button
              type="submit"
              className="w-full"
              disabled={verifyOtp.isPending || sendOtp.isPending || forgotPin.isPending}
            >
              {verifyOtp.isPending ? "Verifying..." : "Verify"}
            </Button>

            <Button
              type="button"
              variant="ghost"
              className="w-full"
              disabled={resendIn > 0 || sendOtp.isPending || forgotPin.isPending}
              onClick={handleResend}
            >
              {resendIn > 0 ? `Resend code in ${resendIn}s` : "Resend code"}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
