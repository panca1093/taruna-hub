"use client";

import { use, useEffect, useRef, useState } from "react";
import { Button, toast } from "@heroui/react";
import { BarChart3, CloudOff, Download } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { EmptyState } from "@/components/empty-state";
import { GradientAvatar } from "@/components/gradient-avatar";
import { Icon } from "@/components/icon";
import { KecermatanResultView } from "@/components/kecermatan-result-view";
import { PageHeading } from "@/components/page-heading";
import { LinkButton } from "@/components/ui/link-button";
import { ApiError, api, type KecermatanStaffStudentDetail } from "@/lib/api/client";
import { savePageAsPdf } from "@/lib/capture-pdf";
import { useKecermatanResultsT } from "@/lib/i18n/dict/results";

type LoadState = "loading" | "ready" | "failed" | "notFound";

const loadAttempt = (sessionId: string) => api.kecermatanResults.get(sessionId);

export default function KecermatanStudentResultPage({ params }: { params: Promise<{ testId: string; studentId: string }> }) {
  const { testId, studentId } = use(params);
  const t = useKecermatanResultsT();
  const [detail, setDetail] = useState<KecermatanStaffStudentDetail | null>(null);
  const [state, setState] = useState<LoadState>("loading");
  const [reloadKey, setReloadKey] = useState(0);
  const [pdfWorking, setPdfWorking] = useState(false);
  const captureRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    let cancelled = false;
    api.kecermatanResults
      .student(studentId, testId)
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
  }, [studentId, testId, reloadKey]);

  const downloadPdf = async () => {
    if (!captureRef.current || !detail) return;
    setPdfWorking(true);
    try {
      const slug = (value: string) => value.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "");
      const background = getComputedStyle(captureRef.current).backgroundColor;
      await savePageAsPdf(captureRef.current, `kecermatan-${slug(detail.student.name)}-${slug(detail.test.title)}.pdf`, background);
    } catch {
      toast.danger(t.pdfFailed);
    } finally {
      setPdfWorking(false);
    }
  };

  const listCrumb = { label: t.pageTitle, href: "/admin/kecermatan-results" };

  if (state === "notFound") {
    return (
      <AppShell title={t.pageTitle}>
        <EmptyState
          icon={BarChart3}
          title={t.detailNotFoundTitle}
          description={t.detailNotFoundDescription}
          action={
            <LinkButton href="/admin/kecermatan-results" variant="primary" size="sm">
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

  const { student, test, progress, attempts } = detail;

  return (
    <AppShell
      title={student.name}
      description={`${test.title} · ${t.detailDescription(test.program_code, detail.remaining_attempts)}`}
      leading={<GradientAvatar user={student} size="lg" />}
      breadcrumb={[listCrumb, { label: student.name }]}
      actions={
        <Button variant="secondary" size="sm" isDisabled={pdfWorking} onPress={() => void downloadPdf()}>
          <Icon icon={Download} size="xs" /> {pdfWorking ? t.pdfPreparing : t.pdfDownload}
        </Button>
      }
    >
      <div className="space-y-6">
        <KecermatanResultView progress={progress} attempts={attempts} loadAttempt={loadAttempt} />
      </div>
      <div aria-hidden inert data-theme="light" className="fixed top-0 -left-300 w-275">
        <div ref={captureRef} className="space-y-6 bg-background p-8">
          <PageHeading
            title={student.name}
            description={`${test.title} · ${t.detailDescription(test.program_code, detail.remaining_attempts)}`}
            leading={<GradientAvatar user={student} size="lg" />}
            size="compact"
          />
          <KecermatanResultView progress={progress} attempts={attempts} loadAttempt={loadAttempt} initialOpenId={null} />
        </div>
      </div>
    </AppShell>
  );
}
