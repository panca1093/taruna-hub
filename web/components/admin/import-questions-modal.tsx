"use client";

import { useRef, useState } from "react";
import { Button, toast } from "@heroui/react";
import { CheckCircle2, Download, FileSpreadsheet, FileUp, RotateCcw, XCircle } from "lucide-react";
import { FormModal } from "@/components/ui/form-modal";
import {
  api,
  ApiError,
  errorText,
  type QuestionImportPreview,
  type QuestionTemplateFormat,
} from "@/lib/api/client";
import { cn } from "@/lib/cn";
import { useAdminImportQuestionsModalT } from "@/lib/i18n/dict/admin-import-questions-modal";

const templateFormats: QuestionTemplateFormat[] = ["xlsx", "csv", "docx"];

export function ImportQuestionsModal({
  isOpen,
  onOpenChange,
  onImported,
}: {
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  onImported: () => void;
}) {
  const t = useAdminImportQuestionsModalT();
  const [file, setFile] = useState<File | null>(null);
  const [preview, setPreview] = useState<QuestionImportPreview | null>(null);
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [pending, setPending] = useState(false);
  const [dragging, setDragging] = useState(false);
  const [inlineError, setInlineError] = useState<string | null>(null);
  const [downloading, setDownloading] = useState<QuestionTemplateFormat | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);
  const latestRequest = useRef(0);

  const reset = () => {
    latestRequest.current++;
    setFile(null);
    setPreview(null);
    setSelected(new Set());
    setInlineError(null);
    setPending(false);
    setDragging(false);
  };

  // The server answers with `fields.file` or `fields.rows`; that copy is what the user needs.
  const showError = (err: unknown, fallback: string) => {
    const message =
      err instanceof ApiError
        ? (err.fields?.file ?? err.fields?.rows ?? err.message)
        : errorText(err, fallback);
    setInlineError(message);
    toast.danger(t.uploadFailedTitle, { description: message });
  };

  const chooseFile = async (picked: File | undefined) => {
    if (!picked || pending) return;
    // Clearing the input lets the same file be chosen again after a failed preview.
    if (fileRef.current) fileRef.current.value = "";
    setInlineError(null);
    setFile(picked);
    setPreview(null);
    setSelected(new Set());
    setPending(true);
    const request = ++latestRequest.current;
    try {
      const result = await api.questions.importPreview(picked);
      if (request !== latestRequest.current) return;
      setPreview(result);
      setSelected(new Set(result.rows.filter((row) => row.valid).map((row) => row.row)));
    } catch (err) {
      if (request !== latestRequest.current) return;
      setFile(null);
      showError(err, t.uploadFailedDescription);
    } finally {
      if (request === latestRequest.current) setPending(false);
    }
  };

  const toggleRow = (row: number) => {
    setSelected((previous) => {
      const next = new Set(previous);
      if (next.has(row)) next.delete(row);
      else next.add(row);
      return next;
    });
  };

  const submit = async () => {
    if (!file || selected.size === 0) return;
    setPending(true);
    try {
      const rows = [...selected].sort((a, b) => a - b);
      const result = await api.questions.importCommit(file, rows);
      toast.success(t.importedTitle, {
        description: t.importedDescription(result.created, result.already_imported),
      });
      onOpenChange(false);
      reset();
      onImported();
    } catch (err) {
      showError(err, t.uploadFailedDescription);
    } finally {
      setPending(false);
    }
  };

  const downloadTemplate = async (format: QuestionTemplateFormat) => {
    if (downloading !== null) return;
    setDownloading(format);
    try {
      const blob = await api.questions.importTemplate(format);
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = `template-soal.${format}`;
      document.body.appendChild(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(url);
      toast.success(t.toastTemplateDownloaded, { description: `template-soal.${format}` });
    } catch (err) {
      showError(err, t.downloadFailed);
    } finally {
      setDownloading(null);
    }
  };

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
            isDisabled={pending || !preview || selected.size === 0}
            onPress={() => void submit()}
          >
            {pending ? t.importing : t.confirmImport}
          </Button>
        </>
      }
    >
      <div className="grid gap-3 sm:grid-cols-3">
        {templateFormats.map((format) => (
          <button
            key={format}
            type="button"
            className="flex items-center gap-3 rounded-tile border border-border bg-surface-secondary px-4 py-3 text-left transition-colors hover:border-accent/40"
            onClick={() => void downloadTemplate(format)}
            disabled={downloading !== null}
          >
            <Download className="size-4 shrink-0 text-accent" />
            <div>
              <p className="text-small font-medium text-foreground">{t.downloadTemplateTitle}</p>
              <p className="text-caption text-muted uppercase">{format}</p>
            </div>
          </button>
        ))}
      </div>
      <p className="-mt-2 text-caption text-muted">{t.downloadTemplateDescription}</p>

      <input
        ref={fileRef}
        type="file"
        accept=".xlsx,.csv,.docx"
        className="hidden"
        onChange={(e) => void chooseFile(e.target.files?.[0])}
      />
      <button
        type="button"
        onClick={() => fileRef.current?.click()}
        onDragOver={(e) => {
          e.preventDefault();
          setDragging(true);
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={(e) => {
          e.preventDefault();
          setDragging(false);
          void chooseFile(e.dataTransfer.files?.[0]);
        }}
        className={cn(
          "flex flex-col items-center gap-2 rounded-tile border border-dashed border-border py-8 text-center transition-colors hover:border-accent/50 hover:bg-accent-soft/30",
          dragging && "border-accent bg-accent-soft/40",
        )}
      >
        <FileUp className="size-5 text-rose" />
        <span className="text-small font-medium text-foreground">{file?.name ?? t.uploadPrompt}</span>
        {file && !pending && (
          <span className="flex items-center gap-1.5 text-caption text-muted">
            <RotateCcw className="size-3" /> {t.replaceFile}
          </span>
        )}
      </button>

      {inlineError && (
        <p className="flex items-start gap-1.5 rounded-control border border-danger/30 bg-danger-soft px-3 py-2 text-caption text-danger-soft-foreground">
          <XCircle className="mt-px size-4 shrink-0" /> {inlineError}
        </p>
      )}

      {pending && !preview && <p className="text-center text-caption text-muted">{t.reading}</p>}

      {preview && (
        <div>
          <p className="mb-2 flex flex-wrap items-center gap-1.5 text-caption font-medium text-success">
            <CheckCircle2 className="size-4" /> {t.detected(preview.valid, preview.invalid)}
            <span className="font-normal text-muted">· {t.selected(selected.size)}</span>
          </p>
          <div className="max-h-72 overflow-auto rounded-tile border border-border">
            <table className="w-full text-caption">
              <thead className="sticky top-0 bg-surface-secondary text-muted">
                <tr>
                  <th className="w-8 px-2 py-2" />
                  <th className="px-3 py-2 text-left font-medium">{t.columns.row}</th>
                  <th className="px-3 py-2 text-left font-medium">{t.columns.status}</th>
                  <th className="px-3 py-2 text-left font-medium">{t.columns.content}</th>
                  <th className="px-3 py-2 text-left font-medium">{t.columns.error}</th>
                </tr>
              </thead>
              <tbody>
                {preview.rows.map((row) => (
                  <tr key={row.row} className={cn("border-t border-border", !row.valid && "bg-danger-soft/40")}>
                    <td className="px-2 py-2 align-top">
                      <input
                        type="checkbox"
                        className="size-3.5"
                        aria-label={t.selectRow(row.row)}
                        checked={selected.has(row.row)}
                        disabled={!row.valid || pending}
                        onChange={() => toggleRow(row.row)}
                      />
                    </td>
                    <td className="px-3 py-2 align-top font-mono text-muted">{row.row}</td>
                    <td className="px-3 py-2 align-top">
                      {row.valid ? (
                        <span className="flex items-center gap-1 text-success">
                          <CheckCircle2 className="size-4" /> {t.valid}
                        </span>
                      ) : (
                        <span className="flex items-center gap-1 text-danger">
                          <XCircle className="size-4" /> {t.invalid}
                        </span>
                      )}
                    </td>
                    <td className="max-w-64 truncate px-3 py-2 align-top text-foreground">
                      {row.preview?.content ?? ""}
                    </td>
                    <td className="px-3 py-2 align-top text-muted">{row.errors[0]?.message ?? ""}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {!file && (
        <p className="flex items-center gap-1.5 text-caption text-muted">
          <FileSpreadsheet className="size-4" /> {t.requiredColumns}
        </p>
      )}
    </FormModal>
  );
}
