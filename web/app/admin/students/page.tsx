"use client";

import { useRef, useState } from "react";
import { Button, Chip, toast } from "@heroui/react";
import { ClipboardList, FileUp, KeyRound, ListChecks, Pencil, Plus, Trash2, Users, Wallet } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { DataTable, type Column } from "@/components/data-table";
import { EmptyState } from "@/components/empty-state";
import { GradientAvatar } from "@/components/gradient-avatar";
import { ProgramBadge } from "@/components/status-badge";
import { FormModal } from "@/components/ui/form-modal";
import { SelectInput, TextInput } from "@/components/ui/form";
import {
  findUser,
  formatDate,
  formatIDR,
  getEnrollmentsForUser,
  getResultsForUser,
  getSubtestsForTryout,
  getTryout,
  orders,
  programs,
  students as initialStudents,
  TOPUP_PRODUCT_ID,
  tryouts,
} from "@/lib/mock-data";
import { useSession } from "@/lib/session-context";
import { useStudentsT } from "@/lib/i18n/dict/students";
import type { AppUser, Order, ProgramCode } from "@/lib/types";

const programOptions = programs.map((p) => ({ id: p.code, label: p.name }));
const tryoutOptions = tryouts.map((t) => ({ id: t.id, label: t.title }));
const gradients = [
  ["#4f46e5", "#8b5cf6"],
  ["#0ea5e9", "#22d3ee"],
  ["#10b981", "#34d399"],
  ["#f59e0b", "#f97316"],
] as const;

