"use client";

import { useState, useEffect, useRef } from "react";
import { useSettings, useUpdateSetting } from "@/hooks/use-settings";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";
import { RefreshCw, Pencil, Save } from "lucide-react";
import { toast } from "sonner";

type EditMode = Record<string, boolean>;

export default function SettingsPage() {
  const { data: settings, isLoading, refetch, isRefetching } = useSettings();
  const updateSetting = useUpdateSetting();

  const [advanceAmount, setAdvanceAmount] = useState("");
  const [killSwitch, setKillSwitch] = useState(false);
  const [windowStart, setWindowStart] = useState("");
  const [windowEnd, setWindowEnd] = useState("");
  const [maxDaily, setMaxDaily] = useState("");
  const [maxMonthly, setMaxMonthly] = useState("");
  const [editing, setEditing] = useState<EditMode>({});
  const initialized = useRef(false);

  useEffect(() => {
    if (settings && !initialized.current) {
      initialized.current = true;
      setAdvanceAmount(String(settings.advance_amount_xaf ?? ""));
      setKillSwitch(settings.kill_switch_enabled === "true");
      setWindowStart(String(settings.request_window_start_day ?? ""));
      setWindowEnd(String(settings.request_window_end_day ?? ""));
      setMaxDaily(String(settings.daily_request_limit ?? ""));
      setMaxMonthly(String(settings.monthly_request_limit ?? ""));
    }
  }, [settings]);

  const toggleEdit = (card: string) => {
    setEditing((prev) => ({ ...prev, [card]: !prev[card] }));
  };

  const isEditing = (card: string) => !!editing[card];

  const handleSave = (card: string, key: string, value: string) => {
    updateSetting.mutate(
      { key, value },
      {
        onSuccess: () => {
          toast.success(`Setting "${key}" updated`);
          setEditing((prev) => ({ ...prev, [card]: false }));
        },
        onError: () => toast.error(`Failed to update "${key}"`),
      }
    );
  };

  if (isLoading) {
    return <div className="text-muted-foreground">Loading settings...</div>;
  }

  return (
    <div>
      <div className="mb-8 flex items-center gap-4">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Settings</h1>
          <p className="text-muted-foreground">Configure advance request settings</p>
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

      <div className="grid gap-6">
        <Card id="card-advance-amount" size="sm">
          <CardHeader>
            <CardTitle>Advance Amount</CardTitle>
            <CardDescription>Amount an employee can request as salary advance</CardDescription>
          </CardHeader>
          <CardContent className="flex items-end gap-4">
            <div className="flex-1">
              <Label htmlFor="advance-amount">Amount (XAF)</Label>
              <div className="relative mt-1.5">
                <Input
                  id="advance-amount"
                  type="number"
                  value={advanceAmount}
                  onChange={(e) => setAdvanceAmount(e.target.value)}
                  disabled={!isEditing("advance-amount")}
                />
                <span className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-sm text-muted-foreground">
                  XAF
                </span>
              </div>
            </div>
            <Button
              variant={isEditing("advance-amount") ? "default" : "outline"}
              onClick={() => {
                if (isEditing("advance-amount")) {
                  handleSave("advance-amount", "advance_amount_xaf", advanceAmount);
                } else {
                  toggleEdit("advance-amount");
                }
              }}
              disabled={updateSetting.isPending}
            >
              {isEditing("advance-amount") ? (
                <><Save className="mr-2 h-4 w-4" />Save</>
              ) : (
                <><Pencil className="mr-2 h-4 w-4" />Edit</>
              )}
            </Button>
          </CardContent>
        </Card>

        <Card id="card-kill-switch" size="sm">
          <CardHeader>
            <CardTitle>Kill Switch</CardTitle>
            <CardDescription>Disable all advance requests globally</CardDescription>
          </CardHeader>
          <CardContent className="flex items-end gap-4">
            <div className="flex flex-1 items-center gap-3">
              <Switch
                id="kill-switch"
                checked={killSwitch}
                onCheckedChange={setKillSwitch}
                disabled={!isEditing("kill-switch")}
              />
              <Label htmlFor="kill-switch">
                {killSwitch ? "Advance requests blocked" : "Advance requests allowed"}
              </Label>
            </div>
            <Button
              variant={isEditing("kill-switch") ? "default" : "outline"}
              onClick={() => {
                if (isEditing("kill-switch")) {
                  handleSave("kill-switch", "kill_switch_enabled", killSwitch ? "true" : "false");
                } else {
                  toggleEdit("kill-switch");
                }
              }}
              disabled={updateSetting.isPending}
            >
              {isEditing("kill-switch") ? (
                <><Save className="mr-2 h-4 w-4" />Save</>
              ) : (
                <><Pencil className="mr-2 h-4 w-4" />Edit</>
              )}
            </Button>
          </CardContent>
        </Card>

        <Card id="card-window" size="sm">
          <CardHeader>
            <CardTitle>Request Window</CardTitle>
            <CardDescription>Day-of-month range when employees can submit advance requests (0 = last day)</CardDescription>
          </CardHeader>
          <CardContent className="flex items-end gap-4">
            <div className="flex flex-1 gap-4">
              <div className="flex-1">
                <Label htmlFor="window-start">Start Day</Label>
                <Input
                  id="window-start"
                  type="number"
                  min={1}
                  max={31}
                  value={windowStart}
                  onChange={(e) => setWindowStart(e.target.value)}
                  disabled={!isEditing("window")}
                  className="mt-1.5"
                />
              </div>
              <div className="flex-1">
                <Label htmlFor="window-end">End Day</Label>
                <Input
                  id="window-end"
                  type="number"
                  min={0}
                  max={31}
                  value={windowEnd}
                  onChange={(e) => setWindowEnd(e.target.value)}
                  disabled={!isEditing("window")}
                  className="mt-1.5"
                />
              </div>
            </div>
            <Button
              variant={isEditing("window") ? "default" : "outline"}
              onClick={() => {
                if (isEditing("window")) {
                  handleSave("window", "request_window_start_day", windowStart);
                  handleSave("window", "request_window_end_day", windowEnd);
                } else {
                  toggleEdit("window");
                }
              }}
              disabled={updateSetting.isPending}
            >
              {isEditing("window") ? (
                <><Save className="mr-2 h-4 w-4" />Save</>
              ) : (
                <><Pencil className="mr-2 h-4 w-4" />Edit</>
              )}
            </Button>
          </CardContent>
        </Card>

        <Card id="card-rate-limits" size="sm">
          <CardHeader>
            <CardTitle>Rate Limits</CardTitle>
            <CardDescription>Maximum number of advance requests per day and per month</CardDescription>
          </CardHeader>
          <CardContent className="flex items-end gap-4">
            <div className="flex flex-1 gap-4">
              <div className="flex-1">
                <Label htmlFor="max-daily">Max Daily</Label>
                <Input
                  id="max-daily"
                  type="number"
                  min={0}
                  value={maxDaily}
                  onChange={(e) => setMaxDaily(e.target.value)}
                  disabled={!isEditing("rate-limits")}
                  className="mt-1.5"
                />
              </div>
              <div className="flex-1">
                <Label htmlFor="max-monthly">Max Monthly</Label>
                <Input
                  id="max-monthly"
                  type="number"
                  min={0}
                  value={maxMonthly}
                  onChange={(e) => setMaxMonthly(e.target.value)}
                  disabled={!isEditing("rate-limits")}
                  className="mt-1.5"
                />
              </div>
            </div>
            <Button
              variant={isEditing("rate-limits") ? "default" : "outline"}
              onClick={() => {
                if (isEditing("rate-limits")) {
                  handleSave("rate-limits", "daily_request_limit", maxDaily);
                  handleSave("rate-limits", "monthly_request_limit", maxMonthly);
                } else {
                  toggleEdit("rate-limits");
                }
              }}
              disabled={updateSetting.isPending}
            >
              {isEditing("rate-limits") ? (
                <><Save className="mr-2 h-4 w-4" />Save</>
              ) : (
                <><Pencil className="mr-2 h-4 w-4" />Edit</>
              )}
            </Button>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
