"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useAcceptTerms } from "@/hooks/use-advance";
import { useUser } from "@/hooks/use-user";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Checkbox } from "@/components/ui/checkbox";
import { Loader2, ArrowLeft } from "lucide-react";

const TERMS_TEXT = `By requesting a salary advance of 10,000 XAF, you agree to the following terms:

1. This is a one-time pilot advance.
2. The advance amount (10,000 XAF) plus any applicable charges will be deducted from your upcoming salary payment.
3. You may only have one active advance request at a time.
4. The advance is sent via mobile money to the phone number you provide.
5. By accepting, you authorize the deduction from your salary.

Please read these terms carefully before proceeding.`;

export default function TermsPage() {
  const router = useRouter();
  const { data: user } = useUser();
  const [accepted, setAccepted] = useState(false);
  const acceptTerms = useAcceptTerms();
  const termsAlreadyAccepted = user?.is_terms_accepted === true;

  const handleAccept = async () => {
    try {
      await acceptTerms.mutateAsync({ version: "v1" });
      router.push("/client");
    } catch {
      // Error handled by mutation state
    }
  };

  return (
    <div className="min-h-screen bg-muted/30">
      <header className="border-b bg-background">
        <div className="flex items-center gap-3 px-6 py-4 max-w-2xl mx-auto">
          <Button variant="ghost" size="icon" onClick={() => router.back()}>
            <ArrowLeft className="h-5 w-5" />
          </Button>
          <h1 className="text-xl font-bold">Terms & Conditions</h1>
        </div>
      </header>

      <main className="max-w-2xl mx-auto px-4 py-6 space-y-6">
        {termsAlreadyAccepted ? (
          <Card>
            <CardContent className="p-8 text-center space-y-4">
              <h2 className="text-xl font-bold">Terms Already Accepted</h2>
              <p className="text-muted-foreground">
                You have already accepted the terms and conditions.
              </p>
              <Button onClick={() => router.push("/client")}>Go Back</Button>
            </CardContent>
          </Card>
        ) : (
          <>
            <Card>
              <CardContent className="p-6 whitespace-pre-line text-sm text-muted-foreground leading-relaxed">
                {TERMS_TEXT}
              </CardContent>
            </Card>

            <div className="flex items-start gap-3">
              <Checkbox
                id="terms"
                checked={accepted}
                onCheckedChange={(checked) => setAccepted(checked === true)}
                data-testid="terms-checkbox"
              />
              <label
                htmlFor="terms"
                className="text-sm leading-tight cursor-pointer"
              >
                I have read and accept the terms and conditions
              </label>
            </div>

            {acceptTerms.isError && (
              <Alert variant="destructive">
                <AlertDescription>
                  Failed to accept terms. Please try again.
                </AlertDescription>
              </Alert>
            )}

            <Button
              className="w-full"
              onClick={handleAccept}
              disabled={!accepted || acceptTerms.isPending}
              data-testid="accept-terms-button"
            >
              {acceptTerms.isPending && (
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              )}
              {acceptTerms.isPending ? "Accepting..." : "Accept Terms"}
            </Button>
          </>
        )}
      </main>
    </div>
  );
}
