"use client";

import { useRouter } from "next/navigation";
import { auth } from "@/lib/firebase";
import { Button } from "@/components/ui/button";
import { ShieldAlert } from "lucide-react";

interface ForbiddenPageProps {
  backHref?: string;
  backLabel?: string;
}

export function ForbiddenPage({
  backHref = "/login",
  backLabel = "Go to Login",
}: ForbiddenPageProps) {
  const router = useRouter();

  return (
    <div className="flex min-h-screen items-center justify-center bg-muted/50 p-4">
      <div className="text-center max-w-md space-y-6">
        <div className="flex justify-center">
          <div className="rounded-full bg-destructive/10 p-4">
            <ShieldAlert className="h-12 w-12 text-destructive" />
          </div>
        </div>
        <div className="space-y-2">
          <h1 className="text-3xl font-bold tracking-tight">403</h1>
          <h2 className="text-xl font-semibold text-muted-foreground">
            Access Denied
          </h2>
          <p className="text-muted-foreground">
            You don&apos;t have permission to access this page.
          </p>
        </div>
        <div className="flex flex-col gap-2">
          <Button
            onClick={() => {
              auth.signOut().then(() => {
                router.push(backHref);
              });
            }}
          >
            {backLabel}
          </Button>
          <button
            type="button"
            onClick={() => router.back()}
            className="text-sm text-muted-foreground hover:text-primary underline"
          >
            Go Back
          </button>
        </div>
      </div>
    </div>
  );
}
