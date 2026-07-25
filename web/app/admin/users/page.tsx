"use client";

import { useState } from "react";
import { Button, Chip, toast } from "@heroui/react";
import { Pencil, Plus, ShieldCheck, Trash2, UserCog } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { RequireRole } from "@/components/require-role";
import { DataTable, type Column } from "@/components/data-table";
import { GradientAvatar } from "@/components/gradient-avatar";
import { FormModal } from "@/components/ui/form-modal";
import { SelectInput, TextInput } from "@/components/ui/form";
import { admins, formatDate, operators as initialOperators } from "@/lib/mock-data";
import { useSession } from "@/lib/session-context";
import { useUsersT } from "@/lib/i18n/dict/users";
import type { AppUser, Role } from "@/lib/types";

const gradients = [
  ["#4f46e5", "#8b5cf6"],
  ["#0ea5e9", "#22d3ee"],
  ["#f43f5e", "#ec4899"],
] as const;

export default function UsersPage() {
  const t = useUsersT();
  const roleOptions = [
    { id: "operator", label: t.roleOperator },
    { id: "admin", label: t.roleAdmin },
  ];
  const { user: sessionUser } = useSession();
  const [rows, setRows] = useState<AppUser[]>([...initialOperators, ...admins]);
  const [modalOpen, setModalOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<AppUser | null>(null);
  const [detail, setDetail] = useState<AppUser | null>(null);
  const [form, setForm] = useState({ name: "", email: "", role: "operator" as Role });

  const openCreate = () => {
    setForm({ name: "", email: "", role: "operator" });
    setModalOpen(true);
  };

  const handleSave = () => {
    const g = gradients[Math.floor(Math.random() * gradients.length)];
    setRows((prev) => [
      {
        id: `u-staff-new-${Date.now()}`,
        name: form.name,
        email: form.email,
        role: form.role,
        createdAt: new Date().toISOString().slice(0, 10),
        avatarColorFrom: g[0],
        avatarColorTo: g[1],
      },
      ...prev,
    ]);
    toast.success(t.accountCreated(form.role === "admin" ? t.roleAdmin : t.roleOperator));
    setModalOpen(false);
  };

  const columns: Column<AppUser>[] = [
    {
      key: "name",
      header: t.colName,
      render: (u) => (
        <div className="flex items-center gap-2.5">
          <GradientAvatar user={u} size="sm" />
          <div>
            <p className="font-medium text-foreground">{u.name}</p>
            <p className="text-xs text-muted">{u.email}</p>
          </div>
        </div>
      ),
    },
    {
      key: "role",
      header: t.colRole,
      render: (u) => (
        <Chip size="sm" className={u.role === "admin" ? "bg-accent-soft text-accent-soft-foreground font-medium" : "bg-default text-default-foreground font-medium"}>
          {u.role === "admin" ? t.roleAdmin : t.roleOperator}
        </Chip>
      ),
    },
    { key: "createdAt", header: t.colCreated, render: (u) => formatDate(u.createdAt) },
    {
      key: "actions",
      header: "",
      headerClassName: "text-right",
      className: "text-right",
      render: (u) => (
        <div className="flex justify-end gap-1" onClick={(e) => e.stopPropagation()}>
          <Button variant="ghost" size="sm" isIconOnly>
            <Pencil className="size-3.5" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            isIconOnly
            className="text-danger"
            isDisabled={u.id === sessionUser.id}
            onPress={() => setDeleteTarget(u)}
          >
            <Trash2 className="size-3.5" />
          </Button>
        </div>
      ),
    },
  ];

  return (
    <RequireRole roles={["admin"]} title={t.pageTitle} deniedMessage={t.deniedMessage}>
      <AppShell
        title={t.pageTitle}
        description={t.pageDescription}
        actions={
          <Button variant="primary" size="sm" onPress={openCreate}>
            <Plus className="size-3.5" /> {t.addAccount}
          </Button>
        }
      >
        <DataTable columns={columns} rows={rows} rowKey={(u) => u.id} onRowClick={setDetail} />

        <FormModal
          isOpen={modalOpen}
          onOpenChange={setModalOpen}
          title={t.addStaffTitle}
          footer={
            <>
              <Button variant="ghost" onPress={() => setModalOpen(false)}>
                {t.cancel}
              </Button>
              <Button variant="primary" onPress={handleSave} isDisabled={!form.name || !form.email}>
                {t.createAccount}
              </Button>
            </>
          }
        >
          <TextInput label={t.fieldFullName} required value={form.name} onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))} />
          <TextInput label={t.fieldEmail} type="email" required value={form.email} onChange={(e) => setForm((f) => ({ ...f, email: e.target.value }))} />
          <SelectInput
            label={t.fieldRole}
            options={roleOptions}
            selectedKey={form.role}
            onSelectionChange={(k) => setForm((f) => ({ ...f, role: (k as Role) ?? "operator" }))}
          />
          <div className="flex items-start gap-2.5 rounded-lg bg-accent-soft p-3 text-xs text-accent-soft-foreground">
            {form.role === "admin" ? <ShieldCheck className="mt-0.5 size-3.5 shrink-0" /> : <UserCog className="mt-0.5 size-3.5 shrink-0" />}
            {form.role === "admin" ? t.adminInfo : t.operatorInfo}
          </div>
        </FormModal>

        <FormModal
          isOpen={!!detail}
          onOpenChange={(open) => !open && setDetail(null)}
          title={detail?.name ?? ""}
          description={detail?.email}
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
                  <p className="text-xs text-muted">{t.colRole}</p>
                  <Chip
                    size="sm"
                    className={
                      detail.role === "admin"
                        ? "bg-accent-soft text-accent-soft-foreground font-medium"
                        : "bg-default text-default-foreground font-medium"
                    }
                  >
                    {detail.role === "admin" ? t.roleAdmin : t.roleOperator}
                  </Chip>
                </div>
                <div>
                  <p className="text-xs text-muted">{t.colCreated}</p>
                  <p className="font-medium text-foreground">{formatDate(detail.createdAt)}</p>
                </div>
              </div>
              <div className="flex items-start gap-2.5 rounded-lg bg-accent-soft p-3 text-xs text-accent-soft-foreground">
                {detail.role === "admin" ? <ShieldCheck className="mt-0.5 size-3.5 shrink-0" /> : <UserCog className="mt-0.5 size-3.5 shrink-0" />}
                {detail.role === "admin" ? t.adminInfo : t.operatorInfo}
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
                  setRows((prev) => prev.filter((u) => u.id !== deleteTarget?.id));
                  toast.success(t.accountDeleted);
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
    </RequireRole>
  );
}
