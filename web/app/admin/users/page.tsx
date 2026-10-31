"use client";

import { useState } from "react";
import { Button, toast } from "@heroui/react";
import { Pencil, Plus, ShieldCheck, Trash2, UserCog } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { RequireRole } from "@/components/require-role";
import { DataTable, type Column } from "@/components/data-table";
import { DetailGrid, DetailItem } from "@/components/admin-detail";
import { Icon } from "@/components/icon";
import { NoteBox } from "@/components/page-blocks";
import { StatusChip } from "@/components/status-badge";
import { GradientAvatar } from "@/components/gradient-avatar";
import { FormModal } from "@/components/ui/form-modal";
import { SelectInput, TextInput } from "@/components/ui/form";
import { admins, formatDate, operators as initialOperators } from "@/lib/mock-data";
import { useUser } from "@/lib/session-context";
import { useUsersT } from "@/lib/i18n/dict/users";
import type { AppUser, Role } from "@/lib/types";

export default function UsersPage() {
  const t = useUsersT();
  const roleOptions = [
    { id: "operator", label: t.roleOperator },
    { id: "admin", label: t.roleAdmin },
  ];
  const sessionUser = useUser();
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
    setRows((prev) => [
      {
        id: `u-staff-new-${Date.now()}`,
        name: form.name,
        email: form.email,
        role: form.role,
        createdAt: new Date().toISOString().slice(0, 10),
        avatarColorFrom: "var(--accent)",
        avatarColorTo: "var(--accent-2)",
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
            <p className="text-label text-muted">{u.email}</p>
          </div>
        </div>
      ),
    },
    {
      key: "role",
      header: t.colRole,
      render: (u) => (
        <StatusChip tone={u.role === "admin" ? "accent" : "neutral"}>{u.role === "admin" ? t.roleAdmin : t.roleOperator}</StatusChip>
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
            <Icon icon={Pencil} size="sm" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            isIconOnly
            className="text-danger"
            isDisabled={u.id === sessionUser.id}
            onPress={() => setDeleteTarget(u)}
          >
            <Icon icon={Trash2} size="sm" />
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
            <Icon icon={Plus} size="sm" /> {t.addAccount}
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
          <NoteBox icon={form.role === "admin" ? ShieldCheck : UserCog}>{form.role === "admin" ? t.adminInfo : t.operatorInfo}</NoteBox>
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
              <div className="flex items-center gap-3.5">
                <GradientAvatar user={detail} size="lg" />
                <div>
                  <p className="font-display text-title text-foreground">{detail.name}</p>
                  <p className="text-label text-muted">{detail.email}</p>
                </div>
              </div>
              <DetailGrid className="border-t border-border pt-5">
                <DetailItem label={t.colRole}>
                  <StatusChip tone={detail.role === "admin" ? "accent" : "neutral"}>{detail.role === "admin" ? t.roleAdmin : t.roleOperator}</StatusChip>
                </DetailItem>
                <DetailItem label={t.colCreated}>{formatDate(detail.createdAt)}</DetailItem>
              </DetailGrid>
              <NoteBox icon={detail.role === "admin" ? ShieldCheck : UserCog}>{detail.role === "admin" ? t.adminInfo : t.operatorInfo}</NoteBox>
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
          <p className="text-small text-muted">{t.deleteDescription}</p>
        </FormModal>
      </AppShell>
    </RequireRole>
  );
}
