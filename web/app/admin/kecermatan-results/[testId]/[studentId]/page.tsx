"use client";

import { use, useEffect, useState } from "react";
import { Button, toast } from "@heroui/react";
import { BarChart3, CloudOff, Download } from "lucide-react";
import { ExportStatus } from "@/components/admin/export-status";
import { AppShell } from "@/components/app-shell";
import { EmptyState } from "@/components/empty-state";
import { GradientAvatar } from "@/components/gradient-avatar";
import { Icon } from "@/components/icon";
import { KecermatanResultView } from "@/components/kecermatan-result-view";
import { LinkButton } from "@/components/ui/link-button";
import { ApiError, api, errorText, type KecermatanStaffStudentDetail } from "@/lib/api/client";
import { useKecermatanResultsT } from "@/lib/i18n/dict/results";
import { useTryoutPdfT } from "@/lib/i18n/dict/tryout-pdf";

type LoadState = "loading" | "ready" | "failed" | "notFound";

const loadAttempt = (sessionId: string) => api.kecermatanResults.get(sessionId);

export default function KecermatanStudentResultPage({ params }: { params: Promise<{ testId: string; studentId: string }> }) {
  const { testId, studentId } = use(params);
  const t = useKecermatanResultsT();
  const pdfT = useTryoutPdfT();
  const [detail, setDetail] = useState<KecermatanStaffStudentDetail | null>(null);
  const [state, setState] = useState<LoadState>("loading");
  const [reloadKey, setReloadKey] = useState(0);
  const [pdfRequesting, setPdfRequesting] = useState(false);
  const [pdfRefresh, setPdfRefresh] = useState(0);

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

  const requestPdf = async () => {
    setPdfRequesting(true);
    try {
      await api.kecermatanResults.requestPdf(studentId, testId);
      setPdfRefresh((n) => n + 1);
      toast.success(pdfT.toastQueued, { description: pdfT.toastQueuedDescription });
    } catch (err) {
      toast.danger(pdfT.toastFailed, { description: errorText(err, pdfT.failedFallback) });
    } finally {
      setPdfRequesting(false);
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
        <Button variant="secondary" size="sm" isDisabled={pdfRequesting} onPress={() => void requestPdf()}>
          <Icon icon={Download} size="xs" /> {pdfRequesting ? pdfT.requesting : pdfT.request}
        </Button>
      }
    >
      <div className="space-y-6">
        <ExportStatus kind="kecermatan_result_pdf" refreshKey={pdfRefresh} />
        <KecermatanResultView progress={progress} attempts={attempts} loadAttempt={loadAttempt} />
      </div>
    </AppShell>
  );
}
