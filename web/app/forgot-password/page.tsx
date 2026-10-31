"use client";

import { useState } from "react";
import Link from "next/link";
import { Button, Spinner } from "@heroui/react";
import { ArrowLeft, ArrowRight, MailCheck } from "lucide-react";
import { AuthLayout } from "@/components/auth-layout";
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
    <AuthLayout>
      <Link href="/login" className="auth-back"><ArrowLeft className="size-4" />{t.backToLogin}</Link>
      {sent ? (
        <div className="auth-done">
          <span aria-hidden="true"><MailCheck className="size-6" /></span>
          <h1 className="auth-title">{t.sentTitle}</h1>
          <p>{t.sentDescriptionBefore} <strong>{email}</strong> {t.sentDescriptionAfter}</p>
          <Link href="/login">{t.backToLogin}</Link>
        </div>
      ) : (
        <>
          <h1 className="auth-title">{t.forgotTitle}</h1>
          <p className="auth-subtitle">{t.forgotSubtitle}</p>
          <form onSubmit={handleSubmit} className="auth-fields" aria-busy={pending}>
            <TextInput label={t.emailLabel} name="email" autoComplete="email" type="email" placeholder={t.emailPlaceholder} required value={email} onChange={(e) => setEmail(e.target.value)} />
            {error && <p role="alert" className="auth-error">{error}</p>}
            <Button type="submit" variant="primary" fullWidth className="auth-submit" isDisabled={pending}>
              {pending && <Spinner size="sm" />}{t.submitReset}{!pending && <ArrowRight className="size-4" />}
            </Button>
          </form>
        </>
      )}
    </AuthLayout>
  );
}
