"use client";

import { useEvents } from "@/hooks/use-events";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { RefreshCw } from "lucide-react";

const EVENT_LABELS: Record<string, string> = {
  request_initiated: "Request Initiated",
  payout_pending: "Payout Pending",
  payout_success: "Payout Successful",
  payout_failed: "Payout Failed",
  signup_completed: "Signup Completed",
  phone_otp_verified: "Phone Verified",
};

const EVENT_COLORS: Record<string, string> = {
  request_initiated: "bg-blue-100 text-blue-800",
  payout_pending: "bg-yellow-100 text-yellow-800",
  payout_success: "bg-green-100 text-green-800",
  payout_failed: "bg-red-100 text-red-800",
  signup_completed: "bg-purple-100 text-purple-800",
  phone_otp_verified: "bg-gray-100 text-gray-800",
};

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export default function EventsPage() {
  const { data: events, isLoading, refetch, isRefetching } = useEvents();

  return (
    <div>
      <div className="mb-8 flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Events</h1>
          <p className="text-muted-foreground">Full activity log</p>
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
        <p className="text-muted-foreground">Loading events...</p>
      ) : !events || events.length === 0 ? (
        <p className="text-muted-foreground">No events yet.</p>
      ) : (
        <div className="rounded-md border">
          <div className="space-y-3 p-4">
            {events.map((event) => {
              const label = EVENT_LABELS[event.event_type] || event.event_type;
              const colorClass = EVENT_COLORS[event.event_type] || "bg-gray-100 text-gray-800";
              const [bg, text] = colorClass.split(" ");
              const userEmail = event.user_email || null;

              return (
                <div
                  key={event.id}
                  className="flex items-center justify-between border-b pb-2 last:border-b-0"
                >
                  <div className="flex flex-wrap items-center gap-3">
                    <Badge className={`${bg} ${text} border-0`}>{label}</Badge>
                    {userEmail && (
                      <span className="text-xs text-muted-foreground">{userEmail}</span>
                    )}
                    <span className="text-xs text-muted-foreground">
                      {formatDate(event.created_at)}
                    </span>
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}
