"use client";

import { Suspense, use, useEffect, useRef, useState } from "react";
import { useSearchParams } from "next/navigation";
import { Button, toast } from "@heroui/react";
import { AlertTriangle, BarChart3, CloudOff, Download, TriangleAlert } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { CaptureStage } from "@/components/capture-stage";
import { EmptyState } from "@/components/empty-state";
import { GradientAvatar } from "@/components/gradient-avatar";
import { Icon } from "@/components/icon";
import { Panel } from "@/components/page-blocks";
import { StatusChip } from "@/components/status-badge";
import { TryoutResultView } from "@/components/tryout-result-view";
import { TryoutResultCaptureSheet, viewOf } from "@/components/tryout-result-capture-sheet";
import { LinkButton } from "@/components/ui/link-button";
import { ApiError, api, type StaffResultAttempt, type StaffResultStudentDetail, type ViolationTimeline } from "@/lib/api/client";
import { savePageAsPdf } from "@/lib/capture-pdf";
import { useResultsT } from "@/lib/i18n/dict/results";

type LoadState = "loading" | "ready" | "failed" | "notFound";

const formatDateTime = (iso: string) =>
  new Intl.DateTimeFormat("id-ID", { day: "numeric", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit" }).format(
    new Date(iso),
  );

function StaffAttemptSection({ attempt }: { attempt: StaffResultAttempt }) {
  const t = useResultsT();
  const [violations, setViolations] = useState<ViolationTimeline | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    let cancelled = false;
    api.monitoring
      .violations(attempt.session_id)
      .then((loaded) => {
        if (!cancelled) setViolations(loaded);
      })
      .catch(() => {
        if (!cancelled) setFailed(true);
      });
    return () => {
      cancelled = true;
    };
  }, [attempt.session_id]);

  return (
    <Panel as="section">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
        <h2 className="font-display text-heading-sm">{t.staffSectionTitle}</h2>
        {attempt.irt_warning && (
          <StatusChip tone="warning" className="gap-1">
            <Icon icon={TriangleAlert} size="xs" /> {t.irtWarningBadge}
          </StatusChip>
        )}
      </div>
      {failed ? (
        <p className="text-caption text-danger">{t.violationsError}</p>
      ) : violations === null ? (
        <p className="text-caption text-muted">{t.violationsLoading}</p>
      ) : (
        <>
          <h3 className="mb-3 font-display text-title">{t.violationHistoryTitle(violations.total)}</h3>
          {violations.violations.length === 0 ? (
            <p className="text-caption text-muted">{t.noViolations}</p>
          ) : (
            <ul>
              {violations.violations.map((v) => (
                <li
                  key={v.id}
                  className="relative flex items-baseline gap-3 py-3 not-last:after:absolute not-last:after:top-7 not-last:after:-bottom-3 not-last:after:left-1 not-last:after:w-px not-last:after:bg-border"
                >
                  <span className="z-10 size-2 shrink-0 rounded-full bg-danger" aria-hidden />
                  <div className="flex min-w-0 flex-1 flex-wrap items-baseline justify-between gap-x-3">
                    <span className="flex items-center gap-2 text-small font-medium text-foreground">
                      <Icon icon={AlertTriangle} size="xs" className="text-danger" />
                      {v.type === "screenshot_attempt" ? t.violationScreenshot : t.violationFullscreenExit}
                    </span>
                    <time className="text-label text-muted">{formatDateTime(v.occurred_at)}</time>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </>
      )}
    </Panel>
  );
}

function TryoutStudentResultContent({ params }: { params: Promise<{ tryoutId: string; studentId: string }> }) {
  const { tryoutId, studentId } = use(params);
  const t = useResultsT();
  const attemptParam = useSearchParams().get("attempt");
  const [detail, setDetail] = useState<StaffResultStudentDetail | null>(null);
  const [state, setState] = useState<LoadState>("loading");
  const [reloadKey, setReloadKey] = useState(0);
  const [pdfWorking, setPdfWorking] = useState(false);
  const captureRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    let cancelled = false;
    api.results
      .student(studentId, tryoutId)
      .then((data) => {
        if (cancelled) return;
        setDetail(data);
        setState("ready");
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setState(err instanceof ApiError && err.status === 404 ? "notFound" : "failed");
      });
    return () => {
      cancelled = true;
    };
  }, [studentId, tryoutId, reloadKey]);

  const downloadPdf = async () => {
    if (!captureRef.current || !detail) return;
    setPdfWorking(true);
    try {
      const slug = (value: string) => value.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "");
      const background = getComputedStyle(captureRef.current).backgroundColor;
      await savePageAsPdf(captureRef.current, `tryout-${slug(detail.student.name)}-${slug(detail.tryout.title)}-attempt-${selected.attempt_number}.pdf`, background);
    } catch {
      toast.danger(t.pdfFailed);
    } finally {
      setPdfWorking(false);
    }
  };

  const listCrumb = { label: t.pageTitle, href: "/admin/results" };

  if (state === "notFound" || (state === "ready" && detail?.attempts.length === 0)) {
    return (
      <AppShell title={t.pageTitle} breadcrumb={[listCrumb]}>
        <EmptyState
          icon={BarChart3}
          title={t.detailNotFoundTitle}
          description={t.detailNotFoundDescription}
          action={
            <LinkButton href="/admin/results" variant="primary" size="sm">
              {t.backToList}
            </LinkButton>
          }
        />
      </AppShell>
    );
  }

  if (state === "failed") {
    return (
      <AppShell title={t.pageTitle} breadcrumb={[listCrumb]}>
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

  if (state === "loading" || !detail) {
    return (
      <AppShell title={t.pageTitle} breadcrumb={[listCrumb]}>
        <p className="py-14 text-center text-caption text-muted">{t.detailLoading}</p>
      </AppShell>
    );
  }

  const { student, tryout, attempts } = detail;
  const newest = attempts[0];
  const selected = attempts.find((a) => a.session_id === attemptParam) ?? newest;
  const description = t.detailDescription(tryout.title, tryout.program_code);

  return (
    <AppShell
      title={student.name}
      description={description}
      leading={<GradientAvatar user={student} size="lg" />}
      breadcrumb={[listCrumb, { label: student.name }]}
      actions={
        <Button variant="secondary" size="sm" isDisabled={pdfWorking} onPress={() => void downloadPdf()}>
          <Icon icon={Download} size="xs" /> {pdfWorking ? t.pdfPreparing : t.pdfDownload}
        </Button>
      }
    >
      <div className="space-y-6">
        <TryoutResultView {...viewOf(detail, selected)} attemptHref={(id) => `/admin/results/${tryoutId}/${studentId}?attempt=${id}`} />
        <StaffAttemptSection key={selected.session_id} attempt={selected} />
      </div>
      <CaptureStage>
        <TryoutResultCaptureSheet ref={captureRef} detail={detail} attempt={selected} />
      </CaptureStage>
    </AppShell>
  );
}

export default function TryoutStudentResultPage({ params }: { params: Promise<{ tryoutId: string; studentId: string }> }) {
  return (
    <Suspense>
      <TryoutStudentResultContent params={params} />
    </Suspense>
  );
}
