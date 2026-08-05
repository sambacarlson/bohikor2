"use client";

import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { ArrowLeft, RefreshCw } from "lucide-react";
import { useMyAdvanceRequests, useRetryAdvanceRequest } from "@/hooks/use-advance-requests";
import type { AdvanceRequest } from "@/types";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { extractApiError } from "@/lib/errors";
import { toast } from "sonner";

const statusVariant: Record<string, "default" | "secondary" | "destructive" | "outline"> = {
  initiated: "secondary",
  processing: "secondary",
  pending: "outline",
  success: "default",
  failed: "destructive",
};

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function RequestCard({ request }: { request: AdvanceRequest }) {
  const retry = useRetryAdvanceRequest();

  const handleRetry = async () => {
    try {
      await retry.mutateAsync(request.id);
      toast.success("Advance request submitted");
    } catch (err) {
      toast.error(extractApiError(err) || "Failed to retry request");
    }
  };

  return (
    <Card>
      <CardContent className="space-y-2 p-5">
        <div className="flex items-center justify-between gap-3">
          <span className="text-lg font-bold">{request.amount_xaf} XAF</span>
          <Badge variant={statusVariant[request.status] || "secondary"}>
            {request.status}
          </Badge>
        </div>
        <p className="text-sm text-muted-foreground">{formatDate(request.created_at)}</p>
        {request.campay_payout_ref && (
          <p className="font-mono text-xs text-muted-foreground">
            Ref: {request.campay_payout_ref}
          </p>
        )}
        {request.failure_reason && (
          <p className="text-sm text-destructive">{request.failure_reason}</p>
        )}
        {request.status === "failed" && (
          <Button variant="outline" size="sm" onClick={handleRetry} disabled={retry.isPending}>
            {retry.isPending ? "Retrying..." : "Retry"}
          </Button>
        )}
      </CardContent>
    </Card>
  );
}

export default function HistoryPage() {
  const router = useRouter();
  const { company } = useParams<{ company: string }>();
  const { data: requests, isLoading, isError, refetch, isRefetching } = useMyAdvanceRequests();

  return (
    <div className="min-h-screen bg-muted/50">
      <header className="border-b bg-background">
        <div className="mx-auto flex max-w-3xl items-center justify-between px-6 py-4">
          <span className="text-xl font-bold">Bohikor</span>
          <Link href={`/${company}`} className="text-sm text-muted-foreground hover:underline">
            Back to home
          </Link>
        </div>
      </header>

      <main className="mx-auto max-w-3xl space-y-6 px-6 py-6">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Button variant="ghost" size="icon" onClick={() => router.back()} aria-label="Go back">
              <ArrowLeft className="h-4 w-4" />
            </Button>
            <h1 className="text-2xl font-bold">History</h1>
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={() => refetch()}
            disabled={isRefetching}
          >
            <RefreshCw className={`mr-2 h-4 w-4 ${isRefetching ? "animate-spin" : ""}`} />
            Refresh
          </Button>
        </div>

        {isLoading ? (
          <p className="text-muted-foreground">Loading history...</p>
        ) : isError ? (
          <div className="flex flex-col items-center gap-3 py-8">
            <p className="text-destructive">Failed to load history</p>
            <Button variant="outline" size="sm" onClick={() => refetch()}>
              Retry
            </Button>
          </div>
        ) : requests && requests.length > 0 ? (
          <div className="space-y-4">
            {requests.map((request) => (
              <RequestCard key={request.id} request={request} />
            ))}
          </div>
        ) : (
          <p className="py-8 text-center text-muted-foreground">
            No advance requests yet.
          </p>
        )}
      </main>
    </div>
  );
}
