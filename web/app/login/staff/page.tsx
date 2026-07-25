"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Button, Card } from "@heroui/react";
import { ShieldCheck, UserCog } from "lucide-react";
import { Logo } from "@/components/logo";
import { ThemeToggle } from "@/components/theme-toggle";
import { LanguageToggle } from "@/components/language-toggle";
import { TextInput } from "@/components/ui/form";
import { useSession } from "@/lib/session-context";
import { cn } from "@/lib/cn";
import type { Role } from "@/lib/types";
import { useAuthT } from "@/lib/i18n/dict/auth";

export default function StaffLoginPage() {
  const router = useRouter();
  const { setRole } = useSession();
  const t = useAuthT();
  const [staffRole, setStaffRole] = useState<Extract<Role, "operator" | "admin">>("operator");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setRole(staffRole);
    router.push("/admin");
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

          <div className="mt-6 grid grid-cols-2 gap-2 rounded-lg border border-border bg-surface-secondary p-1">
            {(
              [
                { key: "operator" as const, label: t.roleOperator, icon: UserCog },
                { key: "admin" as const, label: t.roleAdmin, icon: ShieldCheck },
              ]
            ).map((opt) => (
              <button
                key={opt.key}
                type="button"
                onClick={() => setStaffRole(opt.key)}
                className={cn(
                  "flex items-center justify-center gap-1.5 rounded-md py-1.5 text-xs font-medium transition-colors",
                  staffRole === opt.key ? "bg-surface text-foreground shadow-surface" : "text-muted",
                )}
              >
                <opt.icon className="size-3.5" />
                {opt.label}
              </button>
            ))}
          </div>

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
              <Button type="submit" variant="primary" fullWidth>
                {t.submitStaff(staffRole === "admin" ? t.roleAdmin.toLowerCase() : t.roleOperator.toLowerCase())}
              </Button>
            </form>
          </Card>

          <p className="mt-6 text-center text-xs text-muted/70">{t.prototypeNote}</p>
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
