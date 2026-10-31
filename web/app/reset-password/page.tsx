"use client";

import { useState } from "react";
import Link from "next/link";
import { Button, Spinner } from "@heroui/react";
import { ArrowLeft, ArrowRight, CheckCircle2 } from "lucide-react";
import { AuthLayout } from "@/components/auth-layout";
import { PasswordInput } from "@/components/password-input";
import { ApiError, api, errorText } from "@/lib/api/client";
import { useAuthT } from "@/lib/i18n/dict/auth";

export default function ResetPasswordPage() {
  const t = useAuthT();
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState("");
  const [fields, setFields] = useState<Record<string, string>>({});
  const [done, setDone] = useState(false);
  const [pending, setPending] = useState(false);
  const edit = (set: (v: string) => void) => (e: React.ChangeEvent<HTMLInputElement>) => {
    set(e.target.value);
    setFields({});
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setFields({});
    if (password !== confirm) {
      setFields({ confirm: t.passwordMismatch });
      return;
    }
    const token = new URLSearchParams(window.location.hash.slice(1)).get("token");
    if (!token) {
      setError(t.resetLinkInvalid);
      return;
    }
    setPending(true);
    try {
      await api.resetPassword(token, password);
      window.history.replaceState(null, "", window.location.pathname);
      setDone(true);
    } catch (err) {
      if (err instanceof ApiError && err.fields?.token) setError(t.resetLinkInvalid);
      else if (err instanceof ApiError && err.fields) {
        setFields(err.fields);
        if (!err.fields.password) setError(err.message);
      }
      else setError(errorText(err, t.networkError));
    } finally {
      setPending(false);
    }
  };

  return (
    <AuthLayout>
      {done ? (
        <div className="auth-done">
          <span aria-hidden="true"><CheckCircle2 className="size-6" /></span>
          <h1 className="auth-title">{t.resetDoneTitle}</h1>
          <p>{t.resetDoneDescription}</p>
          <Link href="/login">{t.backToLogin}</Link>
        </div>
      ) : (
        <>
          <Link href="/login" className="auth-back"><ArrowLeft className="size-4" />{t.backToLogin}</Link>
          <h1 className="auth-title">{t.resetTitle}</h1>
          <p className="auth-subtitle">{t.resetSubtitle}</p>
          <form onSubmit={handleSubmit} className="auth-fields" aria-busy={pending}>
            <PasswordInput label={t.newPasswordLabel} name="new-password" autoComplete="new-password" placeholder={t.registerPasswordPlaceholder} required error={fields.password} value={password} onChange={edit(setPassword)} />
            <PasswordInput label={t.confirmPasswordLabel} name="confirm-password" autoComplete="new-password" placeholder={t.passwordPlaceholder} required error={fields.confirm} value={confirm} onChange={edit(setConfirm)} />
            {error && (
              <p role="alert" className="auth-error">
                {error}{" "}
                <Link href="/forgot-password" className="font-medium underline">{t.requestNewLink}</Link>
              </p>
            )}
            <Button type="submit" variant="primary" fullWidth className="auth-submit" isDisabled={pending}>
              {pending && <Spinner size="sm" />}{t.submitNewPassword}{!pending && <ArrowRight className="size-4" />}
            </Button>
          </form>
        </>
      )}
    </AuthLayout>
  );
}
