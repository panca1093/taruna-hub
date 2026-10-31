"use client";

import { Suspense, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Button, Spinner, toast } from "@heroui/react";
import { ArrowLeft, ArrowRight } from "lucide-react";
import { AuthLayout } from "@/components/auth-layout";
import { PasswordInput } from "@/components/password-input";
import { TextInput } from "@/components/ui/form";
import { safeReturnPath, withReturn } from "@/lib/return-intent";
import { useSession } from "@/lib/session-context";
import { ApiError, errorText } from "@/lib/api/client";
import { useAuthT } from "@/lib/i18n/dict/auth";

const registerFields = ["name", "email", "phone", "password"];

function RegisterForm() {
  const router = useRouter();
  const next = safeReturnPath(useSearchParams().get("next"), "student");
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
      router.push(withReturn("/verify-email", next));
    } catch (err) {
      if (err instanceof ApiError && err.fields) {
        setFields(err.fields);
        if (!Object.keys(err.fields).some((k) => registerFields.includes(k))) setError(err.message);
      } else setError(errorText(err, t.networkError));
    } finally {
      setPending(false);
    }
  };

  return (
    <AuthLayout variant="register">
      <Link href={withReturn("/login", next)} className="auth-back"><ArrowLeft className="size-4" />{t.backToLogin}</Link>
      <h1 className="auth-title">{t.registerTitle}</h1>
      <p className="auth-subtitle">{t.registerSubtitle}</p>
      <form onSubmit={handleSubmit} className="auth-fields" aria-busy={pending}>
        <TextInput label={t.fullNameLabel} name="name" autoComplete="name" placeholder={t.fullNamePlaceholder} required error={fields.name} value={name} onChange={edit("name", setName)} />
        <TextInput label={t.emailLabel} name="email" autoComplete="email" type="email" placeholder={t.emailPlaceholder} required error={fields.email} value={email} onChange={edit("email", setEmail)} />
        <TextInput label={t.phoneLabel} name="phone" autoComplete="tel" type="tel" placeholder={t.phonePlaceholder} error={fields.phone} value={phone} onChange={edit("phone", setPhone)} />
        <div>
          <PasswordInput label={t.passwordLabel} name="password" autoComplete="new-password" minLength={8} placeholder={t.registerPasswordPlaceholder} required error={fields.password} value={password} onChange={edit("password", setPassword)} aria-describedby="password-hint" />
          <p id="password-hint" className="auth-hint">{t.passwordHint}</p>
        </div>
        {error && <p role="alert" className="auth-error">{error}</p>}
        <Button type="submit" variant="primary" fullWidth className="auth-submit" isDisabled={pending}>
          {pending && <Spinner size="sm" />}{pending ? t.creatingAccount : t.submitRegister}{!pending && <ArrowRight className="size-4" />}
        </Button>
      </form>
      <p className="auth-secondary">{t.haveAccount}{" "}<Link href={withReturn("/login", next)}>{t.loginLink}</Link></p>
    </AuthLayout>
  );
}

export default function RegisterPage() {
  return <Suspense><RegisterForm /></Suspense>;
}
