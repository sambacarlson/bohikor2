"use client";

import { useAuth } from "@/components/providers";
import { useAdmin } from "@/hooks/use-admin";
import { ForbiddenPage } from "@/components/forbidden";
import { useRouter } from "next/navigation";
import { useEffect } from "react";

export function AuthGuard({ children }: { children: React.ReactNode }) {
  const { subjectType, loading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!loading && !subjectType) {
      router.push("/login");
    }
  }, [subjectType, loading, router]);

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

  return (
    <AdminCheck>
      {children}
    </AdminCheck>
  );
}

function AdminCheck({ children }: { children: React.ReactNode }) {
  const { data: admin, isLoading } = useAdmin();

  if (isLoading) {
    return (
      <div className="flex h-screen items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
      </div>
    );
  }

  if (!admin) {
    return <ForbiddenPage backHref="/login" backLabel="Go to Login" />;
  }

  return <>{children}</>;
}
