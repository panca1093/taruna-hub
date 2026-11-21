"use client";

import { GradientAvatar } from "@/components/gradient-avatar";
import { KecermatanResultView } from "@/components/kecermatan-result-view";
import { PageHeading } from "@/components/page-heading";
import { api, type KecermatanStaffStudentDetail } from "@/lib/api/client";
import { useKecermatanResultsT } from "@/lib/i18n/dict/results";

const loadAttempt = (sessionId: string) => api.kecermatanResults.get(sessionId);

/** The one-student kecermatan result as it is printed: the single PDF and every page of the bulk PDF render this. */
export function KecermatanResultCaptureSheet({ detail, ref }: { detail: KecermatanStaffStudentDetail; ref?: React.Ref<HTMLDivElement> }) {
  const t = useKecermatanResultsT();
  const { student, test, progress, attempts } = detail;
  return (
    <div ref={ref} className="space-y-6 bg-background p-8 text-foreground">
      <PageHeading
        title={student.name}
        description={`${test.title} · ${t.detailDescription(test.program_code, detail.remaining_attempts)}`}
        leading={<GradientAvatar user={student} size="lg" eager />}
        size="compact"
      />
      <KecermatanResultView progress={progress} attempts={attempts} loadAttempt={loadAttempt} initialOpenId={null} />
    </div>
  );
}
