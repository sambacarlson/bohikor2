"use client";

import * as React from "react";
import { Dialog as DialogPrimitive } from "radix-ui";
import { X } from "lucide-react";
import { cn } from "@/lib/utils";
import { useMediaQuery } from "@/hooks/use-media-query";

const Dialog = DialogPrimitive.Root;
const DialogTrigger = DialogPrimitive.Trigger;
const DialogClose = DialogPrimitive.Close;

function DialogOverlay({
  className,
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Overlay>) {
  return (
    <DialogPrimitive.Overlay
      data-slot="dialog-overlay"
      className={cn(
        "fixed inset-0 z-50 bg-black/40 backdrop-blur-sm data-[state=open]:animate-[dialog-overlay-in_0.2s_ease-out] data-[state=closed]:animate-[dialog-overlay-out_0.2s_ease-in]",
        className
      )}
      {...props}
    />
  );
}

function DialogContent({
  className,
  children,
  showClose = true,
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Content> & { showClose?: boolean }) {
  const isDesktop = useMediaQuery("(min-width: 1024px)");

  return (
    <DialogPrimitive.Portal>
      <DialogOverlay />
      <DialogPrimitive.Content
        data-slot="dialog-content"
        className={cn(
          "fixed z-50 flex flex-col border border-border bg-card text-card-foreground shadow-lg outline-none",
          isDesktop
            ? "top-1/2 left-1/2 w-full max-w-md -translate-x-1/2 -translate-y-1/2 rounded-2xl max-h-[85vh] data-[state=open]:animate-[dialog-scale-in_0.2s_ease-out] data-[state=closed]:animate-[dialog-scale-out_0.15s_ease-in]"
            : "inset-x-0 bottom-0 rounded-t-2xl max-h-[85svh] data-[state=open]:animate-[dialog-sheet-in_0.3s_ease-out] data-[state=closed]:animate-[dialog-sheet-out_0.2s_ease-in]",
          className
        )}
        {...props}
      >
        {/* Close stays a direct child of the fixed outer box (not the scrollable
            div below) so it can't scroll out of view on tall content. */}
        {showClose && (
          <DialogPrimitive.Close
            aria-label="Close"
            className="absolute top-4 right-4 rounded-lg p-1 text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
          >
            <X className="h-4 w-4" />
          </DialogPrimitive.Close>
        )}
        <div className={cn("min-h-0 overflow-y-auto p-6", !isDesktop && "pb-8")}>
          {!isDesktop && (
            <div
              data-testid="dialog-sheet-handle"
              aria-hidden="true"
              className="mx-auto mb-4 h-1.5 w-9 rounded-full bg-border-strong"
            />
          )}
          {children}
        </div>
      </DialogPrimitive.Content>
    </DialogPrimitive.Portal>
  );
}

function DialogHeader({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div data-slot="dialog-header" className={cn("mb-4 space-y-1.5", className)} {...props} />
  );
}

function DialogTitle({
  className,
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Title>) {
  return (
    <DialogPrimitive.Title
      data-slot="dialog-title"
      className={cn("text-base font-semibold text-foreground", className)}
      {...props}
    />
  );
}

function DialogDescription({
  className,
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Description>) {
  return (
    <DialogPrimitive.Description
      data-slot="dialog-description"
      className={cn("text-sm text-muted-foreground", className)}
      {...props}
    />
  );
}

function DialogFooter({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="dialog-footer"
      className={cn("mt-6 flex justify-end gap-3", className)}
      {...props}
    />
  );
}

export {
  Dialog,
  DialogTrigger,
  DialogClose,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
};
