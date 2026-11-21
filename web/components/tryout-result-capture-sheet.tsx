"use client";

import { GradientAvatar } from "@/components/gradient-avatar";
import { PageHeading } from "@/components/page-heading";
import { TryoutResultView } from "@/components/tryout-result-view";
import type { StaffResultAttempt, StaffResultStudentDetail } from "@/lib/api/client";
import { useResultsT } from "@/lib/i18n/dict/results";

export const viewOf = (detail: StaffResultStudentDetail, attempt: StaffResultAttempt) => ({
  score: attempt.score,
  subtests: attempt.subtests,
  ranking: detail.ranking,
  attempts: detail.attempts,
  currentSessionId: attempt.session_id,
});

/** The one-student tryout result as it is printed: the single PDF and every page of the bulk PDF render this. */
export function TryoutResultCaptureSheet({ detail, attempt, ref }: { detail: StaffResultStudentDetail; attempt: StaffResultAttempt; ref?: React.Ref<HTMLDivElement> }) {
  const t = useResultsT();
  return (
    <div ref={ref} className="space-y-6 bg-background p-8 text-foreground">
      <PageHeading
        title={detail.student.name}
        description={t.detailDescription(detail.tryout.title, detail.tryout.program_code)}
        leading={<GradientAvatar user={detail.student} size="lg" eager />}
        size="compact"
      />
      <TryoutResultView {...viewOf(detail, attempt)} />
    </div>
  );
}
