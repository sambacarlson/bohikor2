import Link from "next/link";

export default function LandingPage() {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-muted/50 p-4">
      <div className="flex w-full max-w-md flex-col items-center text-center">
        <h1 className="text-4xl font-bold tracking-tight">Bohikor</h1>
        <p className="mt-2 text-lg text-muted-foreground">Salary advances, made simple.</p>
        <p className="mt-6 text-sm text-muted-foreground">
          Your employer gives you a personal sign-in link when you&apos;re invited — check your
          invitation email, or ask your admin if you can&apos;t find it.
        </p>
      </div>
      <div className="mt-16">
        <Link
          href="/platform/login"
          className="text-sm text-muted-foreground underline-offset-4 hover:text-primary hover:underline"
        >
          Platform administrator? Sign in →
        </Link>
      </div>
    </div>
  );
}
