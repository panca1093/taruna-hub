"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Button, Card, toast } from "@heroui/react";
import { Logo } from "@/components/logo";
import { ThemeToggle } from "@/components/theme-toggle";
import { LanguageToggle } from "@/components/language-toggle";
import { TextInput } from "@/components/ui/form";
import { useSession } from "@/lib/session-context";
import { ApiError, errorText } from "@/lib/api/client";
import { useAuthT } from "@/lib/i18n/dict/auth";

export default function RegisterPage() {
  const router = useRouter();
  const { register } = useSession();
  const t = useAuthT();
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [phone, setPhone] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [fields, setFields] = useState<Record<string, string>>({});
  const [pending, setPending] = useState(false);
  const edit = (key: string, set: (v: string) => void) => (e: React.ChangeEvent<HTMLInputElement>) => {
    set(e.target.value);
    setFields((f) => ({ ...f, [key]: "" }));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setFields({});
    setPending(true);
    try {
      await register({ name, email, phone: phone.trim() || undefined, password });
      toast.success(t.toastAccountCreatedTitle, { description: t.toastAccountCreatedDescription });
      router.push("/catalog");
    } catch (err) {
      if (err instanceof ApiError && err.fields) setFields(err.fields);
      else setError(errorText(err, t.networkError));
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
          <h1 className="font-display text-2xl font-semibold tracking-tight text-foreground">{t.registerTitle}</h1>
          <p className="mt-1.5 text-sm text-muted">{t.registerSubtitle}</p>

          <Card className="mt-6 gap-4 p-5">
            <form onSubmit={handleSubmit} className="flex flex-col gap-4">
              <TextInput
                label={t.fullNameLabel}
                placeholder={t.fullNamePlaceholder}
                required
                error={fields.name}
                value={name}
                onChange={edit("name", setName)}
              />
              <TextInput
                label={t.emailLabel}
                type="email"
                placeholder={t.emailPlaceholder}
                required
                error={fields.email}
                value={email}
                onChange={edit("email", setEmail)}
              />
              <TextInput
                label={t.phoneLabel}
                type="tel"
                placeholder={t.phonePlaceholder}
                error={fields.phone}
                value={phone}
                onChange={edit("phone", setPhone)}
              />
              <TextInput
                label={t.passwordLabel}
                type="password"
                placeholder={t.registerPasswordPlaceholder}
                required
                error={fields.password}
                value={password}
                onChange={edit("password", setPassword)}
              />
              {error && <p role="alert" className="text-sm text-danger">{error}</p>}
              <Button type="submit" variant="primary" fullWidth className="mt-1" isDisabled={pending}>
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
