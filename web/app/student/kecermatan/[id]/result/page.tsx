"use client";

import { Suspense, use, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Button } from "@heroui/react";
import { ArrowRight, CloudOff } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { EmptyState } from "@/components/empty-state";
import { Icon } from "@/components/icon";
import { KecermatanResultView } from "@/components/kecermatan-result-view";
import { NoteBox } from "@/components/page-blocks";
import { LinkButton } from "@/components/ui/link-button";
import { ApiError, api, type KecermatanOverview } from "@/lib/api/client";
import { useKecermatanResultT } from "@/lib/i18n/dict/kecermatan-result";

type LoadState = "loading" | "ready" | "failed";

const loadAttempt = (sessionId: string) => api.kecermatanRuntime.result(sessionId);

function ResultContent({ testId }: { testId: string }) {
  const router = useRouter();
  const t = useKecermatanResultT();
  const attemptParam = useSearchParams().get("attempt");
  const [overview, setOverview] = useState<KecermatanOverview | null>(null);
  const [state, setState] = useState<LoadState>("loading");
  const [reloadKey, setReloadKey] = useState(0);
  const [initialOpenId, setInitialOpenId] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    api.kecermatanRuntime
      .overview(testId)
      .then((data) => {
        if (cancelled) return;
        const completed = data.attempts.filter((attempt) => attempt.status === "completed");
        if (!data.progress || completed.length === 0) {
          router.replace(`/student/kecermatan/${testId}`);
          return;
        }
        setOverview(data);
        setInitialOpenId(completed.find((attempt) => attempt.session_id === attemptParam)?.session_id ?? completed[0].session_id);
        setState("ready");
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        if (err instanceof ApiError && err.status === 404) {
          router.replace(`/student/kecermatan/${testId}`);
          return;
        }
        setState("failed");
      });
    return () => {
      cancelled = true;
    };
  }, [testId, attemptParam, router, reloadKey]);

  if (state === "failed") {
    return (
      <AppShell title={t.pageTitle}>
        <EmptyState
          icon={CloudOff}
          title={t.loadErrorTitle}
          description={t.loadErrorDescription}
          action={
            <Button
              variant="primary"
              size="sm"
              onPress={() => {
                setState("loading");
                setReloadKey((key) => key + 1);
              }}
            >
              {t.retry}
            </Button>
          }
        />
      </AppShell>
    );
  }

  if (state === "loading" || !overview?.progress) {
    return (
      <AppShell title={t.pageTitle}>
        <p className="py-14 text-center text-sm text-muted">{t.loadingResult}</p>
      </AppShell>
    );
  }

  const { test, progress } = overview;
  const completed = overview.attempts.filter((attempt) => attempt.status === "completed");
  const canStart = overview.current !== null || test.attempts_remaining > 0;

  return (
    <AppShell
      title={t.heading}
      description={`${test.title} · ${t.attemptsDone(completed.length)}`}
      breadcrumb={[{ label: t.crumbDashboard, href: "/student" }, { label: t.crumbResult }]}
      actions={
        canStart ? (
          <LinkButton href={`/student/kecermatan/${testId}/attempt`} variant="primary">
            {overview.current !== null ? t.continueCta : t.tryAgainCta} <Icon icon={ArrowRight} size="sm" />
          </LinkButton>
        ) : undefined
      }
    >
      <div className="space-y-6">
        <KecermatanResultView key={initialOpenId} progress={progress} attempts={completed} loadAttempt={loadAttempt} initialOpenId={initialOpenId} />

        <NoteBox>{t.quotaLeft(test.attempts_remaining)}</NoteBox>

        <div className="flex flex-wrap items-center gap-3.5">
          <LinkButton href={`/student/kecermatan/${testId}`} variant="secondary">
            {t.backToDetail} <Icon icon={ArrowRight} size="xs" />
          </LinkButton>
          <LinkButton href="/student" variant="secondary">
            {t.backToDashboard} <Icon icon={ArrowRight} size="xs" />
          </LinkButton>
        </div>
      </div>
    </AppShell>
  );
}

export default function KecermatanResultPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  return (
    <Suspense fallback={null}>
      <ResultContent testId={id} />
    </Suspense>
  );
}
