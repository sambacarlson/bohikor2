"use client";

import { useState } from "react";
import Link from "next/link";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import { useCheckInvite, useSendEmailOtp } from "@/hooks/use-auth";
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
import { toast } from "sonner";

const NO_INVITATION_MSG = "No invitation found for this email. Contact your manager.";

export default function SignupPage() {
  const router = useRouter();
  const { company } = useParams<{ company: string }>();
  const searchParams = useSearchParams();
  const [email, setEmail] = useState(searchParams.get("email") ?? "");
  const [error, setError] = useState("");
  const [checking, setChecking] = useState(false);

  const checkInvite = useCheckInvite(email, false);
  const sendOtp = useSendEmailOtp();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");

    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
      setError("Enter a valid email address");
      return;
    }

    setChecking(true);
    try {
      const result = await checkInvite.refetch();
      const status = result.data?.status;

      if (status === "accepted") {
        toast.info("You already have an account — please sign in.");
        router.push(`/${company}/login`);
        return;
      }

      if (status === "pending" || status === "sent") {
        await sendOtp.mutateAsync(email);
        router.push(
          `/${company}/verify?email=${encodeURIComponent(email)}&purpose=signup`
        );
        return;
      }

      // revoked / failed — defensive, check-invite normally only succeeds for
      // pending | sent | accepted.
      setError(NO_INVITATION_MSG);
    } catch {
      setError(NO_INVITATION_MSG);
    } finally {
      setChecking(false);
    }
  };

  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-muted/50 p-4">
      <Card className="w-full max-w-md">
        <CardHeader className="space-y-1">
          <CardTitle className="text-2xl font-bold">Create your account</CardTitle>
          <CardDescription>
            Enter the email address your employer invited you with.
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

            <Button type="submit" className="w-full" disabled={checking || sendOtp.isPending}>
              {checking || sendOtp.isPending ? "Checking..." : "Continue"}
            </Button>
          </form>
        </CardContent>
      </Card>

      <Link
        href={`/${company}/login`}
        className="mt-4 text-sm text-muted-foreground underline-offset-4 hover:text-primary hover:underline"
      >
        Already have an account? Sign in
      </Link>
    </div>
  );
}
