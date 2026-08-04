"use client";

import { AuthGuard } from "@/components/auth-guard";
import { useAuth } from "@/components/providers";
import { ForbiddenPage } from "@/components/forbidden";

export default function PlatformLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <AuthGuard loginHref="/platform/login">
      <PlatformAdminCheck>{children}</PlatformAdminCheck>
    </AuthGuard>
  );
}

function PlatformAdminCheck({ children }: { children: React.ReactNode }) {
  const { subjectType } = useAuth();

  if (subjectType !== "platform_admin") {
    return <ForbiddenPage backHref="/platform/login" backLabel="Go to Login" />;
  }

  return <>{children}</>;
}
