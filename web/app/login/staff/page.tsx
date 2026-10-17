"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Button, Card } from "@heroui/react";
import { Logo } from "@/components/logo";
import { ThemeToggle } from "@/components/theme-toggle";
import { LanguageToggle } from "@/components/language-toggle";
import { TextInput } from "@/components/ui/form";
import { homeFor, useSession } from "@/lib/session-context";
import { errorText } from "@/lib/api/client";
import { useAuthT } from "@/lib/i18n/dict/auth";

export default function StaffLoginPage() {
  const router = useRouter();
  const { status, role, login } = useSession();
  const t = useAuthT();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);

  useEffect(() => {
    if (status === "authenticated" && role) router.replace(homeFor(role));
  }, [status, role, router]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setPending(true);
    try {
      const user = await login(email, password);
      router.push(homeFor(user.role));
    } catch (err) {
      setError(errorText(err, t.networkError));
    } finally {
      setPending(false);
    }
  };

  return (
    <div className="flex min-h-dvh flex-col bg-background">
      <header className="flex items-center justify-between px-6 py-5">
        <Logo />
        <div className="flex items-center gap-2">
          <LanguageToggle />
          <ThemeToggle />
        </div>
      </header>

      <main className="flex flex-1 items-center justify-center px-5 pb-16">
        <div className="w-full max-w-sm">
          <h1 className="font-display text-2xl font-semibold tracking-tight text-foreground">{t.staffTitle}</h1>
          <p className="mt-1.5 text-sm text-muted">{t.staffSubtitle}</p>

          <Card className="mt-6 gap-4 p-5">
            <form onSubmit={handleSubmit} className="flex flex-col gap-4">
              <TextInput
                label={t.emailOrPhoneLabel}
                type="text"
                placeholder={t.staffEmailOrPhonePlaceholder}
                required
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
              <TextInput
                label={t.passwordLabel}
                type="password"
                placeholder={t.passwordPlaceholder}
                required
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
              <Link href="/forgot-password" className="-mt-2 text-right text-xs text-accent hover:underline">
                {t.forgotPasswordLink}
              </Link>
              {error && <p role="alert" className="text-sm text-danger">{error}</p>}
              <Button type="submit" variant="primary" fullWidth isDisabled={pending}>
                {t.submitStaff}
              </Button>
            </form>
          </Card>

          <p className="mt-4 text-center text-sm text-muted">
            {t.studentPrompt}{" "}
            <Link href="/login" className="font-medium text-accent hover:underline">
              {t.studentLoginLink}
            </Link>
          </p>
        </div>
      </main>
    </div>
  );
}
