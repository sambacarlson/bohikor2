"use client";

import { useUsers } from "@/hooks/use-users";
import { useInvitations } from "@/hooks/use-invitations";
import { useRequests } from "@/hooks/use-requests";
import { useEvents } from "@/hooks/use-events";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Users, Mail, Banknote, Activity, RefreshCw } from "lucide-react";

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

export default function DashboardPage() {
  const { data: users } = useUsers();
  const { data: invitations } = useInvitations();
  const { data: requests } = useRequests();
  const { data: events, refetch: refetchEvents, isRefetching: isRefetchingEvents } = useEvents();

  const totalUsers = users?.length ?? 0;
  const totalInvitations = invitations?.length ?? 0;
  const totalRequests = requests?.length ?? 0;
  const successCount = requests?.filter((r) => r.status === "success").length ?? 0;
  const failedCount = requests?.filter((r) => r.status === "failed").length ?? 0;
  const pendingCount = requests?.filter((r) => r.status === "initiated" || r.status === "pending").length ?? 0;

  return (
    <div>
      <div className="mb-8">
        <h1 className="text-3xl font-bold tracking-tight">Dashboard</h1>
        <p className="text-muted-foreground">
          Salary advance pilot overview
        </p>
      </div>

      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4 mb-8">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Users</CardTitle>
            <Users className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{totalUsers}</div>
            <p className="text-xs text-muted-foreground">Registered employees</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Invitations</CardTitle>
            <Mail className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{totalInvitations}</div>
            <p className="text-xs text-muted-foreground">Sent and pending</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Advance Requests</CardTitle>
            <Banknote className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{totalRequests}</div>
            <p className="text-xs text-muted-foreground">
              {successCount} success &middot; {failedCount} failed &middot; {pendingCount} pending
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Activity</CardTitle>
            <Activity className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{events?.length ?? 0}</div>
            <p className="text-xs text-muted-foreground">Recent events</p>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle className="text-lg">Recent Events</CardTitle>
          <Button
            variant="outline"
            size="sm"
            onClick={() => refetchEvents()}
            disabled={isRefetchingEvents}
          >
            <RefreshCw className={`mr-2 h-4 w-4 ${isRefetchingEvents ? "animate-spin" : ""}`} />
            Refresh
          </Button>
        </CardHeader>
        <CardContent>
          {!events || events.length === 0 ? (
            <p className="text-sm text-muted-foreground">No events yet.</p>
          ) : (
            <div className="space-y-3">
              {events.slice(0, 20).map((event) => {
                const label = EVENT_LABELS[event.event_type] || event.event_type;
                const colorClass = EVENT_COLORS[event.event_type] || "bg-gray-100 text-gray-800";
                const [bg, text] = colorClass.split(" ");
                const userEmail = event.user_email || null;

                return (
                  <div key={event.id} className="flex items-center justify-between border-b pb-2 last:border-b-0">
                    <div className="flex items-center gap-3">
                      <Badge className={`${bg} ${text} border-0`}>
                        {label}
                      </Badge>
                      {userEmail && (
                        <span className="text-xs text-muted-foreground">
                          {userEmail}
                        </span>
                      )}
                      <span className="text-xs text-muted-foreground">
                        {formatDate(event.created_at)}
                      </span>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
