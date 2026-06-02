"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api";
import { setTokens } from "@/lib/auth";
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
import { Loader2 } from "lucide-react";
import Link from "next/link";

export default function LoginPage() {
  const router = useRouter();
const { user, refreshSubject } = useAuth();
  const [step, setStep] = useState<"phone" | "otp">("phone");
  const [phone, setPhone] = useState("");
  const [otp, setOtp] = useState("");
  const [error, setError] = useState("");
  const [sendingCode, setSendingCode] = useState(false);
  const [verifying, setVerifying] = useState(false);

  useEffect(() => {
    if (user) {
      router.replace("/client");
    }
  }, [user, router]);

  const formatPhone = (value: string) => {
    const digits = value.replace(/\D/g, "");
    if (digits.startsWith("237")) {
      return "+" + digits;
    }
    return "+237" + digits;
  };

  const handleSendCode = async () => {
    setError("");
    const formatted = formatPhone(phone);
    if (formatted.length < 10) {
      setError("Enter a valid phone number");
      return;
    }

    setSendingCode(true);
    try {
      await api.post("/api/auth/send-phone-otp", { phone_number: formatted });
      setStep("otp");
    } catch (err: unknown) {
      const msg =
        err && typeof err === "object" && "response" in err
          ? (err as { response?: { data?: { error?: string } } }).response?.data?.error || "Failed to send verification code."
          : "Network error. Please try again.";
      setError(msg);
    } finally {
      setSendingCode(false);
    }
  };

  const handleVerify = async () => {
    setError("");
    if (otp.length !== 6) {
      setError("Please enter the full 6-digit code");
      return;
    }

    setVerifying(true);
    try {
      const formatted = formatPhone(phone);
      const { data } = await api.post("/api/auth/verify-phone-otp", {
        phone_number: formatted,
        code: otp,
      });
      await setTokens(data.data.access_token, data.data.refresh_token);
      await refreshSubject();
      router.replace("/client");
    } catch (err: unknown) {
      const msg =
        err && typeof err === "object" && "response" in err
          ? (err as { response?: { data?: { error?: string } } }).response?.data?.error || "Invalid code. Please try again."
          : "Network error. Please try again.";
      setError(msg);
      setOtp("");
    } finally {
      setVerifying(false);
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-muted/50 p-4">
      <Card className="w-full max-w-md">
        <CardHeader className="space-y-1">
          <CardTitle className="text-2xl font-bold">Bohikor2</CardTitle>
          <CardDescription>Salary Advance Pilot</CardDescription>
        </CardHeader>
        <CardContent>
          {step === "phone" ? (
            <div className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="phone">Phone Number</Label>
                <Input
                  id="phone"
                  type="tel"
                  placeholder="e.g. 671234567"
                  value={phone}
                  onChange={(e) => {
                    setPhone(e.target.value);
                    setError("");
                  }}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") handleSendCode();
                  }}
                  autoComplete="tel"
                  autoFocus
                />
                <p className="text-xs text-muted-foreground">
                  Enter your Cameroon phone number (e.g., 671234567 or 237671234567)
                </p>
              </div>

              {error && (
                <Alert variant="destructive">
                  <AlertDescription>{error}</AlertDescription>
                </Alert>
              )}

              <Button
                className="w-full"
                onClick={handleSendCode}
                disabled={sendingCode || !phone.trim()}
              >
                {sendingCode && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                Continue
              </Button>

              <div className="relative my-4">
                <div className="absolute inset-0 flex items-center">
                  <span className="w-full border-t" />
                </div>
                <div className="relative flex justify-center text-xs uppercase">
                  <span className="bg-card px-2 text-muted-foreground">or</span>
                </div>
              </div>

              <div className="text-center">
                <Link
                  href="/login/admin"
                  className="text-sm text-muted-foreground hover:text-primary underline"
                >
                  Login as admin
                </Link>
              </div>
            </div>
          ) : (
            <div className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="otp">Verification Code</Label>
                <p className="text-sm text-muted-foreground">
                  Enter the 6-digit code sent to{" "}
                  <span className="font-medium">{formatPhone(phone)}</span>
                </p>
                <Input
                  id="otp"
                  type="text"
                  inputMode="numeric"
                  placeholder="000000"
                  maxLength={6}
                  value={otp}
                  onChange={(e) => {
                    setOtp(e.target.value.replace(/\D/g, ""));
                    setError("");
                  }}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") handleVerify();
                  }}
                  autoComplete="one-time-code"
                  autoFocus
                  className="text-center text-2xl tracking-widest"
                />
              </div>

              {error && (
                <Alert variant="destructive">
                  <AlertDescription>{error}</AlertDescription>
                </Alert>
              )}

              <Button
                className="w-full"
                onClick={handleVerify}
                disabled={verifying || otp.length !== 6}
              >
                {verifying && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                Verify
              </Button>

              <div className="text-center">
                <button
                  type="button"
                  onClick={() => {
                    setStep("phone");
                    setOtp("");
                    setError("");
                  }}
                  className="text-sm text-muted-foreground hover:text-primary underline"
                >
                  Change phone number
                </button>
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}