"use client";

import { useState } from "react";
import { Button, toast } from "@heroui/react";
import { MailWarning } from "lucide-react";
import { TextInput } from "@/components/ui/form";
import { ApiError, api, errorText } from "@/lib/api/client";
import { useAuthT } from "@/lib/i18n/dict/auth";
import { useSession } from "@/lib/session-context";

export function EmailVerifyBanner() {
  const { user, reloadUser } = useSession();
  const t = useAuthT();
  const [code, setCode] = useState("");
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);
  const [resending, setResending] = useState(false);

  if (!user || !user.email || user.emailVerified) return null;

  const verify = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setPending(true);
    try {
      await api.verifyEmail(code);
      await reloadUser();
      toast.success(t.verifiedToast);
    } catch (err) {
      setError(err instanceof ApiError && err.fields?.code ? err.fields.code : errorText(err, t.networkError));
    } finally {
      setPending(false);
    }
  };

  const resend = async () => {
    setError("");
    setResending(true);
    try {
      await api.resendEmailCode();
      toast.success(t.resentToast);
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) await reloadUser().catch(() => {});
      else setError(errorText(err, t.networkError));
    } finally {
      setResending(false);
    }
  };

  return (
    <div className="mb-6 flex items-start gap-3 rounded-xl border border-warning/30 bg-warning-soft p-4 text-sm text-warning-soft-foreground">
      <MailWarning className="mt-0.5 size-4 shrink-0" />
      <div className="min-w-0 flex-1">
        <p className="font-medium">{t.verifyTitle}</p>
        <p className="mt-0.5 text-xs">{t.verifyDescription(user.email)}</p>
        <form onSubmit={verify} className="mt-3 flex flex-wrap items-end gap-2">
          <TextInput
            label={t.codeLabel}
            inputMode="numeric"
            maxLength={6}
            placeholder="000000"
            className="w-32"
            value={code}
            onChange={(e) => setCode(e.target.value.replace(/\D/g, ""))}
          />
          <Button type="submit" size="sm" variant="primary" isDisabled={pending || code.length !== 6}>
            {t.submitCode}
          </Button>
          <Button type="button" size="sm" variant="ghost" onPress={resend} isDisabled={resending}>
            {t.resendCode}
          </Button>
        </form>
        {error && <p role="alert" className="mt-2 text-xs text-danger">{error}</p>}
      </div>
    </div>
  );
}
