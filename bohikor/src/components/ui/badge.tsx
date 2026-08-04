import * as React from "react";
import { cn } from "@/lib/utils";

const BADGE_VARIANTS = {
  default: "bg-primary text-primary-foreground",
  secondary: "bg-secondary text-secondary-foreground",
  destructive: "bg-destructive/10 text-destructive",
  outline: "border border-border text-foreground",
} as const;

function Badge({
  className,
  variant = "default",
  ...props
}: React.ComponentProps<"span"> & { variant?: keyof typeof BADGE_VARIANTS }) {
  return (
    <span
      data-slot="badge"
      className={cn(
        "inline-flex h-5 w-fit shrink-0 items-center justify-center gap-1 rounded-full border border-transparent px-2 py-0.5 text-xs font-medium whitespace-nowrap",
        BADGE_VARIANTS[variant],
        className
      )}
      {...props}
    />
  );
}

export { Badge };
