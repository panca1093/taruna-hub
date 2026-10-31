"use client";

import { useState } from "react";
import Link from "next/link";
import { Button, Card } from "@heroui/react";
import { MailCheck } from "lucide-react";
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
              <h1 className="auth-title">{t.forgotTitle}</h1>
              <p className="auth-subtitle">{t.forgotSubtitle}</p>

              <div className="mt-8">
                <form onSubmit={handleSubmit} className="auth-fields">
                  <TextInput
                    label={t.emailLabel}
                    type="email"
                    placeholder={t.emailPlaceholder}
                    required
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                  />
                  {error && <p role="alert" className="text-sm text-danger">{error}</p>}
                  <Button type="submit" variant="primary" fullWidth className="auth-submit" isDisabled={pending}>
                    {t.submitReset}
                  </Button>
                </form>
              </div>

              <p className="mt-6 text-center text-sm text-muted">
                <Link href="/login" className="font-medium text-accent hover:underline">
                  {t.backToLogin}
                </Link>
              </p>
            </>
          )}
    </AuthLayout>
  );
}
