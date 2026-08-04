"use client";

import { useUsers, useSuspendUser, useActivateUser, useUnlockUser } from "@/hooks/use-users";
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
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { MoreHorizontal, UserMinus, UserCheck, RefreshCw, Unlock } from "lucide-react";
import { toast } from "sonner";

export default function UsersPage() {
  const { data, isLoading, refetch, isRefetching } = useUsers();
  const suspendUser = useSuspendUser();
  const activateUser = useActivateUser();
  const unlockUser = useUnlockUser();

  const handleSuspend = (userId: string, email: string) => {
    suspendUser.mutate(userId, {
      onSuccess: () => toast.success(`User ${email} suspended`),
      onError: () => toast.error("Failed to suspend user"),
    });
  };

  const handleActivate = (userId: string, email: string) => {
    activateUser.mutate(userId, {
      onSuccess: () => toast.success(`User ${email} activated`),
      onError: () => toast.error("Failed to activate user"),
    });
  };

  const handleUnlock = (userId: string, email: string) => {
    unlockUser.mutate(userId, {
      onSuccess: () => toast.success(`User ${email} unlocked`),
      onError: () => toast.error("Failed to unlock user"),
    });
  };

  if (isLoading) {
    return <div className="text-muted-foreground">Loading users...</div>;
  }

  return (
    <div>
      <div className="mb-8">
        <h1 className="text-3xl font-bold tracking-tight">Users</h1>
        <p className="text-muted-foreground">
          Manage employee accounts
        </p>
        <Button
          variant="outline"
          size="sm"
          className="ml-4"
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
              <TableHead>Email</TableHead>
              <TableHead>Name</TableHead>
              <TableHead>Phone</TableHead>
              <TableHead>Verified</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>PIN</TableHead>
              <TableHead>Locked Until</TableHead>
              <TableHead>Terms</TableHead>
              <TableHead>Created</TableHead>
              <TableHead className="w-17.5"></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {data?.length === 0 ? (
              <TableRow>
                <TableCell colSpan={10} className="text-center text-muted-foreground">
                  No users found
                </TableCell>
              </TableRow>
            ) : (
              data?.map((user) => (
                <TableRow key={user.id}>
                  <TableCell className="font-medium">{user.email}</TableCell>
                  <TableCell>{user.full_name || "—"}</TableCell>
                  <TableCell>{user.phone_number || "—"}</TableCell>
                  <TableCell>
                    {user.phone_verified ? (
                      <Badge variant="default">Yes</Badge>
                    ) : (
                      <Badge variant="secondary">No</Badge>
                    )}
                  </TableCell>
                  <TableCell>
                    <Badge
                      variant={
                        user.status === "active"
                          ? "default"
                          : user.status === "locked"
                            ? "destructive"
                            : "secondary"
                      }
                    >
                      {user.status}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    {user.pin_hash ? (
                      <Badge variant="outline">Set</Badge>
                    ) : (
                      <Badge variant="secondary">None</Badge>
                    )}
                  </TableCell>
                  <TableCell>
                    {user.locked_until
                      ? new Date(user.locked_until).toLocaleString()
                      : "—"}
                  </TableCell>
                  <TableCell>
                    {user.is_terms_accepted ? (
                      <Badge variant="outline">Accepted</Badge>
                    ) : (
                      <Badge variant="secondary">Pending</Badge>
                    )}
                  </TableCell>
                  <TableCell>
                    {new Date(user.created_at).toLocaleDateString()}
                  </TableCell>
                  <TableCell>
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button variant="ghost" size="icon">
                          <MoreHorizontal className="h-4 w-4" />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        {user.status === "locked" && (
                          <DropdownMenuItem
                            onClick={() => handleUnlock(user.id, user.email)}
                          >
                            <Unlock className="mr-2 h-4 w-4" />
                            Unlock
                          </DropdownMenuItem>
                        )}
                        {user.status === "active" ? (
                          <DropdownMenuItem
                            onClick={() => handleSuspend(user.id, user.email)}
                          >
                            <UserMinus className="mr-2 h-4 w-4" />
                            Suspend
                          </DropdownMenuItem>
                        ) : user.status === "suspended" ? (
                          <DropdownMenuItem
                            onClick={() => handleActivate(user.id, user.email)}
                          >
                            <UserCheck className="mr-2 h-4 w-4" />
                            Activate
                          </DropdownMenuItem>
                        ) : null}
                      </DropdownMenuContent>
                    </DropdownMenu>
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
