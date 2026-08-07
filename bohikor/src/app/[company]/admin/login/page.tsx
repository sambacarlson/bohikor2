"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import Image from "next/image";
import { useParams, useRouter } from "next/navigation";
import { api } from "@/lib/api";
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

export default function AdminLoginPage() {
  const router = useRouter();
  const { company } = useParams<{ company: string }>();
  const { admin, refreshSubject } = useAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (admin) {
      router.replace(`/${company}/admin`);
    }
  }, [admin, company, router]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setLoading(true);

    try {
      const { data } = await api.post("/api/auth/admin/login", { email, password });
      setTokens(data.data.access_token, data.data.refresh_token);
      setSubjectHint("admin");
      await refreshSubject();
      toast.success("Signed in successfully");
      router.push(`/${company}/admin`);
    } catch (err: unknown) {
      const message =
        err && typeof err === "object" && "response" in err
          ? (err as { response?: { data?: { error?: string } } }).response?.data?.error || "Invalid email or password"
          : "Failed to sign in";
      setError(message);
      toast.error("Sign in failed");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-muted/50 p-4">
      <div className="mb-6 flex flex-row items-center gap-2">
        <Image src="/logo.png" alt="Bohikor" width={24} height={24} className="rounded-md" />
        <span className="text-xs font-semibold tracking-wide text-muted-foreground">BOHIKOR</span>
      </div>

      <Card className="w-full max-w-md animate-fade-up py-6">
        <CardHeader className="space-y-1 px-6">
          <CardTitle className="font-heading text-2xl font-bold">{company}</CardTitle>
          <CardDescription>
            Enter your credentials to access the admin dashboard
          </CardDescription>
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
                placeholder="admin@bohikor2.com"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                autoComplete="email"
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="password">Password</Label>
              <PasswordInput
                id="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                autoComplete="current-password"
              />
            </div>

            <Button type="submit" className="w-full" disabled={loading}>
              {loading ? "Signing in..." : "Sign In"}
            </Button>
          </form>
        </CardContent>
      </Card>

      <Link
        href={`/${company}/login`}
        className="mt-4 text-sm text-muted-foreground underline-offset-4 hover:text-primary hover:underline"
      >
        Employee? Sign in
      </Link>
    </div>
  );
}
