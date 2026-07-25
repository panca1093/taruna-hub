"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Button, Card } from "@heroui/react";
import { Logo } from "@/components/logo";
import { ThemeToggle } from "@/components/theme-toggle";
import { LanguageToggle } from "@/components/language-toggle";
import { TextInput } from "@/components/ui/form";
import { useSession } from "@/lib/session-context";
import { useAuthT } from "@/lib/i18n/dict/auth";

export default function LoginPage() {
  const router = useRouter();
  const { setRole } = useSession();
  const t = useAuthT();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setRole("student");
    router.push("/student");
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
          <h1 className="font-display text-2xl font-semibold tracking-tight text-foreground">{t.loginTitle}</h1>
          <p className="mt-1.5 text-sm text-muted">{t.loginSubtitle}</p>

          <Card className="mt-6 gap-4 p-5">
            <form onSubmit={handleSubmit} className="flex flex-col gap-4">
              <TextInput
                label={t.emailOrPhoneLabel}
                type="text"
                placeholder={t.loginEmailOrPhonePlaceholder}
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
              <Button type="submit" variant="primary" fullWidth>
                {t.submitStudent}
              </Button>
            </form>
          </Card>

          <p className="mt-6 text-center text-sm text-muted">
            {t.noAccount}{" "}
            <Link href="/register" className="font-medium text-accent hover:underline">
              {t.registerLink}
            </Link>
          </p>
          <p className="mt-2 text-center text-xs text-muted/70">{t.prototypeNote}</p>
          <p className="mt-4 text-center text-xs text-muted">
            {t.staffPrompt}{" "}
            <Link href="/login/staff" className="font-medium text-accent hover:underline">
              {t.staffPortalLink}
            </Link>
          </p>
        </div>
      </main>
    </div>
  );
}
