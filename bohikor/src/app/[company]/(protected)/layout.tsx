"use client";

import { useParams } from "next/navigation";
import { AuthGuard } from "@/components/auth-guard";
import { useAuth } from "@/components/providers";
import { ForbiddenPage } from "@/components/forbidden";

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

  if (subjectType !== "user" || !user) {
    return <ForbiddenPage backHref={`/${company}/login`} backLabel="Go to Login" />;
  }

  return <>{children}</>;
}
