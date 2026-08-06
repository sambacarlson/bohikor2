"use client";

import { useEffect, useRef, useState } from "react";
import { useSearchParams } from "next/navigation";
import { Phone } from "lucide-react";
import { useAuth } from "@/components/providers";
import {
  useAcceptTerms,
  useAddPhone,
  useChangePin,
  usePhoneVerificationStatus,
} from "@/hooks/use-user";
import { useMediaQuery } from "@/hooks/use-media-query";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { extractApiError } from "@/lib/errors";
import { toast } from "sonner";

const TERMS = [
  "This is a one-time salary advance of the amount shown on your home screen.",
  "The amount will be deducted from your next salary payment.",
  "You may only have one active advance request at a time.",
  "Payout is made via mobile money to your verified phone number.",
  "By accepting, you authorize the deduction described above.",
];

const statusText: Record<string, { label: string; className: string }> = {
  success: { label: "Verified", className: "bg-green-100 text-green-700" },
  failed: { label: "Failed", className: "bg-red-100 text-red-700" },
  pending: { label: "Processing…", className: "bg-yellow-100 text-yellow-700" },
  initiated: { label: "Processing…", className: "bg-yellow-100 text-yellow-700" },
};

type Section = "phone" | "pin" | "terms";
const SECTIONS: { id: Section; label: string }[] = [
  { id: "phone", label: "Phone Number" },
  { id: "pin", label: "Change PIN" },
  { id: "terms", label: "Terms & Conditions" },
];

