"use client";

import { useEffect, useRef, useState } from "react";
import { Button, toast } from "@heroui/react";
import { CloudOff, Search, Users, Wallet } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { DataTable, type Column } from "@/components/data-table";
import { EmptyState } from "@/components/empty-state";
import { GradientAvatar } from "@/components/gradient-avatar";
import { FormModal } from "@/components/ui/form-modal";
import { SelectInput, TextInput, TextareaInput } from "@/components/ui/form";
import { api, ApiError, errorText, type WalletEntry, type WalletStudent } from "@/lib/api/client";
import { formatIDR, formatSignedIDR } from "@/lib/format-money";
import { useSession } from "@/lib/session-context";
import { useStudentsT } from "@/lib/i18n/dict/students";

const PAGE_SIZE = 50;
const LEDGER_SIZE = 20;

// A fresh key per submit attempt, reused on a retry so a network error cannot credit twice.
function newIdempotencyKey(): string {
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  return Array.from(bytes, (byte) => byte.toString(16).padStart(2, "0")).join("");
}

const joined = (iso: string) => new Date(iso).toLocaleDateString();

export default function StudentsPage() {
  const t = useStudentsT();
  const { role } = useSession();

  const [rows, setRows] = useState<WalletStudent[] | null>(null);
  const [total, setTotal] = useState(0);
  const [loadFailed, setLoadFailed] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [moreFailed, setMoreFailed] = useState("");
  const [reloadKey, setReloadKey] = useState(0);
  const [search, setSearch] = useState("");
  const [query, setQuery] = useState("");

  const [detail, setDetail] = useState<WalletStudent | null>(null);
  const [detailEntries, setDetailEntries] = useState<WalletEntry[] | null>(null);
  const [detailFailed, setDetailFailed] = useState(false);

  const [adjustTarget, setAdjustTarget] = useState<WalletStudent | null>(null);
  const [adjustType, setAdjustType] = useState<"credit" | "debit">("credit");
  const [adjustAmount, setAdjustAmount] = useState("");
  const [adjustReason, setAdjustReason] = useState("");
  const [formErrors, setFormErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);
  const idempotencyKey = useRef<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    api.students
      .list({ q: query, limit: PAGE_SIZE })
      .then((page) => {
        if (cancelled) return;
        setRows(page.students);
        setTotal(page.total);
      })
      .catch(() => {
        if (!cancelled) setLoadFailed(true);
      });
    return () => {
      cancelled = true;
    };
  }, [query, reloadKey]);

  const reload = (next: string) => {
    setRows(null);
    setLoadFailed(false);
    setMoreFailed("");
    if (next === query) {
      setReloadKey((key) => key + 1);
      return;
    }
    setQuery(next);
  };

  const loadMore = async () => {
    setLoadingMore(true);
    setMoreFailed("");
    try {
      const page = await api.students.list({ q: query, limit: PAGE_SIZE, offset: rows?.length ?? 0 });
      setRows((prev) => {
        const seen = new Set((prev ?? []).map((row) => row.id));
        return [...(prev ?? []), ...page.students.filter((row) => !seen.has(row.id))];
      });
      setTotal(page.total);
    } catch (err) {
      setMoreFailed(errorText(err, t.loadErrorDescription));
    } finally {
      setLoadingMore(false);
    }
  };

  const loadLedger = async (studentId: string) => {
    setDetailEntries(null);
    setDetailFailed(false);
    try {
      const page = await api.students.entries(studentId, { limit: LEDGER_SIZE });
      setDetailEntries(page.entries);
    } catch {
      setDetailFailed(true);
    }
  };

  const openDetail = (student: WalletStudent) => {
    setDetail(student);
    void loadLedger(student.id);
  };

  const openAdjust = (student: WalletStudent) => {
    setDetail(null);
    setAdjustTarget(student);
    setAdjustType("credit");
    setAdjustAmount("");
    setAdjustReason("");
    setFormErrors({});
    idempotencyKey.current = null;
  };

  const closeAdjust = () => {
    setAdjustTarget(null);
    setFormErrors({});
    idempotencyKey.current = null;
  };

  const submitAdjustment = async () => {
    const target = adjustTarget;
    if (!target) return;
    idempotencyKey.current ??= newIdempotencyKey();
    setSaving(true);
    setFormErrors({});
    try {
      const result = await api.students.adjust(target.id, {
        type: adjustType,
        amount: adjustAmount.trim(),
        reason: adjustReason.trim(),
        idempotency_key: idempotencyKey.current,
      });
      setRows((prev) => prev?.map((row) => (row.id === target.id ? { ...row, balance: result.balance } : row)) ?? []);
      toast.success(t.adjustSaved, { description: t.adjustSavedDescription(formatIDR(result.balance)) });
      setAdjustTarget(null);
      idempotencyKey.current = null;
    } catch (err) {
      if (err instanceof ApiError && err.fields) setFormErrors(err.fields);
      toast.danger(t.adjustFailed, { description: errorText(err, t.loadErrorDescription) });
    } finally {
      setSaving(false);
    }
  };

  const columns: Column<WalletStudent>[] = [
    {
      key: "name",
      header: t.colStudent,
      render: (student) => (
        <div className="flex items-center gap-2.5">
          <GradientAvatar user={student} size="sm" />
          <div>
            <p className="font-medium text-foreground">{student.name}</p>
            <p className="text-xs text-muted">{student.email ?? "—"}</p>
          </div>
        </div>
      ),
    },
    { key: "phone", header: t.colPhone, render: (student) => <span className="text-muted">{student.phone ?? "—"}</span> },
    { key: "balance", header: t.colBalance, render: (student) => <span className="font-medium">{formatIDR(student.balance)}</span> },
    { key: "joined", header: t.colJoined, render: (student) => joined(student.created_at) },
    {
      key: "actions",
      header: "",
      headerClassName: "text-right",
      className: "text-right",
      render: (student) => (
        <div className="flex justify-end gap-1" onClick={(event) => event.stopPropagation()}>
          {role === "admin" && (
            <Button variant="ghost" size="sm" isIconOnly onPress={() => openAdjust(student)} aria-label={t.adjustTitle}>
              <Wallet className="size-3.5" />
            </Button>
          )}
        </div>
      ),
    },
  ];

  return (
    <AppShell title={t.pageTitle} description={t.studentCount(total)}>
      <form
        className="mb-4 flex flex-wrap items-end gap-2"
        onSubmit={(event) => {
          event.preventDefault();
          reload(search.trim());
        }}
      >
        <TextInput
          className="w-full max-w-sm"
          placeholder={t.searchPlaceholder}
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
        <Button variant="secondary" size="sm" type="submit">
          <Search className="size-3.5" /> {t.searchAction}
        </Button>
        {query !== "" && (
          <Button
            variant="ghost"
            size="sm"
            onPress={() => {
              setSearch("");
              reload("");
            }}
          >
            {t.searchClear}
          </Button>
        )}
      </form>

      {loadFailed ? (
        <EmptyState
          icon={CloudOff}
          title={t.loadErrorTitle}
          description={t.loadErrorDescription}
          action={
            <Button variant="primary" size="sm" onPress={() => reload(query)}>
              {t.retry}
            </Button>
          }
        />
      ) : rows === null ? (
        <p className="py-14 text-center text-sm text-muted">{t.loading}</p>
      ) : rows.length === 0 ? (
        <EmptyState
          icon={Users}
          title={query === "" ? t.emptyTitle : t.noMatchTitle}
          description={query === "" ? t.emptyDescription : t.noMatchDescription}
        />
      ) : (
        <>
          <DataTable columns={columns} rows={rows} rowKey={(student) => student.id} onRowClick={openDetail} />
          {moreFailed !== "" && <p className="mt-3 text-center text-sm text-danger">{moreFailed}</p>}
          {rows.length < total && (
            <div className="mt-4 flex justify-center">
              <Button variant="secondary" size="sm" onPress={loadMore} isDisabled={loadingMore}>
                {loadingMore ? t.loadingMore : t.loadMore}
              </Button>
            </div>
          )}
        </>
      )}

      <FormModal
        isOpen={!!adjustTarget}
        onOpenChange={(open) => !open && closeAdjust()}
        title={t.adjustTitle}
        description={t.adjustDescription(adjustTarget?.name ?? "", formatIDR(adjustTarget?.balance ?? "0.00"))}
        footer={
          <>
            <Button variant="ghost" onPress={closeAdjust}>
              {t.cancel}
            </Button>
            <Button
              variant="primary"
              onPress={submitAdjustment}
              isDisabled={saving || adjustAmount.trim() === "" || adjustReason.trim() === ""}
            >
              {saving ? t.adjustSaving : t.adjustSubmit}
            </Button>
          </>
        }
      >
        <SelectInput
          label={t.adjustType}
          options={[
            { id: "credit", label: t.adjustTypeCredit },
            { id: "debit", label: t.adjustTypeDebit },
          ]}
          selectedKey={adjustType}
          onSelectionChange={(key) => setAdjustType(key === "debit" ? "debit" : "credit")}
          error={formErrors.type}
        />
        <TextInput
          label={t.adjustAmount}
          required
          inputMode="decimal"
          placeholder="50000.00"
          description={t.adjustAmountHint}
          value={adjustAmount}
          onChange={(event) => setAdjustAmount(event.target.value)}
          error={formErrors.amount}
        />
        <TextareaInput
          label={t.adjustReason}
          required
          rows={3}
          placeholder={t.adjustReasonPlaceholder}
          value={adjustReason}
          onChange={(event) => setAdjustReason(event.target.value)}
          error={formErrors.reason}
        />
      </FormModal>

      <FormModal
        isOpen={!!detail}
        onOpenChange={(open) => !open && setDetail(null)}
        title={detail?.name ?? ""}
        description={detail?.email ?? ""}
        size="lg"
        footer={
          <Button variant="ghost" onPress={() => setDetail(null)}>
            {t.close}
          </Button>
        }
      >
        {detail && (
          <>
            <div className="flex items-center gap-3">
              <GradientAvatar user={detail} size="lg" />
              <div>
                <p className="text-sm font-semibold text-foreground">{detail.name}</p>
                <p className="text-xs text-muted">{detail.email ?? "—"}</p>
              </div>
            </div>
            <div className="grid grid-cols-2 gap-3 border-t border-border pt-4 text-sm">
              <div>
                <p className="text-xs text-muted">{t.detailJoined}</p>
                <p className="font-medium text-foreground">{joined(detail.created_at)}</p>
              </div>
              <div>
                <p className="text-xs text-muted">{t.detailBalance}</p>
                <p className="font-medium text-foreground">{formatIDR(detail.balance)}</p>
              </div>
              <div>
                <p className="text-xs text-muted">{t.detailPhone}</p>
                <p className="font-medium text-foreground">{detail.phone ?? "—"}</p>
              </div>
            </div>
            <div>
              <p className="mb-2 text-[13px] font-medium text-foreground">{t.detailLedgerTitle}</p>
              {detailFailed ? (
                <p className="text-sm text-danger">{t.detailLedgerFailed}</p>
              ) : detailEntries === null ? (
                <p className="text-sm text-muted">{t.loading}</p>
              ) : detailEntries.length === 0 ? (
                <p className="text-sm text-muted">{t.detailLedgerEmpty}</p>
              ) : (
                <ul className="space-y-2">
                  {detailEntries.map((entry) => (
                    <li
                      key={entry.id}
                      className="flex items-center justify-between gap-3 rounded-lg border border-border px-3.5 py-2.5"
                    >
                      <div className="min-w-0">
                        <p className="flex items-center gap-1.5 text-sm font-medium text-foreground">
                          {t.ledgerType[entry.kind] ?? entry.kind}
                          <span className="text-xs font-normal text-muted">{new Date(entry.created_at).toLocaleString()}</span>
                        </p>
                        <p className="truncate text-xs text-muted">{entry.reason === "" ? "—" : entry.reason}</p>
                      </div>
                      <div className="shrink-0 text-right">
                        <p className="font-mono text-sm font-medium text-foreground">{formatSignedIDR(entry.amount)}</p>
                        <p className="font-mono text-xs text-muted">{formatIDR(entry.balance_after)}</p>
                      </div>
                    </li>
                  ))}
                </ul>
              )}
            </div>
            {role === "admin" && (
              <div className="border-t border-border pt-4">
                <Button variant="secondary" size="sm" onPress={() => openAdjust(detail)}>
                  <Wallet className="size-3.5" /> {t.adjustTitle}
                </Button>
              </div>
            )}
          </>
        )}
      </FormModal>
    </AppShell>
  );
}
