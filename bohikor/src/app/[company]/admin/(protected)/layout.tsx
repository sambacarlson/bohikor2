"use client";

import { useParams } from "next/navigation";
import { AuthGuard } from "@/components/auth-guard";
import { useAuth } from "@/components/providers";
import { ForbiddenPage } from "@/components/forbidden";
import { Sidebar } from "@/components/sidebar";

export default function CompanyAdminLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const { company } = useParams<{ company: string }>();

  return (
    <AuthGuard loginHref={`/${company}/admin/login`}>
      <AdminCheck company={company}>{children}</AdminCheck>
    </AuthGuard>
  );
}

function AdminCheck({
  children,
  company,
}: {
  children: React.ReactNode;
  company: string;
}) {
  // By the time this renders, AuthGuard has already resolved `loading` to
  // false, so `admin` from context (fetched once via /api/admin/me on
  // mount) is ready — no need for a second, duplicate fetch here. `admin`
  // is null for a non-admin subject (e.g. platform_admin), which correctly
  // falls through to ForbiddenPage below.
  const { admin } = useAuth();

  if (!admin) {
    return (
      <ForbiddenPage
        backHref={`/${company}/admin/login`}
        backLabel="Go to Login"
      />
    );
  }

  return (
    <div className="flex h-screen">
      <Sidebar />
      <main className="flex-1 overflow-y-auto p-8">{children}</main>
    </div>
  );
}
