"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Button, Spinner } from "@heroui/react";
import { ArrowLeft, ArrowRight } from "lucide-react";
import { AuthLayout } from "@/components/auth-layout";
import { PasswordInput } from "@/components/password-input";
import { TextInput } from "@/components/ui/form";
import { safeReturnPath, withReturn } from "@/lib/return-intent";
import { homeFor, useSession } from "@/lib/session-context";
import { errorText } from "@/lib/api/client";
import { useAuthT } from "@/lib/i18n/dict/auth";

export function AuthLogin({ staff = false }: { staff?: boolean }) {
  const router = useRouter();
  const { status, role, login } = useSession();
  const t = useAuthT();
  const next = useSearchParams().get("next");
  const [identifier, setIdentifier] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);
  useEffect(() => {
    if (status === "authenticated" && role) router.replace(safeReturnPath(next, role) ?? homeFor(role));
  }, [status, role, router, next]);
  const submit = async (event: React.FormEvent) => {
    event.preventDefault();setError("");setPending(true);
    try {
      const user = await login(identifier, password);
      router.replace(safeReturnPath(next, user.role) ?? homeFor(user.role));
    } catch (err) {
      setError(errorText(err, t.networkError));
    } finally {
      setPending(false);
    }
  };
  return (
    <AuthLayout variant={staff ? "portal" : "login"}>
      <Link href="/" className="auth-back"><ArrowLeft className="size-4" />{t.backToHome}</Link>
      <span className="auth-mobile-emblem auth-small-ring" aria-hidden="true" />
      <h1 className="auth-title">{staff ? t.staffTitle : t.loginTitle}</h1>
      <p className="auth-subtitle">{staff ? t.staffSubtitle : t.loginSubtitle}</p>
      <form onSubmit={submit} className="auth-fields" aria-busy={pending}>
        <TextInput label={t.emailOrPhoneLabel} name="username" autoComplete="username" type="text" placeholder={staff ? t.staffEmailOrPhonePlaceholder : t.loginEmailOrPhonePlaceholder} required value={identifier} onChange={(event) => setIdentifier(event.target.value)} />
        <div className="relative">
          <Link href="/forgot-password" className="auth-forgot">{t.forgotPasswordLink}</Link>
          <PasswordInput label={t.passwordLabel} name="password" autoComplete="current-password" placeholder={t.passwordPlaceholder} required value={password} onChange={(event) => setPassword(event.target.value)} />
        </div>
        {error && <p role="alert" className="auth-error">{error}</p>}
        <Button type="submit" variant="primary" fullWidth className="auth-submit" isDisabled={pending}>
          {pending && <Spinner size="sm" />}{pending ? t.signingIn : staff ? t.submitStaff : t.submitStudent}{!pending && <ArrowRight className="size-4" />}
        </Button>
      </form>
      {!staff && <p className="auth-secondary">{t.noAccount}{" "}<Link href={withReturn("/register", safeReturnPath(next, "student"))}>{t.registerLink}</Link></p>}
    </AuthLayout>
  );
}
