"use client";

import { useEffect, useRef, useState } from "react";
import { Button, toast } from "@heroui/react";
import { KeyRound } from "lucide-react";
import { Icon } from "@/components/icon";
import { StatusChip } from "@/components/status-badge";
import { FormModal } from "@/components/ui/form-modal";
import { SelectInput, TextInput, TextareaInput } from "@/components/ui/form";
import {
  api,
  ApiError,
  errorText,
  type AdminUser,
  type KecermatanTestSummary,
  type StudentAccess,
  type TryoutSummary,
} from "@/lib/api/client";
import { useStudentsT } from "@/lib/i18n/dict/students";

// A fresh key per submit attempt, reused on a retry so a network error cannot grant twice.
function newIdempotencyKey(): string {
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  return Array.from(bytes, (byte) => byte.toString(16).padStart(2, "0")).join("");
}

/** Remount it (change its key) to reload; state starts as loading. */
export function StudentAccessSection({ student }: { student: AdminUser }) {
  const t = useStudentsT();
  const [rows, setRows] = useState<StudentAccess[] | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    let cancelled = false;
    api.students
      .access(student.id, { limit: 100 })
      .then((page) => {
        if (!cancelled) setRows(page.access);
      })
      .catch(() => {
        if (!cancelled) setFailed(true);
      });
    return () => {
      cancelled = true;
    };
  }, [student.id]);

  if (failed) return <p className="text-caption text-danger">{t.accessFailed}</p>;
  if (rows === null) return <p className="text-caption text-muted">{t.loading}</p>;
  if (rows.length === 0) return <p className="text-caption text-muted">{t.accessEmpty}</p>;
  return (
    <ul className="space-y-2">
      {rows.map((row) => (
        <li key={row.enrollment_id} className="flex items-center justify-between gap-3 rounded-control border border-border px-4 py-3">
          <div className="min-w-0">
            <p className="truncate text-small font-medium text-foreground">{row.title}</p>
            <p className="truncate text-label text-muted">
              {row.kind === "tryout" ? t.accessKindTryout : t.accessKindKecermatan}
              {" · "}
              {row.kind === "tryout" && row.active_until
                ? t.accessUntil(new Date(row.active_until).toLocaleDateString())
                : t.accessCredits(row.attempts_used, row.attempts_purchased ?? 0)}
              {" · "}
              {t.accessSource[row.source_type] ?? row.source_type}
            </p>
          </div>
          <StatusChip tone={row.active ? "success" : "neutral"} dot>
            {row.active ? t.accessActive : t.accessInactive}
          </StatusChip>
        </li>
      ))}
    </ul>
  );
}

export function GrantAccessButton({ onPress }: { onPress: () => void }) {
  const t = useStudentsT();
  return (
    <Button variant="secondary" size="sm" onPress={onPress}>
      <Icon icon={KeyRound} size="xs" /> {t.grantAction}
    </Button>
  );
}

type Kind = "tryout" | "kecermatan";

export function GrantAccessModal({
  student,
  onClose,
  onGranted,
}: {
  student: AdminUser | null;
  onClose: () => void;
  onGranted: () => void;
}) {
  return student ? <GrantAccessForm key={student.id} student={student} onClose={onClose} onGranted={onGranted} /> : null;
}

function GrantAccessForm({
  student,
  onClose,
  onGranted,
}: {
  student: AdminUser;
  onClose: () => void;
  onGranted: () => void;
}) {
  const t = useStudentsT();
  const [kind, setKind] = useState<Kind>("tryout");
  const [tryouts, setTryouts] = useState<TryoutSummary[]>([]);
  const [tests, setTests] = useState<KecermatanTestSummary[]>([]);
  const [testId, setTestId] = useState("");
  const [attempts, setAttempts] = useState("1");
  const [reason, setReason] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);
  const idempotencyKey = useRef<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    api.tryouts
      .list({ status: "published", limit: 100 })
      .then((page) => !cancelled && setTryouts(page.tryouts))
      .catch(() => !cancelled && setTryouts([]));
    api.kecermatan
      .list({ limit: 100 })
      .then((page) => !cancelled && setTests(page.tests))
      .catch(() => !cancelled && setTests([]));
    return () => {
      cancelled = true;
    };
  }, []);

  const options = (kind === "tryout" ? tryouts : tests).map((test) => ({ id: test.id, label: test.title }));

  const submit = async () => {
    idempotencyKey.current ??= newIdempotencyKey();
    setSaving(true);
    setErrors({});
    try {
      await api.students.grant(student.id, {
        ...(kind === "tryout"
          ? { tryout_id: testId }
          : { kecermatan_test_id: testId, attempts: Number.parseInt(attempts, 10) || 0 }),
        reason: reason.trim(),
        idempotency_key: idempotencyKey.current,
      });
      toast.success(t.grantSaved);
      idempotencyKey.current = null;
      onGranted();
      onClose();
    } catch (err) {
      if (err instanceof ApiError && err.fields) setErrors(err.fields);
      toast.danger(t.grantFailed, { description: errorText(err, t.loadErrorDescription) });
    } finally {
      setSaving(false);
    }
  };

  return (
    <FormModal
      isOpen
      onOpenChange={(open) => !open && onClose()}
      title={t.grantTitle}
      description={t.grantDescription(student.name)}
      footer={
        <>
          <Button variant="ghost" onPress={onClose}>
            {t.cancel}
          </Button>
          <Button variant="primary" onPress={submit} isDisabled={saving || testId === "" || reason.trim() === ""}>
            {saving ? t.grantSaving : t.grantSubmit}
          </Button>
        </>
      }
    >
      <SelectInput
        label={t.grantKind}
        options={[
          { id: "tryout", label: t.grantKindTryout },
          { id: "kecermatan", label: t.grantKindKecermatan },
        ]}
        selectedKey={kind}
        onSelectionChange={(key) => {
          setKind(key === "kecermatan" ? "kecermatan" : "tryout");
          setTestId("");
          idempotencyKey.current = null;
        }}
      />
      {options.length === 0 ? (
        <p className="text-caption text-muted">{t.grantNoTests}</p>
      ) : (
        <SelectInput
          label={t.grantTest}
          placeholder={t.grantTestPlaceholder}
          options={options}
          selectedKey={testId === "" ? null : testId}
          onSelectionChange={(key) => {
            setTestId(key ? String(key) : "");
            idempotencyKey.current = null;
          }}
          error={errors.tryout_id}
        />
      )}
      {kind === "kecermatan" && (
        <TextInput
          label={t.grantAttempts}
          inputMode="numeric"
          required
          value={attempts}
          onChange={(event) => setAttempts(event.target.value)}
          description={t.grantAttemptsHint}
          error={errors.attempts}
        />
      )}
      <TextareaInput
        label={t.grantReason}
        required
        rows={3}
        placeholder={t.grantReasonPlaceholder}
        value={reason}
        onChange={(event) => setReason(event.target.value)}
        error={errors.reason}
      />
    </FormModal>
  );
}
