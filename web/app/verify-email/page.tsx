"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Button, Spinner, toast } from "@heroui/react";
import { ArrowLeft, Mail } from "lucide-react";
import { AuthLayout } from "@/components/auth-layout";
import { RequireRole } from "@/components/require-role";
import { TextInput } from "@/components/ui/form";
import { ApiError, api, errorText } from "@/lib/api/client";
import { useAuthT } from "@/lib/i18n/dict/auth";
import { homeFor, useSession, useUser } from "@/lib/session-context";

function VerificationForm() {
  const t = useAuthT();
  const user = useUser();
  const { reloadUser } = useSession();
  const router = useRouter();
  const destination = homeFor(user.role);
  const [code, setCode] = useState("");
  const [error, setError] = useState("");
  const [fieldError, setFieldError] = useState("");
  const [pending, setPending] = useState(false);
  const [resending, setResending] = useState(false);
  const [verified, setVerified] = useState(false);
  useEffect(() => {
    if (user.emailVerified || !user.email) router.replace(destination);
  }, [user.emailVerified, user.email, router, destination]);
  const submit = async (event: React.FormEvent) => {
    event.preventDefault();setError("");setFieldError("");setPending(true);
    try {
      if (!verified) {
        await api.verifyEmail(code);
        setVerified(true);
      }
      await reloadUser();
      toast.success(t.verifiedToast);
      router.replace(destination);
    } catch (err) {
      if (err instanceof ApiError && err.fields?.code) setFieldError(err.fields.code);
      else setError(errorText(err, t.networkError));
    } finally {
      setPending(false);
    }
  };
  const resend = async () => {
    setError("");setFieldError("");setResending(true);
    try {
      await api.resendEmailCode();
      setCode("");toast.success(t.resentToast);
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        try { await reloadUser(); } catch (reloadError) { setError(errorText(reloadError, t.networkError)); }
      } else setError(errorText(err, t.networkError));
    } finally {
      setResending(false);
    }
  };
  return (
    <AuthLayout variant="verify">
      <Link href={destination} className="auth-back"><ArrowLeft className="size-4" />{t.skipVerification}</Link>
      <span className="auth-email-emblem" aria-hidden="true"><Mail className="size-7" /></span>
      <h1 className="auth-title">{t.verifyPageTitle}</h1>
      <p className="auth-subtitle">{t.verifyPageSubtitle}</p>
      <div className="auth-email-destination"><span>{t.emailDestination}</span><strong>{user.email}</strong></div>
      <form onSubmit={submit} className="auth-fields" aria-busy={pending}>
        <TextInput label={t.codeLabel} name="code" autoComplete="one-time-code" inputMode="numeric" pattern="[0-9]{6}" minLength={6} maxLength={6} placeholder={t.codePlaceholder} required value={code} onChange={(event) => { setCode(event.target.value.replace(/\D/g, ""));setError("");setFieldError(""); }} inputClassName="auth-code-input" description={t.codeHint} error={fieldError} />
        {error && <p role="alert" className="auth-error">{error}</p>}
        <Button type="submit" variant="primary" fullWidth className="auth-submit" isDisabled={pending || resending || code.length !== 6}>
          {pending && <Spinner size="sm" />}{pending ? t.verifying : t.verifyContinue}
        </Button>
      </form>
      <p className="auth-resend"><span>{t.notReceived}</span><Button type="button" variant="ghost" size="sm" onPress={resend} isDisabled={resending || pending || verified}>{resending && <Spinner size="sm" />}{t.resendCode}</Button></p>
      <Link href={destination} className="auth-skip">{t.skipVerification}</Link>
    </AuthLayout>
  );
}

export default function VerifyEmailPage() {
  const t = useAuthT();
  return <RequireRole roles={["student", "operator", "admin"]} title={t.verifyPageTitle}><VerificationForm /></RequireRole>;
}
