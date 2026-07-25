"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Button, Card, toast } from "@heroui/react";
import { Logo } from "@/components/logo";
import { ThemeToggle } from "@/components/theme-toggle";
import { LanguageToggle } from "@/components/language-toggle";
import { SelectInput, TextInput } from "@/components/ui/form";
import { useSession } from "@/lib/session-context";
import { programs } from "@/lib/mock-data";
import { useAuthT } from "@/lib/i18n/dict/auth";

const programOptions = programs.map((p) => ({ id: p.code, label: `${p.name} (${p.code})` }));

export default function RegisterPage() {
  const router = useRouter();
  const { setRole } = useSession();
  const t = useAuthT();
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [phone, setPhone] = useState("");
  const [password, setPassword] = useState("");
  const [program, setProgram] = useState<string | null>(null);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setRole("student");
    toast.success(t.toastAccountCreatedTitle, { description: t.toastAccountCreatedDescription });
    router.push("/catalog");
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
          <h1 className="font-display text-2xl font-semibold tracking-tight text-foreground">{t.registerTitle}</h1>
          <p className="mt-1.5 text-sm text-muted">{t.registerSubtitle}</p>

          <Card className="mt-6 gap-4 p-5">
            <form onSubmit={handleSubmit} className="flex flex-col gap-4">
              <TextInput
                label={t.fullNameLabel}
                placeholder={t.fullNamePlaceholder}
                required
                value={name}
                onChange={(e) => setName(e.target.value)}
              />
              <TextInput
                label={t.emailLabel}
                type="email"
                placeholder={t.emailPlaceholder}
                required
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
              <TextInput
                label={t.phoneLabel}
                type="tel"
                placeholder={t.phonePlaceholder}
                required
                value={phone}
                onChange={(e) => setPhone(e.target.value)}
              />
              <TextInput
                label={t.passwordLabel}
                type="password"
                placeholder={t.registerPasswordPlaceholder}
                required
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
              <SelectInput
                label={t.programLabel}
                placeholder={t.programPlaceholder}
                options={programOptions}
                selectedKey={program}
                onSelectionChange={setProgram}
              />
              <Button type="submit" variant="primary" fullWidth className="mt-1">
                {t.submitRegister}
              </Button>
            </form>
          </Card>

          <p className="mt-6 text-center text-sm text-muted">
            {t.haveAccount}{" "}
            <Link href="/login" className="font-medium text-accent hover:underline">
              {t.loginLink}
            </Link>
          </p>
        </div>
      </main>
    </div>
  );
}
