"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { useResetPin } from "@/hooks/use-user";
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

export default function ResetPinPage() {
  const router = useRouter();
  const { company } = useParams<{ company: string }>();
  const { user, loading, refreshSubject } = useAuth();
  const [newPin, setNewPin] = useState("");
  const [confirmPin, setConfirmPin] = useState("");
  const [error, setError] = useState("");

  const resetPin = useResetPin();

  useEffect(() => {
    if (!loading && !user) {
      router.replace(`/${company}/login`);
    }
  }, [loading, user, company, router]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");

    if (!/^\d{5}$/.test(newPin) || !/^\d{5}$/.test(confirmPin)) {
      setError("PIN must be 5 digits");
      return;
    }
    if (newPin !== confirmPin) {
      setError("PINs do not match");
      return;
    }

    try {
      await resetPin.mutateAsync(newPin);
      await refreshSubject();
      toast.success("PIN reset");
      router.push(`/${company}`);
    } catch {
      setError("Failed to reset PIN. Please try again.");
    }
  };

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center p-4">
        <p className="text-muted-foreground">Loading...</p>
      </div>
    );
  }

  if (!user) {
    return null;
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-muted/50 p-4">
      <Card className="w-full max-w-md animate-fade-up">
        <CardHeader className="space-y-1">
          <CardTitle className="font-heading text-2xl font-bold">Set new PIN</CardTitle>
          <CardDescription>
            Choose a new 5-digit PIN for your account
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
              <Label htmlFor="new-pin">New PIN</Label>
              <PasswordInput
                id="new-pin"
                inputMode="numeric"
                maxLength={5}
                placeholder="00000"
                value={newPin}
                onChange={(e) => setNewPin(e.target.value.replace(/\D/g, ""))}
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

            <Button type="submit" className="w-full" disabled={resetPin.isPending}>
              {resetPin.isPending ? "Saving..." : "Set New PIN"}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
