"use client";

import { useState } from "react";
import {
  useCompanies,
  useCompany,
  useCreateCompany,
  useUpdateCompanyStatus,
  useTopUpCompany,
  useAdjustCompanyLedger,
  useCreateCompanyAdmin,
} from "@/hooks/use-companies";
import { useRequestsHealth, useRequestsNeedingReview } from "@/hooks/use-platform-requests";
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
import { PasswordInput } from "@/components/ui/password-input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { RefreshCw } from "lucide-react";
import Link from "next/link";
import { extractApiErrorCode } from "@/lib/errors";
import { toast } from "sonner";

const slugPattern = /^[a-z0-9]+(-[a-z0-9]+)*$/;

function CompanyDetail({
  companyId,
  open,
  onOpenChange,
}: {
  companyId: string | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { data: company, isLoading } = useCompany(companyId ?? "", !!companyId);
  const updateStatus = useUpdateCompanyStatus();
  const topUp = useTopUpCompany();
  const adjust = useAdjustCompanyLedger();
  const createAdmin = useCreateCompanyAdmin();

  const [confirmSuspend, setConfirmSuspend] = useState(false);
  const [topUpAmount, setTopUpAmount] = useState("");
  const [topUpNote, setTopUpNote] = useState("");
  const [topUpError, setTopUpError] = useState("");
  const [adjustAmount, setAdjustAmount] = useState("");
  const [adjustNote, setAdjustNote] = useState("");
  const [adjustError, setAdjustError] = useState("");
  const [adminEmail, setAdminEmail] = useState("");
  const [adminPassword, setAdminPassword] = useState("");
  const [adminError, setAdminError] = useState("");
  const [adminCreated, setAdminCreated] = useState("");

  // The dialog now stays mounted across open/close so Radix can play the close
  // animation, instead of the parent unmounting this component synchronously, so
  // this is no longer freshly mounted on every open. Reset the ephemeral form state
  // ourselves on every closed->open transition (not just when the company changes —
  // reopening the *same* company must also start clean, e.g. an armed "Confirm
  // Suspend?" from a prior visit must not survive close/reopen and fire on a single
  // click), following React's "adjust state during render" pattern rather than an
  // effect (avoids an extra commit/paint, so there's no visible flash to defaults).
  const [wasOpen, setWasOpen] = useState(open);
  if (open !== wasOpen) {
    setWasOpen(open);
    if (open) {
      setConfirmSuspend(false);
      setTopUpAmount("");
      setTopUpNote("");
      setTopUpError("");
      setAdjustAmount("");
      setAdjustNote("");
      setAdjustError("");
      setAdminEmail("");
      setAdminPassword("");
      setAdminError("");
      setAdminCreated("");
    }
  }

  const handleToggleStatus = async () => {
    if (!company) return;
    if (company.status === "active" && !confirmSuspend) {
      setConfirmSuspend(true);
      return;
    }
    try {
      await updateStatus.mutateAsync({
        id: company.id,
        status: company.status === "active" ? "suspended" : "active",
      });
      setConfirmSuspend(false);
      toast.success(company.status === "active" ? "Company suspended" : "Company activated");
    } catch {
      toast.error("Failed to update company status.");
    }
  };

  const handleTopUp = async (e: React.FormEvent) => {
    e.preventDefault();
    setTopUpError("");
    if (!topUpAmount || Number.isNaN(Number(topUpAmount)) || Number(topUpAmount) <= 0) {
      setTopUpError("Amount must be a positive number.");
      return;
    }
    try {
      await topUp.mutateAsync({
        companyId: company!.id,
        amount_xaf: topUpAmount,
        note: topUpNote || undefined,
      });
      toast.success("Company topped up");
      setTopUpAmount("");
      setTopUpNote("");
    } catch {
      setTopUpError("Failed to top up company. Please try again.");
    }
  };

  const handleAdjust = async (e: React.FormEvent) => {
    e.preventDefault();
    setAdjustError("");
    if (!adjustAmount || Number.isNaN(Number(adjustAmount)) || Number(adjustAmount) === 0) {
      setAdjustError("Amount must be non-zero.");
      return;
    }
    if (!adjustNote.trim()) {
      setAdjustError("A note is required for adjustments.");
      return;
    }
    try {
      await adjust.mutateAsync({
        companyId: company!.id,
        amount_xaf: adjustAmount,
        note: adjustNote.trim(),
      });
      toast.success("Ledger adjusted");
      setAdjustAmount("");
      setAdjustNote("");
    } catch {
      setAdjustError("Failed to apply adjustment. Please try again.");
    }
  };

  const handleCreateAdmin = async (e: React.FormEvent) => {
    e.preventDefault();
    setAdminError("");
    setAdminCreated("");
    if (!adminEmail || !adminPassword) {
      setAdminError("Email and password are required.");
      return;
    }
    try {
      const result = await createAdmin.mutateAsync({
        companyId: company!.id,
        email: adminEmail,
        password: adminPassword,
      });
      setAdminCreated(`Admin ${result.email} created — share the password with them securely.`);
      setAdminEmail("");
      setAdminPassword("");
    } catch (err) {
      if (extractApiErrorCode(err) === "create_failed") {
        setAdminError("That email already has an admin account.");
      } else {
        setAdminError("Failed to create admin.");
      }
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        {isLoading || !company ? (
          <>
            <DialogHeader>
              <DialogTitle className="sr-only">Loading company</DialogTitle>
              <DialogDescription className="sr-only">
                Fetching company details
              </DialogDescription>
            </DialogHeader>
            <p className="text-muted-foreground">Loading company...</p>
          </>
        ) : (
          <>
            <DialogHeader>
              <DialogTitle>{company.name}</DialogTitle>
              <DialogDescription className="sr-only">
                Manage company details, balance, and admin access
              </DialogDescription>
            </DialogHeader>
            <div className="mb-2 flex items-center gap-2">
              <span className="font-mono text-sm text-muted-foreground">{company.slug}</span>
              <Badge variant={company.status === "active" ? "default" : "destructive"}>
                {company.status}
              </Badge>
            </div>
            <div className="space-y-6">
              <div>
                <p className="text-sm text-muted-foreground">Balance</p>
                <p className="text-3xl font-bold">{company.balance_xaf} XAF</p>
              </div>

              <div>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleToggleStatus}
                  disabled={updateStatus.isPending}
                >
                  {updateStatus.isPending
                    ? "Updating..."
                    : company.status === "active"
                      ? confirmSuspend
                        ? "Confirm Suspend?"
                        : "Suspend"
                      : "Activate"}
                </Button>
                {confirmSuspend && (
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => setConfirmSuspend(false)}
                  >
                    Cancel
                  </Button>
                )}
              </div>

              <form onSubmit={handleTopUp} className="space-y-4 border-t pt-4">
                <h3 className="font-semibold">Top Up</h3>
                {topUpError && <p className="text-sm text-destructive">{topUpError}</p>}
                <div className="grid gap-4 md:grid-cols-2">
                  <div className="space-y-2">
                    <Label htmlFor="topup-amount">Amount (XAF)</Label>
                    <Input
                      id="topup-amount"
                      inputMode="numeric"
                      value={topUpAmount}
                      onChange={(e) => setTopUpAmount(e.target.value.replace(/[^\d]/g, ""))}
                      placeholder="50000"
                      required
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="topup-note">Note (optional)</Label>
                    <Input
                      id="topup-note"
                      value={topUpNote}
                      onChange={(e) => setTopUpNote(e.target.value)}
                      placeholder="Initial funding"
                    />
                  </div>
                </div>
                <Button type="submit" size="sm" disabled={topUp.isPending}>
                  {topUp.isPending ? "Topping up..." : "Top Up"}
                </Button>
              </form>

              <form onSubmit={handleAdjust} className="space-y-4 border-t pt-4">
                <div>
                  <h3 className="font-semibold">Manual Adjustment (corrections only)</h3>
                  <p className="text-xs text-muted-foreground">
                    Use a negative amount to deduct. A note is required.
                  </p>
                </div>
                {adjustError && <p className="text-sm text-destructive">{adjustError}</p>}
                <div className="grid gap-4 md:grid-cols-2">
                  <div className="space-y-2">
                    <Label htmlFor="adjust-amount">Amount (XAF)</Label>
                    <Input
                      id="adjust-amount"
                      inputMode="text"
                      value={adjustAmount}
                      onChange={(e) => setAdjustAmount(e.target.value)}
                      placeholder="-5000"
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="adjust-note">Note (required)</Label>
                    <Input
                      id="adjust-note"
                      value={adjustNote}
                      onChange={(e) => setAdjustNote(e.target.value)}
                      placeholder="Correction reason"
                    />
                  </div>
                </div>
                <Button type="submit" size="sm" disabled={adjust.isPending}>
                  {adjust.isPending ? "Adjusting..." : "Apply Adjustment"}
                </Button>
              </form>

              <form onSubmit={handleCreateAdmin} className="space-y-4 border-t pt-4">
                <h3 className="font-semibold">Create Admin</h3>
                {adminError && <p className="text-sm text-destructive">{adminError}</p>}
                {adminCreated && <p className="text-sm text-green-600">{adminCreated}</p>}
                <div className="grid gap-4 md:grid-cols-2">
                  <div className="space-y-2">
                    <Label htmlFor="admin-email">Email</Label>
                    <Input
                      id="admin-email"
                      type="email"
                      value={adminEmail}
                      onChange={(e) => setAdminEmail(e.target.value)}
                      placeholder="admin@acme.com"
                      required
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="admin-password">Password</Label>
                    <PasswordInput
                      id="admin-password"
                      value={adminPassword}
                      onChange={(e) => setAdminPassword(e.target.value)}
                      required
                    />
                  </div>
                </div>
                <Button type="submit" size="sm" disabled={createAdmin.isPending}>
                  {createAdmin.isPending ? "Creating..." : "Create Admin"}
                </Button>
              </form>
            </div>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}

export default function PlatformConsolePage() {
  const { data: companies, isLoading, refetch, isRefetching } = useCompanies();
  const createCompany = useCreateCompany();
  const { data: health } = useRequestsHealth();
  const { data: needsReview } = useRequestsNeedingReview();

  const [createOpen, setCreateOpen] = useState(false);
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [formError, setFormError] = useState("");
  // selectedCompanyId is kept (not nulled) across close so the exit animation shows
  // the right company's content instead of flashing to the loading state; detailOpen
  // alone controls visibility.
  const [selectedCompanyId, setSelectedCompanyId] = useState<string | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    setFormError("");

    if (!slugPattern.test(slug)) {
      setFormError("Slug must be lowercase letters, numbers, and hyphens only.");
      return;
    }

    try {
      await createCompany.mutateAsync({ slug, name });
      toast.success("Company created");
      setCreateOpen(false);
      setName("");
      setSlug("");
    } catch (err) {
      const code = extractApiErrorCode(err);
      if (code === "create_failed") {
        setFormError("That slug is already taken.");
      } else if (code === "invalid_slug") {
        setFormError("Slug must be lowercase letters, numbers, and hyphens only.");
      } else {
        setFormError("Failed to create company.");
      }
    }
  };

  return (
    <div>
      <div className="mb-8 flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Platform Console</h1>
          <p className="text-muted-foreground">Manage companies and reconciliation health</p>
        </div>
        <div className="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => refetch()}
            disabled={isRefetching}
          >
            <RefreshCw className={`mr-2 h-4 w-4 ${isRefetching ? "animate-spin" : ""}`} />
            Refresh
          </Button>
          <Button size="sm" onClick={() => setCreateOpen(true)}>
            Create Company
          </Button>
        </div>
      </div>

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Create Company</DialogTitle>
            <DialogDescription className="sr-only">
              Register a new company on the platform
            </DialogDescription>
          </DialogHeader>
          <form onSubmit={handleCreate} className="space-y-4">
            {formError && (
              <p className="text-sm text-destructive">{formError}</p>
            )}

            <div className="space-y-2">
              <Label htmlFor="company-name">Name</Label>
              <Input
                id="company-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Acme Corp"
                required
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="company-slug">Slug</Label>
              <Input
                id="company-slug"
                value={slug}
                onChange={(e) => setSlug(e.target.value)}
                placeholder="acme-corp"
                required
              />
              <p className="text-xs text-muted-foreground">
                lowercase letters, numbers, and hyphens only
              </p>
            </div>

            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                onClick={() => setCreateOpen(false)}
              >
                Cancel
              </Button>
              <Button type="submit" disabled={createCompany.isPending}>
                {createCompany.isPending ? "Creating..." : "Create"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {isLoading ? (
        <p className="text-muted-foreground">Loading companies...</p>
      ) : !companies || companies.length === 0 ? (
        <p className="text-muted-foreground">No companies yet.</p>
      ) : (
        <div className="rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Slug</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Balance</TableHead>
                <TableHead>Created</TableHead>
                <TableHead></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {companies.map((company) => (
                <TableRow key={company.id}>
                  <TableCell className="font-medium">{company.name}</TableCell>
                  <TableCell className="font-mono text-xs">{company.slug}</TableCell>
                  <TableCell>
                    <Badge
                      variant={company.status === "active" ? "default" : "destructive"}
                    >
                      {company.status}
                    </Badge>
                  </TableCell>
                  <TableCell>{company.balance_xaf} XAF</TableCell>
                  <TableCell>
                    {new Date(company.created_at).toLocaleDateString()}
                  </TableCell>
                  <TableCell>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => {
                        setSelectedCompanyId(company.id);
                        setDetailOpen(true);
                      }}
                    >
                      Manage
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      <CompanyDetail
        companyId={selectedCompanyId}
        open={detailOpen}
        onOpenChange={setDetailOpen}
      />

      <Card className="mt-8">
        <CardHeader>
          <CardTitle className="text-lg">Reconciliation Health</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="rounded-md border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Company</TableHead>
                  <TableHead>Processing</TableHead>
                  <TableHead>Pending</TableHead>
                  <TableHead>Needs Review</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {!health || health.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={4} className="text-center text-muted-foreground">
                      All companies healthy — no requests need review.
                    </TableCell>
                  </TableRow>
                ) : (
                  health.map((row) => (
                    <TableRow key={row.company_id}>
                      <TableCell className="font-medium">
                        {row.company_name} ({row.company_slug})
                      </TableCell>
                      <TableCell>{row.processing_count}</TableCell>
                      <TableCell>{row.pending_count}</TableCell>
                      <TableCell>
                        <Badge variant={row.needs_review_count > 0 ? "destructive" : "secondary"}>
                          {row.needs_review_count}
                        </Badge>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>

      <Card className="mt-8">
        <CardHeader>
          <CardTitle className="text-lg">Needs Review</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="rounded-md border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Company</TableHead>
                  <TableHead>User Email</TableHead>
                  <TableHead>Amount</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {!needsReview || needsReview.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={6} className="text-center text-muted-foreground">
                      Nothing flagged for review.
                    </TableCell>
                  </TableRow>
                ) : (
                  needsReview.map((request) => (
                    <TableRow key={request.id}>
                      <TableCell className="font-medium">
                        {request.company_name} ({request.company_slug})
                      </TableCell>
                      <TableCell>{request.user_email || request.user_id}</TableCell>
                      <TableCell>{request.amount_xaf} XAF</TableCell>
                      <TableCell>
                        <Badge variant="destructive">{request.status}</Badge>
                      </TableCell>
                      <TableCell>
                        {new Date(request.created_at).toLocaleDateString()}
                      </TableCell>
                      <TableCell>
                        <Link
                          href={`/${request.company_slug}/admin/requests`}
                          className="text-sm text-primary underline-offset-4 hover:underline"
                        >
                          Open company requests
                        </Link>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
