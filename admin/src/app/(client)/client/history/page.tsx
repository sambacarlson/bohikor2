"use client";

import { useRouter } from "next/navigation";
import { useAdvanceRequests } from "@/hooks/use-advance";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { ArrowLeft, RefreshCw, AlertCircle } from "lucide-react";

const STATUS_VARIANTS: Record<string, "default" | "secondary" | "destructive" | "outline"> = {
  initiated: "secondary",
  pending: "outline",
  success: "default",
  failed: "destructive",
};

function StatusBadge({ status }: { status: string }) {
  const variant = STATUS_VARIANTS[status] || "secondary";
  return <Badge variant={variant}>{status}</Badge>;
}

function formatDate(dateStr: string): string {
  if (!dateStr) return "—";
  const date = new Date(dateStr);
  if (isNaN(date.getTime())) return "—";
  return date.toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export default function HistoryPage() {
  const router = useRouter();
  const { data: requests, isLoading, isError, refetch, isRefetching } = useAdvanceRequests();

  return (
    <div className="min-h-screen bg-muted/30">
      <header className="border-b bg-background">
        <div className="flex items-center justify-between px-6 py-4 max-w-2xl mx-auto">
          <div className="flex items-center gap-3">
            <Button variant="ghost" size="icon" onClick={() => router.back()}>
              <ArrowLeft className="h-5 w-5" />
            </Button>
            <h1 className="text-xl font-bold">History</h1>
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={() => refetch()}
            disabled={isRefetching}
            data-testid="refresh-button"
          >
            <RefreshCw className={`mr-2 h-4 w-4 ${isRefetching ? "animate-spin" : ""}`} />
            Refresh
          </Button>
        </div>
      </header>

      <main className="max-w-2xl mx-auto px-4 py-6 space-y-4">
        {isLoading ? (
          <div className="space-y-4" data-testid="history-loading">
            {[1, 2, 3].map((i) => (
              <Card key={i}>
                <CardContent className="p-5">
                  <Skeleton className="h-5 w-24 mb-2" />
                  <Skeleton className="h-4 w-32" />
                </CardContent>
              </Card>
            ))}
          </div>
        ) : isError ? (
          <Alert variant="destructive">
            <AlertCircle className="h-4 w-4" />
            <AlertDescription className="flex flex-col gap-2">
              <span>Failed to load history</span>
              <Button variant="outline" size="sm" className="self-start" onClick={() => refetch()}>
                Retry
              </Button>
            </AlertDescription>
          </Alert>
        ) : requests && requests.length > 0 ? (
          requests.map((req) => (
            <Card key={req.id}>
              <CardContent className="p-5">
                <div className="flex items-center justify-between mb-2">
                  <span className="text-lg font-bold">{req.amount_xaf} XAF</span>
                  <StatusBadge status={req.status} />
                </div>
                <p className="text-sm text-muted-foreground">
                  {formatDate(req.created_at)}
                </p>
                {req.campay_payout_ref && (
                  <p className="text-xs text-muted-foreground mt-1">
                    Ref: {req.campay_payout_ref}
                  </p>
                )}
                {req.failure_reason && (
                  <p className="text-sm text-red-500 mt-1">{req.failure_reason}</p>
                )}
              </CardContent>
            </Card>
          ))
        ) : (
          <Card>
            <CardContent className="p-8 text-center">
              <p className="text-lg text-muted-foreground">
                No advance requests yet.
              </p>
              <p className="text-sm text-muted-foreground mt-2">
                Your transaction history will appear here after you make a request.
              </p>
            </CardContent>
          </Card>
        )}
      </main>
    </div>
  );
}
