"use client";

import { useState } from "react";
import { useInvitations, useSendInvite } from "@/hooks/use-invitations";
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
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { toast } from "sonner";
import { RefreshCw, Send, Mail } from "lucide-react";
import { getApiErrorMessage } from "@/lib/api";

export default function InvitePage() {
  const [email, setEmail] = useState("");
  const [error, setError] = useState("");
  const { data: invitations, isLoading, refetch } = useInvitations();
  const sendMutation = useSendInvite();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");

    if (!email.trim()) {
      setError("Email is required");
      return;
    }

    sendMutation.mutate(email, {
      onSuccess: () => {
        toast.success(`Invitation sent to ${email}`);
        setEmail("");
      },
      onError: (err: unknown) => {
        const message = getApiErrorMessage(err);
        setError(message);
        toast.error(message);
      },
    });
  };

  return (
    <div>
      <div className="mb-8">
        <h1 className="text-3xl font-bold tracking-tight">Invite Employees</h1>
        <p className="text-muted-foreground">
          Send invitations to employees to join the Bohikor2 platform
        </p>
      </div>

      <div className="space-y-6">
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Mail className="h-5 w-5" />
              Send Invitation
            </CardTitle>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleSubmit} className="flex gap-4">
              <div className="flex-1 space-y-2">
                <Label htmlFor="invite-email">Email Address</Label>
                <Input
                  id="invite-email"
                  type="email"
                  placeholder="admin@company.com"
                  value={email}
                  onChange={(e) => {
                    setEmail(e.target.value);
                    setError("");
                  }}
                  disabled={sendMutation.isPending}
                />
              </div>
              <div className="flex items-end">
                <Button type="submit" disabled={sendMutation.isPending}>
                  {sendMutation.isPending ? (
                    "Sending..."
                  ) : (
                    <>
                      <Send className="mr-2 h-4 w-4" />
                      Invite
                    </>
                  )}
                </Button>
              </div>
            </form>
            {error && (
              <Alert variant="destructive" className="mt-4">
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between">
            <CardTitle>Invitations</CardTitle>
            <Button
              variant="outline"
              size="sm"
              onClick={() => refetch()}
              disabled={isLoading}
            >
              <RefreshCw
                className={`mr-2 h-4 w-4 ${isLoading ? "animate-spin" : ""}`}
              />
              Refresh
            </Button>
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <p className="text-muted-foreground">Loading invitations...</p>
            ) : invitations?.length === 0 ? (
              <p className="text-muted-foreground">No invitations sent yet.</p>
            ) : (
              <div className="rounded-md border">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Email</TableHead>
                      <TableHead>Status</TableHead>
                      <TableHead>Sent At</TableHead>
                      <TableHead>Accepted At</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {invitations?.map((invitation) => (
                      <TableRow key={invitation.id}>
                        <TableCell className="font-medium">
                          {invitation.email}
                        </TableCell>
                        <TableCell>
                          <Badge
                            variant={
                              invitation.status === "accepted"
                                ? "default"
                                : invitation.status === "sent"
                                  ? "secondary"
                                  : invitation.status === "pending"
                                    ? "outline"
                                    : "destructive"
                            }
                          >
                            {invitation.status}
                          </Badge>
                        </TableCell>
                        <TableCell>
                          {invitation.sent_at
                            ? new Date(invitation.sent_at).toLocaleDateString()
                            : "—"}
                        </TableCell>
                        <TableCell>
                          {invitation.accepted_at
                            ? new Date(invitation.accepted_at).toLocaleDateString()
                            : "—"}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
