"use client";

import Link from "next/link";
import Image from "next/image";
import { useParams, useRouter } from "next/navigation";
import { Home, History, LogOut, MoreVertical, User } from "lucide-react";
import { useAuth } from "@/components/providers";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

export function EmployeeHeader() {
  const router = useRouter();
  const { company } = useParams<{ company: string }>();
  const { signOut, refreshSubject } = useAuth();

  const handleSignOut = async () => {
    await signOut();
    await refreshSubject();
    router.push(`/${company}/login`);
  };

  return (
    <header className="border-b border-border bg-background">
      <div className="flex items-center justify-between px-5 py-3.5">
        <Link href={`/${company}`} className="flex items-center gap-2">
          <Image src="/logo.png" alt="" width={22} height={22} className="rounded-md" />
          <span className="font-heading text-base font-semibold">Bohikor</span>
        </Link>

        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" aria-label="Navigation menu">
              <MoreVertical className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onClick={() => router.push(`/${company}`)}>
              <Home className="mr-2 h-4 w-4" />
              Home
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => router.push(`/${company}/history`)}>
              <History className="mr-2 h-4 w-4" />
              History
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => router.push(`/${company}/account`)}>
              <User className="mr-2 h-4 w-4" />
              Account
            </DropdownMenuItem>
            <DropdownMenuItem onClick={handleSignOut}>
              <LogOut className="mr-2 h-4 w-4" />
              Sign Out
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </header>
  );
}
