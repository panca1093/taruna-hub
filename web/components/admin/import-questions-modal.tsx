"use client";

import { useRef, useState } from "react";
import { Button, toast } from "@heroui/react";
import { CheckCircle2, Download, FileSpreadsheet, FileUp } from "lucide-react";
import { FormModal } from "@/components/ui/form-modal";
import { DifficultyBadge } from "@/components/status-badge";
import { useAdminImportQuestionsModalT } from "@/lib/i18n/dict/admin-import-questions-modal";

const previewRows = [
  { subject: "Matematika", content: "Hasil dari 15% dari 200 adalah ...", difficulty: "easy" as const },
  { subject: "IPA", content: "Proses perubahan wujud dari gas ke cair disebut ...", difficulty: "medium" as const },
  { subject: "Bahasa Indonesia", content: "Kalimat berikut yang menggunakan majas personifikasi adalah ...", difficulty: "hard" as const },
];

export function ImportQuestionsModal({
  isOpen,
  onOpenChange,
  onImported,
}: {
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  onImported: (count: number) => void;
}) {
  const t = useAdminImportQuestionsModalT();
  const [fileName, setFileName] = useState<string | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);

  const reset = () => setFileName(null);

  return (
    <FormModal
      isOpen={isOpen}
      onOpenChange={(open) => {
        onOpenChange(open);
        if (!open) reset();
      }}
      title={t.title}
      description={t.description}
      size="lg"
      footer={
        <>
          <Button variant="ghost" onPress={() => onOpenChange(false)}>
            {t.cancel}
          </Button>
          <Button
            variant="primary"
            isDisabled={!fileName}
            onPress={() => {
              onImported(previewRows.length);
              onOpenChange(false);
              reset();
            }}
          >
            {t.confirmImport}
          </Button>
        </>
      }
    >
      <button
        type="button"
        className="flex items-center gap-3 rounded-lg border border-border bg-surface-secondary px-4 py-3 text-left transition-colors hover:border-accent/40"
        onClick={() => toast.info(t.toastTemplateDownloaded, { description: "bank-soal-template.xlsx" })}
      >
        <Download className="size-4 text-accent" />
        <div>
          <p className="text-sm font-medium text-foreground">{t.downloadTemplateTitle}</p>
          <p className="text-xs text-muted">{t.downloadTemplateDescription}</p>
        </div>
      </button>

      <input
        ref={fileRef}
        type="file"
        accept=".xlsx,.csv,.docx"
        className="hidden"
        onChange={(e) => setFileName(e.target.files?.[0]?.name ?? null)}
      />
      <button
        type="button"
        onClick={() => fileRef.current?.click()}
        className="flex flex-col items-center gap-2 rounded-xl border border-dashed border-border py-8 text-center transition-colors hover:border-accent/50 hover:bg-accent-soft/30"
      >
        <FileUp className="size-5 text-muted" />
        <span className="text-sm font-medium text-foreground">{fileName ?? t.uploadPrompt}</span>
      </button>

      {fileName && (
        <div>
          <p className="mb-2 flex items-center gap-1.5 text-xs font-medium text-success">
            <CheckCircle2 className="size-3.5" /> {t.detected(previewRows.length)}
          </p>
          <div className="overflow-hidden rounded-lg border border-border">
            <table className="w-full text-xs">
              <thead className="bg-surface-secondary text-muted">
                <tr>
                  <th className="px-3 py-2 text-left font-medium">{t.columns.subject}</th>
                  <th className="px-3 py-2 text-left font-medium">{t.columns.content}</th>
                  <th className="px-3 py-2 text-left font-medium">{t.columns.difficulty}</th>
                </tr>
              </thead>
              <tbody>
                {previewRows.map((r, i) => (
                  <tr key={i} className="border-t border-border">
                    <td className="px-3 py-2 text-muted">{r.subject}</td>
                    <td className="max-w-64 truncate px-3 py-2 text-foreground">{r.content}</td>
                    <td className="px-3 py-2">
                      <DifficultyBadge difficulty={r.difficulty} />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {!fileName && (
        <p className="flex items-center gap-1.5 text-xs text-muted">
          <FileSpreadsheet className="size-3.5" /> {t.requiredColumns}
        </p>
      )}
    </FormModal>
  );
}
