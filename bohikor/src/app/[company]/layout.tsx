"use client";

import { useEffect } from "react";
import { useParams, useRouter } from "next/navigation";
import { useAuth } from "@/components/providers";

export default function CompanyLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const { company } = useParams<{ company: string }>();
  const { admin, subjectType, loading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (loading || subjectType !== "admin" || !admin) return;
    if (admin.company_slug !== company) {
      router.replace(`/${admin.company_slug}/admin`);
    }
  }, [loading, subjectType, admin, company, router]);

  return <>{children}</>;
}
