"use client";

import { useState } from "react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useForgotPin } from "@/hooks/use-auth";
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

export default function ForgotPinPage() {
  const router = useRouter();
  const { company } = useParams<{ company: string }>();
  const [email, setEmail] = useState("");
  const [error, setError] = useState("");

  const forgotPin = useForgotPin();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");

    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
      setError("Please enter a valid email address");
      return;
    }

    try {
      await forgotPin.mutateAsync(email);
      router.push(`/${company}/verify?email=${encodeURIComponent(email)}&purpose=pin_reset`);
    } catch (err) {
      const data = (err as { response?: { data?: { code?: string; error?: string } } })
        .response?.data;
      if (data?.code === "not_found") {
        setError("No account found with this email");
      } else if (data?.code === "account_locked" && data.error) {
        setError(data.error);
      } else {
        setError("Failed to send reset code. Please try again.");
      }
    }
  };

  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-muted/50 p-4">
      <Card className="w-full max-w-md">
        <CardHeader className="space-y-1">
          <CardTitle className="text-2xl font-bold">Forgot PIN?</CardTitle>
          <CardDescription>
            Enter your email and we&apos;ll send you a verification code to reset your PIN.
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
              <Label htmlFor="email">Email</Label>
              <Input
                id="email"
                type="email"
                placeholder="you@company.com"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                autoComplete="email"
              />
            </div>

            <Button type="submit" className="w-full" disabled={forgotPin.isPending}>
              {forgotPin.isPending ? "Sending..." : "Send Reset Code"}
            </Button>
          </form>
        </CardContent>
      </Card>

      <Link
        href={`/${company}/login`}
        className="mt-4 text-sm text-muted-foreground underline-offset-4 hover:text-primary hover:underline"
      >
        Back to Login
      </Link>
    </div>
  );
}
