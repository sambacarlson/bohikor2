"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { auth } from "@/lib/firebase";
import { getApiErrorMessage } from "@/lib/api";
import { useUser } from "@/hooks/use-user";
import { useCreateAdvanceRequest } from "@/hooks/use-advance";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Alert, AlertDescription } from "@/components/ui/alert";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Loader2, LogOut, User, ChevronRight, MoreVertical } from "lucide-react";
import Link from "next/link";

export default function ClientHomePage() {
  const router = useRouter();
  const { data: user } = useUser();
  const createRequest = useCreateAdvanceRequest();
  const [showConfirm, setShowConfirm] = useState(false);

  const displayName = user?.full_name || user?.email || "User";
  const email = user?.email || "";
  const phone = user?.phone_number || "";
  const termsAccepted = user?.is_terms_accepted ?? false;

  const handleSignOut = async () => {
    await auth.signOut();
    router.replace("/login");
  };

  const handleRequestAdvance = () => {
    if (!termsAccepted) {
      router.push("/client/terms");
      return;
    }
    setShowConfirm(true);
  };

  const handleConfirmRequest = async () => {
    try {
      await createRequest.mutateAsync({ phoneNumber: phone });
      setShowConfirm(false);
      router.push("/client/history");
    } catch {
      // Error shown via mutation state
    }
  };

  return (
    <div className="min-h-screen bg-muted/30">
      <header className="border-b bg-background">
        <div className="flex items-center justify-between px-6 py-4 max-w-2xl mx-auto">
          <div className="flex items-center gap-2">
            <div className="h-8 w-8 rounded-lg bg-primary flex items-center justify-center">
              <span className="text-primary-foreground font-bold text-sm">B</span>
            </div>
            <h1 className="text-xl font-bold">Bohikor</h1>
          </div>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" data-testid="menu-button">
                <MoreVertical className="h-5 w-5" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onClick={handleSignOut} data-testid="signout-menu-item">
                <LogOut className="mr-2 h-4 w-4" />
                Sign Out
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </header>

      <main className="max-w-2xl mx-auto px-4 py-6 space-y-6">
        <div>
          <h2 className="text-2xl font-bold">{displayName}</h2>
        </div>

        {!termsAccepted && (
          <Alert className="border-yellow-200 bg-yellow-50">
            <AlertDescription className="flex flex-col gap-2">
              <span className="font-medium text-yellow-800">
                Terms not accepted
              </span>
              <span className="text-sm text-yellow-700">
                You must accept the terms before requesting an advance.
              </span>
              <Button
                variant="outline"
                size="sm"
                className="self-start border-yellow-400 text-yellow-800 hover:bg-yellow-100"
                onClick={() => router.push("/client/terms")}
                data-testid="accept-terms-link"
              >
                Accept Terms
              </Button>
            </AlertDescription>
          </Alert>
        )}

        <Card>
          <CardContent className="p-6">
            <Button
              className="w-full text-lg py-6"
              onClick={handleRequestAdvance}
              data-testid="request-advance-button"
            >
              Request Advance
              <span className="ml-2 text-primary-foreground/80 text-sm">
                10,000 XAF
              </span>
            </Button>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <User className="h-5 w-5" />
              Your Information
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex justify-between items-center">
              <span className="text-muted-foreground">Email</span>
              <span className="font-medium">{email}</span>
            </div>
            <div className="flex justify-between items-center">
              <span className="text-muted-foreground">Phone</span>
              <span className="font-medium">{phone}</span>
            </div>
            {user?.full_name && (
              <div className="flex justify-between items-center">
                <span className="text-muted-foreground">Name</span>
                <span className="font-medium">{user.full_name}</span>
              </div>
            )}
            <div className="flex justify-between items-center">
              <span className="text-muted-foreground">Status</span>
              <span className="font-medium">{user?.status || "—"}</span>
            </div>
            <div className="flex justify-between items-center">
              <span className="text-muted-foreground">Terms</span>
              <span className="font-medium">
                {termsAccepted ? "Accepted" : "Not accepted"}
              </span>
            </div>
          </CardContent>
        </Card>

        <Link
          href="/client/history"
          className="block"
          data-testid="view-history-link"
        >
          <Card className="hover:bg-accent/50 transition-colors cursor-pointer">
            <CardContent className="flex items-center justify-between p-5">
              <span className="font-semibold">View Transaction History</span>
              <ChevronRight className="h-5 w-5 text-muted-foreground" />
            </CardContent>
          </Card>
        </Link>
      </main>

      <Dialog open={showConfirm} onOpenChange={setShowConfirm}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Confirm Advance Request</DialogTitle>
            <DialogDescription>
              You are about to request a salary advance of{" "}
              <span className="font-semibold">10,000 XAF</span>.
              This amount plus any applicable charges will be deducted from your
              upcoming salary.
            </DialogDescription>
          </DialogHeader>

          {createRequest.isError && (
            <Alert variant="destructive">
              <AlertDescription>
                {getApiErrorMessage(createRequest.error)}
              </AlertDescription>
            </Alert>
          )}

          <DialogFooter className="gap-2">
            <Button
              variant="outline"
              onClick={() => setShowConfirm(false)}
              disabled={createRequest.isPending}
              data-testid="cancel-request-button"
            >
              Cancel
            </Button>
            <Button
              onClick={handleConfirmRequest}
              disabled={createRequest.isPending}
              data-testid="confirm-request-button"
            >
              {createRequest.isPending && (
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              )}
              {createRequest.isPending ? "Requesting..." : "Confirm"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
