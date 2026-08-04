"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/components/providers";

export function AuthGuard({
  children,
  loginHref,
}: {
  children: React.ReactNode;
  loginHref: string;
}) {
  const { subjectType, loading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!loading && !subjectType) {
      router.push(loginHref);
    }
  }, [subjectType, loading, loginHref, router]);

  if (loading) {
    return (
      <div className="flex h-screen items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
      </div>
    );
  }

  if (!subjectType) {
    return null;
  }

  return <>{children}</>;
}
