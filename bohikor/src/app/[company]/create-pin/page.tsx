"use client";

import { useState } from "react";
import Link from "next/link";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import { useCreatePin } from "@/hooks/use-auth";
import { setSubjectHint, setTokens } from "@/lib/auth";
import { useAuth } from "@/components/providers";
import { Button } from "@/components/ui/button";
import { PasswordInput } from "@/components/ui/password-input";
import { Label } from "@/components/ui/label";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { toast } from "sonner";

export default function CreatePinPage() {
  const router = useRouter();
  const { company } = useParams<{ company: string }>();
  const searchParams = useSearchParams();
  const email = searchParams.get("email") ?? "";

  const [pin, setPin] = useState("");
  const [confirmPin, setConfirmPin] = useState("");
  const [error, setError] = useState("");
  const [userExists, setUserExists] = useState(false);

  const createPin = useCreatePin();
  const { refreshSubject } = useAuth();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setUserExists(false);

    if (!/^\d{5}$/.test(pin) || !/^\d{5}$/.test(confirmPin)) {
      setError("PIN must be 5 digits");
      return;
    }
    if (pin !== confirmPin) {
      setError("PINs do not match");
      return;
    }

    try {
      const result = await createPin.mutateAsync({ email, pin });
      setTokens(result.access_token, result.refresh_token);
      setSubjectHint("user");
      await refreshSubject();
      toast.success("Account created");
      router.push(`/${company}`);
    } catch (err) {
      const code = (err as { response?: { data?: { code?: string; error?: string } } })
        .response?.data?.code;
      if (code === "user_exists") {
        setUserExists(true);
      } else if (code === "no_invitation") {
        setError("No invitation found for this email. Contact your manager.");
      } else if (code === "invalid_pin") {
        setError("PIN must be 5 digits.");
      } else {
        setError("Couldn't create your account. Please try again.");
      }
    }
  };

  if (!email) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-muted/50 p-4">
        <Card className="w-full max-w-md animate-fade-up">
          <CardHeader className="space-y-1">
            <CardTitle className="font-heading text-2xl font-bold">Create your PIN</CardTitle>
            <CardDescription>
              We couldn&apos;t load your sign-up details. Please try again.
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
      <Card className="w-full max-w-md animate-fade-up">
        <CardHeader className="space-y-1">
          <CardTitle className="font-heading text-2xl font-bold">Create your PIN</CardTitle>
          <CardDescription>
            Choose a 5-digit PIN to secure your account. You&apos;ll use it to sign in.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            {error && (
              <Alert variant="destructive">
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            )}

            {userExists && (
              <Alert variant="destructive">
                <AlertDescription>
                  An account with this email already exists.{" "}
                  <Link
                    href={`/${company}/login`}
                    className="font-medium underline underline-offset-4"
                  >
                    Please log in instead.
                  </Link>
                </AlertDescription>
              </Alert>
            )}

            <div className="space-y-2">
              <Label htmlFor="pin">PIN</Label>
              <PasswordInput
                id="pin"
                inputMode="numeric"
                maxLength={5}
                placeholder="00000"
                value={pin}
                onChange={(e) => setPin(e.target.value.replace(/\D/g, ""))}
                required
                autoComplete="new-password"
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="confirm-pin">Confirm PIN</Label>
              <PasswordInput
                id="confirm-pin"
                inputMode="numeric"
                maxLength={5}
                placeholder="00000"
                value={confirmPin}
                onChange={(e) => setConfirmPin(e.target.value.replace(/\D/g, ""))}
                required
                autoComplete="new-password"
              />
            </div>

            <Button type="submit" className="w-full" disabled={createPin.isPending}>
              {createPin.isPending ? "Creating..." : "Create account"}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
