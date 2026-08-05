"use client";

import { AuthGuard } from "@/components/auth-guard";
import { useAuth } from "@/components/providers";
import { ForbiddenPage } from "@/components/forbidden";
import { PlatformSidebar } from "@/components/platform-sidebar";

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

  return (
    <div className="flex h-screen">
      <PlatformSidebar />
      <main className="flex-1 overflow-y-auto p-8">{children}</main>
    </div>
  );
}
