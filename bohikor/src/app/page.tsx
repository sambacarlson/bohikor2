import Link from "next/link";
import Image from "next/image";
import { Card, CardContent, CardHeader } from "@/components/ui/card";

export default function LandingPage() {
  return (
    <div className="flex min-h-screen items-center justify-center bg-muted/50 p-4">
      <Card className="w-full max-w-md animate-fade-up">
        <CardHeader className="items-center gap-3 pt-6 text-center">
          <Image src="/logo.png" alt="Bohikor" width={40} height={40} />
          <div>
            <h1 className="font-heading text-2xl font-bold tracking-tight">Bohikor</h1>
            <p className="mt-1 text-sm text-muted-foreground">Salary advances, made simple.</p>
          </div>
        </CardHeader>
        <CardContent className="space-y-6 pb-6 text-center">
          <p className="text-sm text-muted-foreground">
            Your employer gives you a personal sign-in link when you&apos;re invited — check your
            invitation email, or ask your admin if you can&apos;t find it.
          </p>
          <div className="border-t pt-4">
            <Link
              href="/platform/login"
              className="text-sm text-muted-foreground underline-offset-4 hover:text-primary hover:underline"
            >
              Platform administrator? Sign in →
            </Link>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
