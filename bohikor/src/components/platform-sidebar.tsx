"use client";

import Link from "next/link";
import Image from "next/image";
import { motion } from "motion/react";
import { usePathname, useRouter } from "next/navigation";
import { cn } from "@/lib/utils";
import { useAuth } from "@/components/providers";
import { useSidebarCollapse } from "@/hooks/use-sidebar-collapse";
import { ThemeToggle } from "@/components/theme-toggle";
import { LayoutDashboard, LogOut, PanelLeftClose, PanelLeftOpen } from "lucide-react";

export function PlatformSidebar() {
  const pathname = usePathname();
  const router = useRouter();
  const { signOut, refreshSubject } = useAuth();
  const { collapsed, toggle, hydrated } = useSidebarCollapse();

  const navItems = [{ href: "/platform", label: "Dashboard", icon: LayoutDashboard }];

  return (
    <motion.aside
      animate={{ width: collapsed ? 56 : 256 }}
      transition={hydrated ? { duration: 0.2, ease: "easeInOut" } : { duration: 0 }}
      className="flex h-screen shrink-0 flex-col overflow-hidden border-r border-sidebar-border bg-sidebar"
    >
      <div className="flex h-16 items-center gap-2 border-b border-sidebar-border px-4">
        <Image src="/logo.png" alt="Bohikor" width={28} height={28} className="shrink-0" />
        {!collapsed && (
          <div className="truncate">
            <h1 className="text-lg font-semibold text-sidebar-foreground">Bohikor</h1>
            <p className="text-xs text-muted-foreground">Platform</p>
          </div>
        )}
      </div>

      <nav className="flex-1 space-y-1 px-3 py-4">
        {navItems.map((item) => {
          const Icon = item.icon;
          const isActive = pathname === item.href;

          return (
            <Link
              key={item.href}
              href={item.href}
              aria-label={item.label}
              title={collapsed ? item.label : undefined}
              className={cn(
                "flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors",
                collapsed && "justify-center px-0",
                isActive
                  ? "bg-primary/10 text-primary dark:bg-primary/15"
                  : "text-sidebar-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
              )}
            >
              <Icon className="h-5 w-5 shrink-0" />
              {!collapsed && <span className="truncate">{item.label}</span>}
            </Link>
          );
        })}
      </nav>

      <div className="space-y-1 border-t border-sidebar-border p-3">
        <ThemeToggle className={collapsed ? "mx-auto" : ""} />
        <button
          onClick={async () => {
            await signOut();
            await refreshSubject();
            router.push("/platform/login");
          }}
          title={collapsed ? "Sign Out" : undefined}
          className={cn(
            "flex w-full items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium text-sidebar-foreground transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
            collapsed && "justify-center px-0"
          )}
        >
          <LogOut className="h-5 w-5 shrink-0" />
          {!collapsed && "Sign Out"}
        </button>
        <button
          onClick={toggle}
          aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
          title={collapsed ? "Expand sidebar" : "Collapse sidebar"}
          className={cn(
            "flex w-full items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium text-muted-foreground transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
            collapsed && "justify-center px-0"
          )}
        >
          {collapsed ? (
            <PanelLeftOpen className="h-5 w-5 shrink-0" />
          ) : (
            <>
              <PanelLeftClose className="h-5 w-5 shrink-0" />
              <span>Collapse</span>
            </>
          )}
        </button>
      </div>
    </motion.aside>
  );
}
