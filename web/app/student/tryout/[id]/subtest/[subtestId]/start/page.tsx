"use client";

import { Suspense, use, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Button, Card, toast } from "@heroui/react";
import { ArrowRight, CheckCircle2, CloudOff, Lock } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { EmptyState } from "@/components/empty-state";
import { LinkButton } from "@/components/ui/link-button";
import { api, ApiError, errorText, type StudentTryout } from "@/lib/api/client";
import { useTryoutPrepareT } from "@/lib/i18n/dict/tryout-prepare";

type LoadState = "loading" | "ready" | "noAccess" | "failed";

function PrepareContent({ tryoutId, subtestId }: { tryoutId: string; subtestId: string }) {
  const router = useRouter();
  const t = useTryoutPrepareT();
  const afterId = useSearchParams().get("after");
  const [overview, setOverview] = useState<StudentTryout | null>(null);
  const [state, setState] = useState<LoadState>("loading");
  const [reloadKey, setReloadKey] = useState(0);
  const [ready, setReady] = useState(false);
  const [starting, setStarting] = useState(false);

  useEffect(() => {
    let cancelled = false;
    api.runtime
      .overview(tryoutId)
      .then((loaded) => {
        if (cancelled) return;
        const part = loaded.current?.subtests.find((p) => p.subtest_id === subtestId);
        if (part?.status === "running") {
          router.replace(`/student/tryout/${tryoutId}/subtest/${subtestId}`);
          return;
        }
        setOverview(loaded);
        setState("ready");
      })
      .catch((err: unknown) => {
        if (!cancelled) setState(err instanceof ApiError && err.status === 404 ? "noAccess" : "failed");
      });
    return () => {
      cancelled = true;
    };
  }, [tryoutId, subtestId, router, reloadKey]);

  const shell = (children: React.ReactNode) => <AppShell title={t.pageTitle}>{children}</AppShell>;
  const backLink = (
    <LinkButton href={`/student/tryout/${tryoutId}`} variant="secondary" size="sm">
      {t.backToDetail}
    </LinkButton>
  );

  if (state === "loading") return shell(<p className="py-14 text-center text-sm text-muted">{t.loading}</p>);
  if (state === "noAccess") return shell(<EmptyState icon={Lock} title={t.noAccess} action={backLink} />);
  if (state === "failed" || !overview) {
    return shell(
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
      />,
    );
  }

  const index = overview.subtests.findIndex((st) => st.id === subtestId);
  const subtest = overview.subtests[index];
  if (!subtest) return shell(<EmptyState icon={Lock} title={t.noAccess} action={backLink} />);

  const current = overview.current;
  const statusOf = (id: string) => current?.subtests.find((p) => p.subtest_id === id)?.status ?? "pending";
  if (statusOf(subtestId) === "submitted") {
    return shell(<EmptyState icon={Lock} title={t.lockedTitle} description={t.lockedDescription} action={backLink} />);
  }
  const firstPending = overview.subtests.find((st) => statusOf(st.id) === "pending");
  const startable = current ? firstPending?.id === subtestId : index === 0;
  if (!startable) {
    return shell(<EmptyState icon={Lock} title={t.unavailableTitle} description={t.unavailableDescription} action={backLink} />);
  }

  const justFinished = afterId ? overview.subtests.find((st) => st.id === afterId && statusOf(st.id) === "submitted") : undefined;
  const heading = justFinished ? t.nextHeading : current ? t.heading : t.firstHeading;
  const rules = [t.rule1, t.rule2, t.rule3, t.rule4, t.rule5];

  // The attempt exists first, then this subtest's clock starts; both are the student's explicit action.
  const handleStart = async () => {
    if (starting) return;
    setStarting(true);
    try {
      const session = current ?? (await api.runtime.start(tryoutId));
      await api.runtime.startSubtest(session.id, subtestId);
      router.replace(`/student/tryout/${tryoutId}/subtest/${subtestId}`);
    } catch (err) {
      // An unknown response may still have started the clock; the same session tells the truth.
      try {
        const fresh = await api.runtime.overview(tryoutId);
        if (fresh.current?.subtests.find((p) => p.subtest_id === subtestId)?.status === "running") {
          router.replace(`/student/tryout/${tryoutId}/subtest/${subtestId}`);
          return;
        }
      } catch {
        // Keep the original failure.
      }
      toast.danger(t.startFailed, { description: errorText(err, t.loadErrorDescription) });
      setStarting(false);
    }
  };

  return shell(
    <div className="space-y-6">
      {justFinished && (
        <p className="flex items-center gap-2 rounded-xl border border-success/30 bg-success-soft p-4 text-sm font-medium text-success-soft-foreground">
          <CheckCircle2 className="size-4 shrink-0" /> {t.afterBanner(justFinished.name)}
        </p>
      )}
      <div>
        <h1 className="font-display text-3xl font-semibold tracking-tight text-foreground">{heading}</h1>
        <p className="mt-1 text-sm text-muted">{overview.title}</p>
      </div>
      <div className="grid gap-6 lg:grid-cols-2">
        <Card className="gap-0 p-6">
          <h2 className="font-display text-lg font-semibold text-foreground">{t.heading}</h2>
          <ul className="mt-3 list-disc space-y-2 pl-5 text-sm text-foreground">
            {rules.map((rule) => (
              <li key={rule}>{rule}</li>
            ))}
          </ul>
        </Card>
        <Card className="gap-0 p-6">
          <h2 className="font-display text-lg font-semibold text-foreground">{t.checkTitle}</h2>
          <p className="mt-2 text-sm font-medium text-foreground">
            {index + 1}. {subtest.name}
          </p>
          <p className="text-sm text-muted">{t.subtestMeta(Math.round(subtest.duration_seconds / 60), subtest.question_count)}</p>
          <p className="mt-2 text-xs text-muted">{t.clockNote}</p>
          <label className="mt-5 flex items-start gap-2.5 text-sm text-foreground">
            <input
              type="checkbox"
              checked={ready}
              onChange={(e) => setReady(e.target.checked)}
              className="mt-0.5 size-4 shrink-0 accent-accent"
            />
            {t.confirmCheck}
          </label>
          <Button variant="primary" className="mt-5 w-full" isDisabled={!ready || starting} onPress={() => void handleStart()}>
            {starting ? t.starting : t.startButton} {!starting && <ArrowRight className="size-4" />}
          </Button>
          <LinkButton href={`/student/tryout/${tryoutId}`} variant="ghost" size="sm" className="mt-2 w-full">
            {t.backToDetail}
          </LinkButton>
        </Card>
      </div>
    </div>,
  );
}

export default function PreparePage({ params }: { params: Promise<{ id: string; subtestId: string }> }) {
  const { id, subtestId } = use(params);
  return (
    <Suspense fallback={null}>
      <PrepareContent tryoutId={id} subtestId={subtestId} />
    </Suspense>
  );
}
