"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import Image from "next/image";
import { useParams, useRouter } from "next/navigation";
import { useLogin } from "@/hooks/use-auth";
import { useCompanyBySlug } from "@/hooks/use-company";
import { setTokens, setSubjectHint } from "@/lib/auth";
import { useAuth } from "@/components/providers";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
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

export default function EmployeeLoginPage() {
  const router = useRouter();
  const { company } = useParams<{ company: string }>();
  const { user, refreshSubject } = useAuth();
  const { data: companyInfo, isLoading: companyLoading, isError: companyNotFound } =
    useCompanyBySlug(company);
  const login = useLogin();
  const [email, setEmail] = useState("");
  const [pin, setPin] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    if (user) {
      router.replace(`/${company}`);
    }
  }, [user, company, router]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");

    if (!/^\d{5}$/.test(pin)) {
      setError("PIN must be exactly 5 digits");
      return;
    }

    try {
      const result = await login.mutateAsync({ email, pin, company_slug: company });
      setTokens(result.access_token, result.refresh_token);
      setSubjectHint("user");
      await refreshSubject();
      toast.success("Signed in successfully");
      router.push(`/${company}`);
    } catch (err: unknown) {
      const data =
        err && typeof err === "object" && "response" in err
          ? (err as { response?: { data?: { error?: string; code?: string } } }).response?.data
          : undefined;

      let message: string;
      switch (data?.code) {
        case "invalid_credentials":
          message = "Invalid email or PIN";
          break;
        case "account_suspended":
          message = "Your account has been suspended.";
          break;
        case "company_suspended":
          message = "Your company account is suspended. Please contact support.";
          break;
        case "account_locked":
        case "too_many_attempts":
          message = data.error || "Failed to sign in";
          break;
        default:
          message = "Failed to sign in";
      }
      setError(message);
      toast.error("Sign in failed");
    }
  };

  if (companyLoading) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center bg-muted/50 p-4">
        <div className="mb-6 flex flex-row items-center gap-2">
          <Image src="/logo.png" alt="Bohikor" width={24} height={24} className="rounded-md" />
          <span className="text-xs font-semibold tracking-wide text-muted-foreground">BOHIKOR</span>
        </div>
      </div>
    );
  }

  if (companyNotFound) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center bg-muted/50 p-4">
        <div className="mb-6 flex flex-row items-center gap-2">
          <Image src="/logo.png" alt="Bohikor" width={24} height={24} className="rounded-md" />
          <span className="text-xs font-semibold tracking-wide text-muted-foreground">BOHIKOR</span>
        </div>
        <Card className="w-full max-w-md animate-fade-up py-6">
          <CardHeader className="space-y-1 px-6">
            <CardTitle className="font-heading text-2xl font-bold">Company not found</CardTitle>
            <CardDescription>
              We couldn&apos;t find a company at &ldquo;{company}&rdquo;. Check the link your
              employer sent you, or contact them for the correct sign-in page.
            </CardDescription>
          </CardHeader>
        </Card>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-muted/50 p-4">
      <div className="mb-6 flex flex-row items-center gap-2">
        <Image src="/logo.png" alt="Bohikor" width={24} height={24} className="rounded-md" />
        <span className="text-xs font-semibold tracking-wide text-muted-foreground">BOHIKOR</span>
      </div>

      <Card className="w-full max-w-md animate-fade-up py-6">
        <CardHeader className="space-y-1 px-6">
          <CardTitle className="font-heading text-2xl font-bold">{companyInfo?.name ?? company}</CardTitle>
          <CardDescription>Sign in with your email and PIN</CardDescription>
        </CardHeader>
        <CardContent className="px-6">
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

            <div className="space-y-2">
              <Label htmlFor="pin">PIN</Label>
              <PasswordInput
                id="pin"
                inputMode="numeric"
                maxLength={5}
                placeholder="•••••"
                value={pin}
                onChange={(e) => setPin(e.target.value)}
                required
                autoComplete="current-password"
              />
            </div>

            <Button type="submit" className="w-full" disabled={login.isPending}>
              {login.isPending ? "Signing in..." : "Sign In"}
            </Button>
          </form>
        </CardContent>
      </Card>

      <div className="mt-4 flex flex-col items-center gap-2 text-sm">
        <Link
          href={`/${company}/forgot-pin`}
          className="text-muted-foreground underline-offset-4 hover:text-primary hover:underline"
        >
          Forgot your PIN?
        </Link>
        <div className="flex items-center gap-2">
          <Link
            href={`/${company}/signup`}
            className="text-muted-foreground underline-offset-4 hover:text-primary hover:underline"
          >
            New here? Sign up
          </Link>
          <span className="text-muted-foreground">·</span>
          <Link
            href={`/${company}/admin/login`}
            className="text-muted-foreground underline-offset-4 hover:text-primary hover:underline"
          >
            Company admin? Sign in
          </Link>
        </div>
      </div>
    </div>
  );
}
