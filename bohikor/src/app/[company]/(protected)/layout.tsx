"use client";

import { useParams } from "next/navigation";
import { AuthGuard } from "@/components/auth-guard";
import { useAuth } from "@/components/providers";
import { ForbiddenPage } from "@/components/forbidden";
import { EmployeeHeader } from "@/components/employee-header";
import { EmployeeSidebar } from "@/components/employee-sidebar";
import { useMediaQuery } from "@/hooks/use-media-query";

export default function EmployeeProtectedLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const { company } = useParams<{ company: string }>();
  return (
    <AuthGuard loginHref={`/${company}/login`}>
      <UserCheck company={company}>{children}</UserCheck>
    </AuthGuard>
  );
}

function UserCheck({
  children,
  company,
}: {
  children: React.ReactNode;
  company: string;
}) {
  const { user, subjectType } = useAuth();
  const isDesktop = useMediaQuery("(min-width: 1024px)");

  if (subjectType !== "user" || !user) {
    return <ForbiddenPage backHref={`/${company}/login`} backLabel="Go to Login" />;
  }

  if (isDesktop) {
    return (
      <div className="flex h-screen">
        <EmployeeSidebar />
        <main className="flex-1 overflow-y-auto p-8">{children}</main>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-muted/50">
      <EmployeeHeader />
      <main className="px-5 py-6">{children}</main>
    </div>
  );
}