export default function StudentsPage() {
  const t = useStudentsT();
  const { role } = useSession();
  const [rows, setRows] = useState<AppUser[]>(initialStudents);
  const [editing, setEditing] = useState<AppUser | null>(null);
  const [modalOpen, setModalOpen] = useState(false);
  const [grantTarget, setGrantTarget] = useState<AppUser | null>(null);
  const [grantTryout, setGrantTryout] = useState<string | null>(null);
  const [topUpTarget, setTopUpTarget] = useState<AppUser | null>(null);
  const [topUpAmount, setTopUpAmount] = useState("");
  const [topUpProofName, setTopUpProofName] = useState<string | null>(null);
  const topUpProofRef = useRef<HTMLInputElement>(null);
  const [deleteTarget, setDeleteTarget] = useState<AppUser | null>(null);
  const [detail, setDetail] = useState<AppUser | null>(null);
  const [form, setForm] = useState({ name: "", email: "", program: "" as string });

  const openCreate = () => {
    setEditing(null);
    setForm({ name: "", email: "", program: "" });
    setModalOpen(true);
  };

  const openEdit = (s: AppUser) => {
    setEditing(s);
    setForm({ name: s.name, email: s.email, program: s.program ?? "" });
    setModalOpen(true);
  };

  const handleSave = () => {
    const g = gradients[Math.floor(Math.random() * gradients.length)];
    if (editing) {
      setRows((prev) =>
        prev.map((s) => (s.id === editing.id ? { ...s, name: form.name, email: form.email, program: form.program as ProgramCode } : s)),
      );
      toast.success(t.studentUpdated);
    } else {
      setRows((prev) => [
        {
          id: `u-student-new-${Date.now()}`,
          name: form.name,
          email: form.email,
          role: "student",
          program: form.program as ProgramCode,
          createdAt: new Date().toISOString().slice(0, 10),
          avatarColorFrom: g[0],
          avatarColorTo: g[1],
        },
        ...prev,
      ]);
      toast.success(t.studentAdded);
    }
    setModalOpen(false);
  };

  const columns: Column<AppUser>[] = [
    {
      key: "name",
      header: t.colStudent,
      render: (s) => (
        <div className="flex items-center gap-2.5">
          <GradientAvatar user={s} size="sm" />
          <div>
            <p className="font-medium text-foreground">{s.name}</p>
            <p className="text-xs text-muted">{s.email}</p>
          </div>
        </div>
      ),
    },
    { key: "program", header: t.colProgram, render: (s) => (s.program ? <ProgramBadge code={s.program} /> : "—") },
    { key: "enrollments", header: t.colEnrollments, render: (s) => getEnrollmentsForUser(s.id).length },
    { key: "balance", header: t.colBalance, render: (s) => formatIDR(s.balance ?? 0) },
    { key: "createdAt", header: t.colJoined, render: (s) => formatDate(s.createdAt) },
    {
      key: "actions",
      header: "",
      headerClassName: "text-right",
      className: "text-right",
      render: (s) => (
        <div className="flex justify-end gap-1" onClick={(e) => e.stopPropagation()}>
          {role === "admin" && (
            <Button variant="ghost" size="sm" isIconOnly onPress={() => { setTopUpTarget(s); setTopUpAmount(""); setTopUpProofName(null); }}>
              <Wallet className="size-3.5" />
            </Button>
          )}
          <Button variant="ghost" size="sm" isIconOnly onPress={() => { setGrantTarget(s); setGrantTryout(null); }}>
            <KeyRound className="size-3.5" />
          </Button>
          <Button variant="ghost" size="sm" isIconOnly onPress={() => openEdit(s)}>
            <Pencil className="size-3.5" />
          </Button>
          <Button variant="ghost" size="sm" isIconOnly className="text-danger" onPress={() => setDeleteTarget(s)}>
            <Trash2 className="size-3.5" />
          </Button>
        </div>
      ),
    },
  ];

  return (
    <AppShell
      title={t.pageTitle}
      description={t.studentCount(rows.length)}
      actions={
        <Button variant="primary" size="sm" onPress={openCreate}>
          <Plus className="size-3.5" /> {t.addStudent}
        </Button>
      }
    >
      {rows.length === 0 ? (
        <EmptyState icon={Users} title={t.emptyTitle} />
      ) : (
        <DataTable columns={columns} rows={rows} rowKey={(s) => s.id} onRowClick={setDetail} />
      )}

      <FormModal
        isOpen={modalOpen}
        onOpenChange={setModalOpen}
        title={editing ? t.editStudent : t.addStudent}
        footer={
          <>
            <Button variant="ghost" onPress={() => setModalOpen(false)}>
              {t.cancel}
            </Button>
            <Button variant="primary" onPress={handleSave} isDisabled={!form.name || !form.email}>
              {t.save}
            </Button>
          </>
        }
      >
        <TextInput label={t.fieldFullName} required value={form.name} onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))} />
        <TextInput label={t.fieldEmail} type="email" required value={form.email} onChange={(e) => setForm((f) => ({ ...f, email: e.target.value }))} />
        <SelectInput label={t.fieldProgram} options={programOptions} selectedKey={form.program} onSelectionChange={(k) => setForm((f) => ({ ...f, program: k ?? "" }))} />
      </FormModal>

      <FormModal
        isOpen={!!grantTarget}
        onOpenChange={(open) => !open && setGrantTarget(null)}
        title={t.grantAccessTitle}
        description={t.grantAccessDescription(grantTarget?.name ?? "")}
        footer={
          <>
            <Button variant="ghost" onPress={() => setGrantTarget(null)}>
              {t.cancel}
            </Button>
            <Button
              variant="primary"
              isDisabled={!grantTryout}
              onPress={() => {
                toast.success(t.accessGranted, { description: tryoutOptions.find((opt) => opt.id === grantTryout)?.label });
                setGrantTarget(null);
              }}
            >
              {t.grantAccessConfirm}
            </Button>
          </>
        }
      >
        <SelectInput label={t.fieldSelectTryout} options={tryoutOptions} selectedKey={grantTryout} onSelectionChange={setGrantTryout} />
      </FormModal>

      <FormModal
        isOpen={!!topUpTarget}
        onOpenChange={(open) => !open && setTopUpTarget(null)}
        title={t.topUpTitle}
        description={t.topUpDescription(topUpTarget?.name ?? "", formatIDR(topUpTarget?.balance ?? 0))}
        footer={
          <>
            <Button variant="ghost" onPress={() => setTopUpTarget(null)}>
              {t.cancel}
            </Button>
            <Button
              variant="primary"
              isDisabled={!topUpAmount || Number(topUpAmount) <= 0 || !topUpProofName}
              onPress={() => {
                if (!topUpTarget) return;
                const amount = Number(topUpAmount) || 0;
                const now = new Date().toISOString();
                const order: Order = {
                  id: `order-topup-${Date.now()}`,
                  code: `TRX-2026-${1000 + orders.length + 1}`,
                  userId: topUpTarget.id,
                  productId: TOPUP_PRODUCT_ID,
                  status: "confirmed",
                  paymentMode: "manual",
                  paymentProofUrl: "/mock/payment-proof.jpg",
                  amount,
                  confirmedAt: now,
                  createdAt: now,
                };
                orders.unshift(order);
                const user = findUser(topUpTarget.id);
                const newBalance = (user?.balance ?? 0) + amount;
                if (user) user.balance = newBalance;
                setRows((prev) => prev.map((s) => (s.id === topUpTarget.id ? { ...s, balance: newBalance } : s)));
                toast.success(t.balanceAdded, {
                  description: t.balanceAddedDescription(formatIDR(amount), topUpTarget.name, order.code),
                });
                setTopUpTarget(null);
              }}
            >
              {t.topUpConfirm}
            </Button>
          </>
        }
      >
        <TextInput label={t.fieldTopUpAmount} type="number" min={1} value={topUpAmount} onChange={(e) => setTopUpAmount(e.target.value)} />
        <div>
          <p className="mb-1.5 text-[13px] font-medium text-foreground">{t.fieldPaymentProof}</p>
          <input
            ref={topUpProofRef}
            type="file"
            accept="image/*,.pdf"
            className="hidden"
            onChange={(e) => setTopUpProofName(e.target.files?.[0]?.name ?? null)}
          />
          <button
            type="button"
            onClick={() => topUpProofRef.current?.click()}
            className="flex w-full flex-col items-center gap-1.5 rounded-xl border border-dashed border-border py-6 text-center transition-colors hover:border-accent/50 hover:bg-accent-soft/30"
          >
            <FileUp className="size-4 text-muted" />
            <span className="text-sm font-medium text-foreground">{topUpProofName ?? t.uploadPaymentProof}</span>
            <span className="text-xs text-muted">{t.uploadRequirement}</span>
          </button>
          <p className="mt-2 text-xs text-muted">{t.topUpAutoOrderNote}</p>
        </div>
      </FormModal>

      <FormModal
        isOpen={!!detail}
        onOpenChange={(open) => !open && setDetail(null)}
        title={detail?.name ?? ""}
        description={detail?.email}
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
                <p className="text-xs text-muted">{detail.email}</p>
              </div>
            </div>
            <div className="grid grid-cols-2 gap-3 border-t border-border pt-4 text-sm">
              <div>
                <p className="text-xs text-muted">{t.detailProgram}</p>
                {detail.program ? <ProgramBadge code={detail.program} /> : <p className="font-medium text-foreground">—</p>}
              </div>
              <div>
                <p className="text-xs text-muted">{t.detailJoined}</p>
                <p className="font-medium text-foreground">{formatDate(detail.createdAt)}</p>
              </div>
              <div>
                <p className="text-xs text-muted">{t.detailBalance}</p>
                <p className="font-medium text-foreground">{formatIDR(detail.balance ?? 0)}</p>
              </div>
            </div>
            <div>
              <p className="mb-2 flex items-center gap-1.5 text-[13px] font-medium text-foreground">
                <ClipboardList className="size-3.5 text-accent" />
                {t.ownedTryouts(getEnrollmentsForUser(detail.id).length)}
              </p>
              {getEnrollmentsForUser(detail.id).length === 0 ? (
                <p className="text-sm text-muted">{t.noOwnedTryouts}</p>
              ) : (
                <ul className="space-y-2">
                  {getEnrollmentsForUser(detail.id).map((e) => {
                    const tryout = getTryout(e.tryoutId);
                    if (!tryout) return null;
                    const result = getResultsForUser(detail.id).find((r) => r.tryoutId === tryout.id);
                    const subtests = getSubtestsForTryout(tryout.id);
                    return (
                      <li
                        key={e.id}
                        className="flex items-center justify-between gap-3 rounded-lg border border-border px-3.5 py-2.5"
                      >
                        <div className="min-w-0">
                          <p className="truncate text-sm font-medium text-foreground">{tryout.title}</p>
                          <p className="flex items-center gap-1.5 text-xs text-muted">
                            <ListChecks className="size-3" /> {t.subtestCountUntil(subtests.length, formatDate(e.activeUntil))}
                          </p>
                        </div>
                        {result ? (
                          <Chip size="sm" className="bg-success-soft text-success-soft-foreground shrink-0 font-medium">
                            {t.resultDone(result.totalScore)}
                          </Chip>
                        ) : (
                          <Chip size="sm" className="bg-default text-default-foreground shrink-0 font-medium">
                            {t.resultNotDone}
                          </Chip>
                        )}
                      </li>
                    );
                  })}
                </ul>
              )}
            </div>
          </>
        )}
      </FormModal>

      <FormModal
        isOpen={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title={t.deleteTitle}
        description={deleteTarget?.name}
        footer={
          <>
            <Button variant="ghost" onPress={() => setDeleteTarget(null)}>
              {t.cancel}
            </Button>
            <Button
              variant="danger"
              onPress={() => {
                setRows((prev) => prev.filter((s) => s.id !== deleteTarget?.id));
                toast.success(t.studentDeleted);
                setDeleteTarget(null);
              }}
            >
              {t.deleteConfirm}
            </Button>
          </>
        }
      >
        <p className="text-sm text-muted">{t.deleteDescription}</p>
      </FormModal>
    </AppShell>
  );
}
