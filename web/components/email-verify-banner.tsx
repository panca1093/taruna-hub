"use client";

import Link from "next/link";
import { MailWarning } from "lucide-react";
import { useAuthT } from "@/lib/i18n/dict/auth";
import { useSession } from "@/lib/session-context";

export function EmailVerifyBanner() {
  const { user } = useSession();
  const t = useAuthT();
  if (!user || !user.email || user.emailVerified) return null;
  return (
    <div className="mb-6 flex flex-wrap items-center gap-3 rounded-tile border border-border bg-surface px-4 py-3.5 text-label">
      <MailWarning className="size-4.5 shrink-0 text-accent" aria-hidden="true" />
      <div className="min-w-0 flex-1 basis-48"><p className="text-caption font-medium text-foreground">{t.verifyTitle}</p><p className="mt-0.5 text-label leading-relaxed text-muted">{t.verifyLaterDescription}</p></div>
      <Link href="/verify-email" className="inline-flex min-h-10 items-center rounded-chip px-2 text-label font-medium whitespace-nowrap text-accent outline-none hover:underline focus-visible:ring-2 focus-visible:ring-focus">{t.verifyNow}</Link>
    </div>
  );
}
