import * as React from "react";
import { cn } from "@/lib/utils";

const ALERT_VARIANTS = {
  default: "bg-card text-card-foreground",
  destructive: "bg-card text-destructive",
} as const;

function Alert({
  className,
  variant = "default",
  ...props
}: React.ComponentProps<"div"> & { variant?: keyof typeof ALERT_VARIANTS }) {
  return (
    <div
      data-slot="alert"
      role="alert"
      className={cn(
        "rounded-lg border px-2.5 py-2 text-sm",
        ALERT_VARIANTS[variant],
        className
      )}
      {...props}
    />
  );
}

function AlertDescription({
  className,
  ...props
}: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="alert-description"
      className={cn("text-sm text-muted-foreground", className)}
      {...props}
    />
  );
}

export { Alert, AlertDescription };
