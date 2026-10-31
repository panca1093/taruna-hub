"use client";

import { useState } from "react";
import Link from "next/link";
import { Button, Card } from "@heroui/react";
import { CheckCircle2 } from "lucide-react";
import { AuthLayout } from "@/components/auth-layout";
import { TextInput } from "@/components/ui/form";
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
            <Card className="items-center gap-3 p-6 text-center">
              <CheckCircle2 className="size-8 text-accent" />
              <h1 className="font-display text-xl font-semibold tracking-tight text-foreground">{t.resetDoneTitle}</h1>
              <p className="text-sm text-muted">{t.resetDoneDescription}</p>
              <Link href="/login" className="mt-2 text-sm font-medium text-accent hover:underline">
                {t.backToLogin}
              </Link>
            </Card>
          ) : (
            <>
              <h1 className="auth-title">{t.resetTitle}</h1>
              <p className="auth-subtitle">{t.resetSubtitle}</p>

              <div className="mt-8">
                <form onSubmit={handleSubmit} className="auth-fields">
                  <TextInput
                    label={t.newPasswordLabel}
                    type="password"
                    placeholder={t.registerPasswordPlaceholder}
                    required
                    error={fields.password}
                    value={password}
                    onChange={edit(setPassword)}
                  />
                  <TextInput
                    label={t.confirmPasswordLabel}
                    type="password"
                    placeholder={t.passwordPlaceholder}
                    required
                    error={fields.confirm}
                    value={confirm}
                    onChange={edit(setConfirm)}
                  />
                  {error && (
                    <p role="alert" className="text-sm text-danger">
                      {error}{" "}
                      <Link href="/forgot-password" className="font-medium underline">
                        {t.requestNewLink}
                      </Link>
                    </p>
                  )}
                  <Button type="submit" variant="primary" fullWidth className="auth-submit" isDisabled={pending}>
                    {t.submitNewPassword}
                  </Button>
                </form>
              </div>
            </>
          )}
    </AuthLayout>
  );
}