export default function AccountPage() {
  const searchParams = useSearchParams();
  const { user, refreshSubject } = useAuth();
  const isDesktop = useMediaQuery("(min-width: 1024px)");

  const [activeSection, setActiveSection] = useState<Section>("phone");

  const [countryCode, setCountryCode] = useState("+237");
  const [phoneNumber, setPhoneNumber] = useState("");
  const [phoneError, setPhoneError] = useState("");
  const [phoneSubmitted, setPhoneSubmitted] = useState(false);

  const [currentPin, setCurrentPin] = useState("");
  const [newPin, setNewPin] = useState("");
  const [confirmPin, setConfirmPin] = useState("");
  const [pinError, setPinError] = useState("");

  const [termsAccepted, setTermsAccepted] = useState(false);

  // Date.now() is impure and can't be called during render (react-hooks/purity), so it's held
  // in state instead. A verification is normally created *after* mount (via handleAddPhone), so
  // capturing "now" once at mount and never updating it would freeze `elapsed` below the 60s
  // retry threshold forever — re-renders triggered by the 5s status poll don't touch this state.
  // Tick it forward independently so the retry gate actually advances with real time.
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const interval = setInterval(() => setNow(Date.now()), 5000);
    return () => clearInterval(interval);
  }, []);

  const addPhone = useAddPhone();
  const changePin = useChangePin();
  const acceptTerms = useAcceptTerms();
  const { data: verifStatus } = usePhoneVerificationStatus();

  const prevPhoneVerified = useRef(false);

  useEffect(() => {
    const section = searchParams.get("section");
    if (section === "phone" || section === "pin" || section === "terms") {
      setActiveSection(section); // eslint-disable-line react-hooks/set-state-in-effect -- syncs desktop panel selection to the ?section= deep link, including changes after mount
    }
    if (!isDesktop && section) {
      document.getElementById(section)?.scrollIntoView({ behavior: "smooth" });
    }
  }, [searchParams, isDesktop]);

  useEffect(() => {
    if (verifStatus?.phone_verified && !prevPhoneVerified.current) {
      prevPhoneVerified.current = true;
      refreshSubject();
    }
  }, [verifStatus?.phone_verified, refreshSubject]);

  if (!user) return null;

  const verification = verifStatus?.verification;
  const ussdCode = verification?.ussd_code ?? null;
  const fullPhone = `${countryCode}${phoneNumber}`;
  const isValidPhone = (phone: string) => /^\+[1-9]\d{6,14}$/.test(phone);

  const canRetry = (() => {
    if (!verification || verifStatus?.phone_verified) return false;
    if (verification.status === "failed") return true;
    if (verification.status === "initiated" || verification.status === "pending") {
      const elapsed = now - new Date(verification.created_at).getTime();
      return elapsed >= 60000;
    }
    return false;
  })();

  const status = verification ? statusText[verification.status] : null;

  const handleAddPhone = async () => {
    setPhoneError("");
    if (!phoneNumber.trim()) {
      setPhoneError("Phone number is required");
      return;
    }
    if (!isValidPhone(fullPhone)) {
      setPhoneError("Enter a valid phone number (e.g., +237 6XXXXXXXX)");
      return;
    }
    try {
      await addPhone.mutateAsync(fullPhone);
      setPhoneSubmitted(true);
    } catch (err) {
      setPhoneError(extractApiError(err) || "Failed to add phone number.");
    }
  };

  const handleChangePin = async (e: React.FormEvent) => {
    e.preventDefault();
    setPinError("");
    if (!/^\d{5}$/.test(currentPin) || !/^\d{5}$/.test(newPin) || !/^\d{5}$/.test(confirmPin)) {
      setPinError("PINs must be 5 digits");
      return;
    }
    if (newPin !== confirmPin) {
      setPinError("New PINs do not match");
      return;
    }
    try {
      await changePin.mutateAsync({ current_pin: currentPin, new_pin: newPin });
      setCurrentPin("");
      setNewPin("");
      setConfirmPin("");
      toast.success("PIN changed");
    } catch (err) {
      setPinError(extractApiError(err) || "Failed to change PIN.");
    }
  };

  const handleAcceptTerms = async () => {
    try {
      await acceptTerms.mutateAsync();
      await refreshSubject();
      toast.success("Terms accepted");
    } catch {
      // Error handled via acceptTerms.isError below
    }
  };

  const phoneContent = !phoneSubmitted && !verifStatus?.phone_number ? (
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground">
        Add your phone number to receive salary advances via mobile money. A small
        verification amount will be deducted from your phone to confirm your number.
      </p>

      <div className="flex gap-3">
        <div className="w-24 space-y-2">
          <Label htmlFor="country-code">Country code</Label>
          <Input
            id="country-code"
            value={countryCode}
            onChange={(e) =>
              setCountryCode(
                e.target.value.startsWith("+") ? e.target.value : `+${e.target.value}`
              )
            }
            inputMode="tel"
          />
        </div>
        <div className="flex-1 space-y-2">
          <Label htmlFor="phone-number">Phone number</Label>
          <Input
            id="phone-number"
            placeholder="6XXXXXXXX"
            inputMode="tel"
            value={phoneNumber}
            onChange={(e) => setPhoneNumber(e.target.value.replace(/[^\d]/g, ""))}
          />
        </div>
      </div>

      {phoneError && (
        <p className="text-sm text-destructive">{phoneError}</p>
      )}

      <Button onClick={handleAddPhone} disabled={addPhone.isPending} className="w-full">
        {addPhone.isPending ? "Submitting..." : "Verify Phone Number"}
      </Button>
    </div>
  ) : (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <span className="text-sm text-muted-foreground">Phone</span>
        <span>{verifStatus?.phone_number || fullPhone}</span>
      </div>
      <div className="flex items-center justify-between">
        <span className="text-sm text-muted-foreground">Verified</span>
        <span>{verifStatus?.phone_verified ? "Yes" : "No"}</span>
      </div>

      {ussdCode && (
        <div className="rounded-lg border border-yellow-200 bg-yellow-50 p-4">
          <p className="mb-1 text-sm font-bold text-yellow-800">
            Dial the USSD code on your phone
          </p>
          <p className="mb-1 font-mono text-lg text-yellow-900">{ussdCode}</p>
          <p className="text-xs text-yellow-700">
            Enter your mobile money PIN when prompted. Verification will complete
            automatically.
          </p>
        </div>
      )}

      {status && (
        <div className="flex items-center justify-between">
          <span className="text-sm text-muted-foreground">Status</span>
          <span className={`rounded-full px-2 py-0.5 text-xs font-semibold ${status.className}`}>
            {status.label}
          </span>
        </div>
      )}

      {canRetry && (
        <Button
          variant="outline"
          className="w-full"
          onClick={() => {
            setPhoneSubmitted(false);
            setPhoneNumber("");
          }}
        >
          Try Again
        </Button>
      )}
    </div>
  );

  const pinContent = (
    <form onSubmit={handleChangePin} className="space-y-4">
      {pinError && <p className="text-sm text-destructive">{pinError}</p>}

      <div className="space-y-2">
        <Label htmlFor="current-pin">Current PIN</Label>
        <Input
          id="current-pin"
          type="password"
          inputMode="numeric"
          maxLength={5}
          placeholder="00000"
          value={currentPin}
          onChange={(e) => setCurrentPin(e.target.value.replace(/\D/g, ""))}
          required
          autoComplete="current-password"
        />
      </div>

      <div className="space-y-2">
        <Label htmlFor="new-pin">New PIN</Label>
        <Input
          id="new-pin"
          type="password"
          inputMode="numeric"
          maxLength={5}
          placeholder="00000"
          value={newPin}
          onChange={(e) => setNewPin(e.target.value.replace(/\D/g, ""))}
          required
          autoComplete="new-password"
        />
      </div>

      <div className="space-y-2">
        <Label htmlFor="confirm-new-pin">Confirm New PIN</Label>
        <Input
          id="confirm-new-pin"
          type="password"
          inputMode="numeric"
          maxLength={5}
          placeholder="00000"
          value={confirmPin}
          onChange={(e) => setConfirmPin(e.target.value.replace(/\D/g, ""))}
          required
          autoComplete="new-password"
        />
      </div>

      <Button type="submit" className="w-full" disabled={changePin.isPending}>
        {changePin.isPending ? "Changing..." : "Change PIN"}
      </Button>
    </form>
  );

  const termsContent = user.is_terms_accepted ? (
    <div className="space-y-2">
      <p className="font-semibold">Terms Already Accepted</p>
      <p className="text-sm text-muted-foreground">
        You have already accepted the terms and conditions.
      </p>
    </div>
  ) : (
    <div className="space-y-4">
      <ol className="list-decimal space-y-2 pl-5 text-sm text-muted-foreground">
        {TERMS.map((point) => (
          <li key={point}>{point}</li>
        ))}
      </ol>

      <label className="flex cursor-pointer items-start gap-3">
        <input
          type="checkbox"
          className="mt-0.5 h-4 w-4 accent-primary"
          checked={termsAccepted}
          onChange={(e) => setTermsAccepted(e.target.checked)}
        />
        <span className="text-sm">
          I have read and accept the terms and conditions
        </span>
      </label>

      <Button
        onClick={handleAcceptTerms}
        disabled={!termsAccepted || acceptTerms.isPending}
        className="w-full"
      >
        {acceptTerms.isPending ? "Accepting..." : "Accept Terms"}
      </Button>

      {acceptTerms.isError && (
        <p className="text-center text-sm text-destructive">
          Failed to accept terms. Please try again.
        </p>
      )}
    </div>
  );

  const sectionContent: Record<Section, React.ReactNode> = {
    phone: phoneContent,
    pin: pinContent,
    terms: termsContent,
  };

  if (isDesktop) {
    return (
      <div className="flex max-w-[720px] gap-8">
        <div className="w-44 shrink-0">
          <h1 className="font-heading mb-4 text-2xl font-bold">Account</h1>
          <nav className="space-y-1">
            {SECTIONS.map((section) => (
              <button
                key={section.id}
                onClick={() => setActiveSection(section.id)}
                className={cn(
                  "block w-full rounded-lg px-3 py-2 text-left text-sm font-medium transition-colors",
                  activeSection === section.id
                    ? "bg-primary/10 text-primary"
                    : "text-muted-foreground hover:bg-accent hover:text-accent-foreground"
                )}
              >
                {section.label}
              </button>
            ))}
          </nav>
        </div>

        <div className="min-w-0 flex-1">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                {activeSection === "phone" && <Phone className="h-4 w-4" />}
                {SECTIONS.find((s) => s.id === activeSection)?.label}
              </CardTitle>
            </CardHeader>
            <CardContent>{sectionContent[activeSection]}</CardContent>
          </Card>
        </div>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-[620px] space-y-5">
      <h1 className="font-heading text-2xl font-bold">Account</h1>

      <Card id="phone">
        <CardHeader className="space-y-1">
          <CardTitle className="flex items-center gap-2">
            <Phone className="h-4 w-4" />
            Phone Number
          </CardTitle>
        </CardHeader>
        <CardContent>{phoneContent}</CardContent>
      </Card>

      <Card id="pin">
        <CardHeader className="space-y-1">
          <CardTitle>Change PIN</CardTitle>
        </CardHeader>
        <CardContent>{pinContent}</CardContent>
      </Card>

      <Card id="terms">
        <CardHeader className="space-y-1">
          <CardTitle>Terms & Conditions</CardTitle>
        </CardHeader>
        <CardContent>{termsContent}</CardContent>
      </Card>
    </div>
  );
}
