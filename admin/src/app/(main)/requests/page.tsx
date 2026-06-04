"use client";

import { useRequests } from "@/hooks/use-requests";
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
import { RefreshCw } from "lucide-react";

const statusVariant: Record<string, "default" | "secondary" | "destructive" | "outline"> = {
  initiated: "secondary",
  pending: "outline",
  success: "default",
  failed: "destructive",
};

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
            </TableRow>
          </TableHeader>
          <TableBody>
            {data?.length === 0 ? (
              <TableRow>
                <TableCell colSpan={6} className="text-center text-muted-foreground">
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
                    <Badge variant={statusVariant[request.status] || "secondary"}>
                      {request.status}
                    </Badge>
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
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>
    </div>
  );
}
