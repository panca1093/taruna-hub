"use client";

import { useState } from "react";
import Link from "next/link";
import { Button, Card } from "@heroui/react";
import { MailCheck } from "lucide-react";
import { Logo } from "@/components/logo";
import { ThemeToggle } from "@/components/theme-toggle";
import { LanguageToggle } from "@/components/language-toggle";
import { TextInput } from "@/components/ui/form";
import { api, errorText } from "@/lib/api/client";
import { useAuthT } from "@/lib/i18n/dict/auth";

export default function ForgotPasswordPage() {
  const t = useAuthT();
  const [email, setEmail] = useState("");
  const [sent, setSent] = useState(false);
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setPending(true);
    try {
      await api.forgotPassword(email);
      setSent(true);
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
          {sent ? (
            <Card className="items-center gap-3 p-6 text-center">
              <MailCheck className="size-8 text-accent" />
              <h1 className="font-display text-xl font-semibold tracking-tight text-foreground">{t.sentTitle}</h1>
              <p className="text-sm text-muted">
                {t.sentDescriptionBefore} <span className="font-medium text-foreground">{email}</span>{" "}
                {t.sentDescriptionAfter}
              </p>
              <Link href="/login" className="mt-2 text-sm font-medium text-accent hover:underline">
                {t.backToLogin}
              </Link>
            </Card>
          ) : (
            <>
              <h1 className="font-display text-2xl font-semibold tracking-tight text-foreground">{t.forgotTitle}</h1>
              <p className="mt-1.5 text-sm text-muted">{t.forgotSubtitle}</p>

              <Card className="mt-6 gap-4 p-5">
                <form onSubmit={handleSubmit} className="flex flex-col gap-4">
                  <TextInput
                    label={t.emailLabel}
                    type="email"
                    placeholder={t.emailPlaceholder}
                    required
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                  />
                  {error && <p role="alert" className="text-sm text-danger">{error}</p>}
                  <Button type="submit" variant="primary" fullWidth className="mt-1" isDisabled={pending}>
                    {t.submitReset}
                  </Button>
                </form>
              </Card>

              <p className="mt-6 text-center text-sm text-muted">
                <Link href="/login" className="font-medium text-accent hover:underline">
                  {t.backToLogin}
                </Link>
              </p>
            </>
          )}
        </div>
      </main>
    </div>
  );
}
