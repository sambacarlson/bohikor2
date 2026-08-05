"use client";

import { useState } from "react";
import { useRequests, useReconcileRequest, useResolveRequest, useReissueRequest } from "@/hooks/use-requests";
import type { AdvanceRequest } from "@/types";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { RefreshCw, AlertTriangle } from "lucide-react";
import { extractApiError, extractApiErrorCode } from "@/lib/errors";
import { toast } from "sonner";

const statusVariant: Record<string, "default" | "secondary" | "destructive" | "outline"> = {
  initiated: "secondary",
  processing: "secondary",
  pending: "outline",
  success: "default",
  failed: "destructive",
};

function RequestActions({ request }: { request: AdvanceRequest }) {
  const reconcile = useReconcileRequest();
  const resolve = useResolveRequest();
  const reissue = useReissueRequest();
  const [resolveOpen, setResolveOpen] = useState(false);
  const [note, setNote] = useState("");

  const anyPending = reconcile.isPending || resolve.isPending || reissue.isPending;

  const handleReconcile = async () => {
    try {
      const result = await reconcile.mutateAsync(request.id);
      toast.success(`Reconciled — status: ${result.status}.`);
    } catch (err) {
      if (extractApiErrorCode(err) === "nothing_to_poll") {
        toast.error("This request has no payout reference to check yet.");
      } else {
        toast.error("Failed to reconcile request.");
      }
    }
  };

  const handleResolve = async () => {
    if (!note.trim()) return;
    try {
      await resolve.mutateAsync({ id: request.id, note: note.trim() });
      toast.success("Marked as resolved.");
      setResolveOpen(false);
      setNote("");
    } catch (err) {
      toast.error(extractApiError(err) || "Failed to resolve request.");
    }
  };

  const handleReissue = async () => {
    try {
      await reissue.mutateAsync(request.id);
      toast.success("Reissued — new request created.");
    } catch (err) {
      const code = extractApiErrorCode(err);
      if (code === "insufficient_employer_float") {
        toast.error(extractApiError(err));
      } else if (code === "already_reissued") {
        toast.error("This request has already been reissued.");
      } else if (code === "transfer_failed") {
        toast.error(
          "Reissue failed — the new attempt also failed. Check the request list for details."
        );
      } else {
        toast.error("Failed to reissue request.");
      }
    }
  };

  return (
    <div className="space-y-2">
      {(request.status === "processing" || request.status === "pending") && (
        <Button
          variant="outline"
          size="sm"
          onClick={handleReconcile}
          disabled={anyPending}
        >
          {reconcile.isPending ? "Reconciling..." : "Reconcile"}
        </Button>
      )}

      {request.needs_admin_review && (
        <>
          {resolveOpen ? (
            <div className="space-y-2">
              <Input
                value={note}
                onChange={(e) => setNote(e.target.value)}
                placeholder="Resolution note (required)"
                className="w-48"
              />
              <div className="flex gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleResolve}
                  disabled={!note.trim() || anyPending}
                >
                  {resolve.isPending ? "Resolving..." : "Confirm Resolve"}
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => {
                    setResolveOpen(false);
                    setNote("");
                  }}
                >
                  Cancel
                </Button>
              </div>
            </div>
          ) : (
            <Button
              variant="outline"
              size="sm"
              onClick={() => setResolveOpen(true)}
              disabled={anyPending}
            >
              Resolve
            </Button>
          )}
        </>
      )}

      {request.status === "failed" && (
        <Button
          variant="outline"
          size="sm"
          onClick={handleReissue}
          disabled={anyPending}
        >
          {reissue.isPending ? "Reissuing..." : "Reissue"}
        </Button>
      )}
    </div>
  );
}

export default function RequestsPage() {
  const { data, isLoading, refetch, isRefetching } = useRequests();

  if (isLoading) {
    return <div className="text-muted-foreground">Loading requests...</div>;
  }

  return (
    <div>
      <div className="mb-8 flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Requests</h1>
          <p className="text-muted-foreground">
            View all salary advance requests
          </p>
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

      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>User Email</TableHead>
              <TableHead>Amount (XAF)</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Payout Ref</TableHead>
              <TableHead>Failure Reason</TableHead>
              <TableHead>Created</TableHead>
              <TableHead>Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {data?.length === 0 ? (
              <TableRow>
                <TableCell colSpan={7} className="text-center text-muted-foreground">
                  No requests found
                </TableCell>
              </TableRow>
            ) : (
              data?.map((request) => (
                <TableRow key={request.id}>
                  <TableCell className="font-medium">
                    {request.user_email || request.user_id}
                  </TableCell>
                  <TableCell>{request.amount_xaf}</TableCell>
                  <TableCell>
                    <div className="flex flex-wrap items-center gap-2">
                      <Badge variant={statusVariant[request.status] || "secondary"}>
                        {request.status}
                      </Badge>
                      {request.needs_admin_review && (
                        <Badge variant="destructive">
                          <AlertTriangle className="mr-1 h-3 w-3" />
                          Needs Review
                        </Badge>
                      )}
                    </div>
                  </TableCell>
                  <TableCell className="font-mono text-xs">
                    {request.campay_payout_ref || "—"}
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground max-w-48 truncate">
                    {request.failure_reason || "—"}
                  </TableCell>
                  <TableCell>
                    {new Date(request.created_at).toLocaleDateString()}
                  </TableCell>
                  <TableCell>
                    <RequestActions request={request} />
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>
    </div>
  );
}
