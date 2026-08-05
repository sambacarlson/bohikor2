"use client";

import { useLedger } from "@/hooks/use-ledger";
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

const entryTypeClass: Record<string, string> = {
  topup: "bg-green-100 text-green-700",
  reversal: "bg-green-100 text-green-700",
  payout_debit: "bg-gray-100 text-gray-700",
  adjustment: "bg-blue-100 text-blue-700",
};

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export default function BalancePage() {
  const { data, isLoading, refetch, isRefetching } = useLedger();

  return (
    <div>
      <div className="mb-8 flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Balance</h1>
          <p className="text-muted-foreground">Company balance &amp; ledger</p>
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

      <div className="mb-8 rounded-lg border bg-card p-6">
        <p className="text-sm text-muted-foreground">Current balance</p>
        <p className="text-3xl font-bold">{data?.balance_xaf ?? "—"} XAF</p>
      </div>

      {isLoading ? (
        <p className="text-muted-foreground">Loading ledger...</p>
      ) : (
        <div className="rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Type</TableHead>
                <TableHead>Amount (XAF)</TableHead>
                <TableHead>Note</TableHead>
                <TableHead>Date</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {!data || data.entries.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={4} className="text-center text-muted-foreground">
                    No ledger entries yet
                  </TableCell>
                </TableRow>
              ) : (
                data.entries.map((entry) => (
                  <TableRow key={entry.id}>
                    <TableCell>
                      <Badge
                        variant="secondary"
                        className={entryTypeClass[entry.entry_type]}
                      >
                        {entry.entry_type}
                      </Badge>
                    </TableCell>
                    <TableCell
                      className={
                        Number(entry.amount_xaf) < 0 ? "text-destructive" : undefined
                      }
                    >
                      {entry.amount_xaf} XAF
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground max-w-48 truncate">
                      {entry.note || "—"}
                    </TableCell>
                    <TableCell>{formatDate(entry.created_at)}</TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  );
}
