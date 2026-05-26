"use client";

import { useState, useRef, useCallback, useEffect } from "react";
import { useRouter } from "next/navigation";
import {
  RecaptchaVerifier,
  signInWithPhoneNumber,
  type ConfirmationResult,
} from "firebase/auth";
import { auth } from "@/lib/firebase";
import { useUser } from "@/hooks/use-user";
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
  const { user: firebaseUser, loading: authLoading } = useAuth();
  const [step, setStep] = useState<"phone" | "otp">("phone");
  const [phone, setPhone] = useState("");
  const [otp, setOtp] = useState("");
  const [error, setError] = useState("");
  const [sendingCode, setSendingCode] = useState(false);
  const [verifying, setVerifying] = useState(false);
  const [verifiedPhone, setVerifiedPhone] = useState("");
  const recaptchaRef = useRef<HTMLDivElement>(null);
  const verifierRef = useRef<RecaptchaVerifier | null>(null);
  const confirmationRef = useRef<ConfirmationResult | null>(null);
  const firebaseReady = !authLoading && !!firebaseUser;
  const {
    data: user,
    refetch: refetchUser,
    isLoading: userLoading,
    isFetched: userFetched,
  } = useUser(firebaseReady);

  useEffect(() => {
    if (firebaseReady && user) {
      router.replace("/client");
    }
  }, [firebaseReady, user, router]);

  useEffect(() => {
    return () => {
      if (verifierRef.current) {
        try {
          verifierRef.current.clear();
        } catch {
          // ignore cleanup errors
        }
        verifierRef.current = null;
      }
    };
  }, []);

  const getRecaptchaVerifier = useCallback(() => {
    if (!verifierRef.current && recaptchaRef.current) {
      verifierRef.current = new RecaptchaVerifier(auth, recaptchaRef.current, {
        size: "invisible",
      });
    }
    return verifierRef.current;
    // auth is a stable module import
  }, []);

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
      const verifier = getRecaptchaVerifier();
      if (!verifier) {
        setError("Failed to initialize. Please refresh the page.");
        setSendingCode(false);
        return;
      }
      const confirmation = await signInWithPhoneNumber(auth, formatted, verifier);
      confirmationRef.current = confirmation;
      setVerifiedPhone(formatted);
      setStep("otp");
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "";
      if (msg.includes("invalid-phone-number")) {
        setError("Invalid phone number. Please check and try again.");
      } else {
        setError("Failed to send verification code. Please try again.");
      }
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
      if (!confirmationRef.current) {
        setError("Verification session expired. Please go back.");
        setVerifying(false);
        return;
      }

      await confirmationRef.current.confirm(otp);
      const result = await refetchUser();

      if (result.data) {
        router.replace("/client");
      } else {
        setError("Account not found. Please sign up using the mobile app.");
      }
    } catch (err: unknown) {
      const code = (err as { code?: string } | undefined)?.code;
      if (code === "auth/invalid-verification-code") {
        setError("Invalid code. Please try again.");
      } else if (code === "auth/code-expired") {
        setError("Code expired. Please request a new one.");
      } else if (code === "auth/too-many-requests") {
        setError("Too many attempts. Please wait and try again.");
      } else {
        setError("Something went wrong. Please try again.");
      }
      setOtp("");
    } finally {
      setVerifying(false);
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-muted/50 p-4">
      <div id="recaptcha-container" ref={recaptchaRef} />

      {firebaseReady && userFetched && !user && !userLoading ? (
        <Card className="w-full max-w-md">
          <CardHeader className="space-y-1">
            <CardTitle className="text-2xl font-bold">Bohikor2</CardTitle>
            <CardDescription>Salary Advance Pilot</CardDescription>
          </CardHeader>
          <CardContent className="text-center space-y-4">
            <p className="text-muted-foreground">
              Account not found. Please sign up using the mobile app.
            </p>
            <Button
              variant="outline"
              onClick={async () => {
                await auth.signOut();
                window.location.reload();
              }}
            >
              Go Back
            </Button>
          </CardContent>
        </Card>
      ) : firebaseReady && userLoading ? (
        <div className="flex items-center justify-center">
          <Loader2 className="h-8 w-8 animate-spin text-primary" />
        </div>
      ) : (
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
                  <span className="font-medium">{verifiedPhone}</span>
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
      )}
    </div>
  );
}
