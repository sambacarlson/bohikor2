"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/components/providers";
import { useUser } from "@/hooks/use-user";
import { ForbiddenPage } from "@/components/forbidden";
import { Loader2 } from "lucide-react";
import { isAxiosError } from "axios";

function isRoleMismatchError(error: unknown): boolean {
  if (!isAxiosError(error)) return false;
  const status = error.response?.status;
  return status === 403 || status === 404;
}

export default function ClientLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const { subjectType, loading: authLoading } = useAuth();
  const authenticated = !authLoading && !!subjectType;
  const {
    data: backendUser,
    isLoading: userLoading,
    isError: userError,
    error,
  } = useUser(subjectType === "user");
  const router = useRouter();

  useEffect(() => {
    if (!authLoading && !subjectType) {
      router.replace("/login");
    }
  }, [subjectType, authLoading, router]);

  if (authLoading || (authenticated && subjectType === "user" && userLoading)) {
    return (
      <div className="flex h-screen items-center justify-center">
        <Loader2 className="h-8 w-8 animate-spin text-primary" />
      </div>
    );
  }

  if (!subjectType) {
    return null;
  }

  if (subjectType === "admin") {
    return <ForbiddenPage backHref="/login/admin" backLabel="Go to Admin Login" />;
  }

  if (userError && isRoleMismatchError(error)) {
    return <ForbiddenPage backHref="/login" backLabel="Go to Login" />;
  }

  if (userError) {
    return (
      <div className="flex h-screen items-center justify-center p-4">
        <div className="text-center max-w-md">
          <h1 className="text-xl font-bold mb-2">Something Went Wrong</h1>
          <p className="text-muted-foreground mb-4">
            Failed to load your account. Please check your connection and try again.
          </p>
          <button
            onClick={() => window.location.reload()}
            className="text-sm text-primary hover:underline"
          >
            Retry
          </button>
        </div>
      </div>
    );
  }

  if (!backendUser) {
    return (
      <div className="flex h-screen items-center justify-center p-4">
        <div className="text-center max-w-md">
          <h1 className="text-xl font-bold mb-2">Account Not Found</h1>
          <p className="text-muted-foreground">
            Your account was not found. Please sign up using the mobile app or contact support.
          </p>
        </div>
      </div>
    );
  }

  if (backendUser.status === "suspended") {
    return (
      <div className="flex h-screen items-center justify-center p-4">
        <div className="text-center max-w-md">
          <h1 className="text-2xl font-bold text-gray-900 mb-2">Account Suspended</h1>
          <p className="text-muted-foreground">
            Your account has been suspended. Please contact your manager.
          </p>
        </div>
      </div>
    );
  }

  return <>{children}</>;
}