"use client";

import { motion } from "motion/react";
import { RefreshCw } from "lucide-react";
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

function RequestCard({ request, index }: { request: AdvanceRequest; index: number }) {
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
    <motion.div
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.4, delay: Math.min(index, 6) * 0.05, ease: "easeOut" }}
    >
      <Card>
        <CardContent className="space-y-2 p-5">
          <div className="flex items-center justify-between gap-3">
            <span className="text-lg font-bold tabular-nums">{request.amount_xaf} XAF</span>
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
    </motion.div>
  );
}

export default function HistoryPage() {
  const { data: requests, isLoading, isError, refetch, isRefetching } = useMyAdvanceRequests();

  return (
    <div className="mx-auto max-w-[620px] space-y-5 lg:mx-0">
      <div className="flex items-center justify-between">
        <h1 className="font-heading text-2xl font-bold">History</h1>
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
          {requests.map((request, index) => (
            <RequestCard key={request.id} request={request} index={index} />
          ))}
        </div>
      ) : (
        <p className="py-8 text-center text-muted-foreground">
          No advance requests yet.
        </p>
      )}
    </div>
  );
}
