"use client";

import { useState } from "react";
import { Button, toast } from "@heroui/react";
import { ShieldCheck, UserCog } from "lucide-react";
import { NoteBox } from "@/components/page-blocks";
import { FormModal } from "@/components/ui/form-modal";
import { SelectInput, TextInput, TextareaInput } from "@/components/ui/form";
import { api, ApiError, errorText, type AdminUser } from "@/lib/api/client";
import { useUsersT } from "@/lib/i18n/dict/users";

type AccountRole = AdminUser["role"];

/** Create when `target` is null, edit otherwise. Role is chosen only on create; editing never changes it. */
export function AccountFormModal(props: {
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  target: AdminUser | null;
  roles: AccountRole[];
  onSaved: (user: AdminUser, created: boolean) => void;
}) {
  // Mounted only while open, so every opening starts from the target's own values.
  return props.isOpen ? <AccountForm key={props.target?.id ?? "new"} {...props} /> : null;
}

function AccountForm({
  onOpenChange,
  target,
  roles,
  onSaved,
}: {
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  target: AdminUser | null;
  roles: AccountRole[];
  onSaved: (user: AdminUser, created: boolean) => void;
}) {
  const t = useUsersT();
  const roleLabel = (role: AccountRole) =>
    role === "admin" ? t.roleAdmin : role === "operator" ? t.roleOperator : t.roleStudent;
  const [form, setForm] = useState({
    name: target?.name ?? "",
    email: target?.email ?? "",
    phone: target?.phone ?? "",
    password: "",
    role: roles[0],
  });
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);

  const submit = async () => {
    setSaving(true);
    setErrors({});
    try {
      const user = target
        ? await api.adminUsers.update(target.id, { name: form.name, email: form.email, phone: form.phone })
        : await api.adminUsers.create({ ...form });
      toast.success(target ? t.accountUpdated : t.accountCreated(roleLabel(user.role)));
      onSaved(user, target === null);
      onOpenChange(false);
    } catch (err) {
      if (err instanceof ApiError && err.fields) setErrors(err.fields);
      toast.danger(t.accountSaveFailed, { description: errorText(err, t.loadErrorDescription) });
    } finally {
      setSaving(false);
    }
  };

  const title = target ? t.editTitle : roles.length === 1 && roles[0] === "student" ? t.addStudentTitle : t.addStaffTitle;
  const incomplete = form.name.trim() === "" || form.email.trim() === "" || (target === null && form.password === "");

  return (
    <FormModal
      isOpen
      onOpenChange={onOpenChange}
      title={title}
      footer={
        <>
          <Button variant="ghost" onPress={() => onOpenChange(false)}>
            {t.cancel}
          </Button>
          <Button variant="primary" onPress={submit} isDisabled={saving || incomplete}>
            {saving ? t.saving : target ? t.save : t.createAccount}
          </Button>
        </>
      }
    >
      <TextInput
        label={t.fieldFullName}
        required
        value={form.name}
        onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
        error={errors.name}
      />
      <TextInput
        label={t.fieldEmail}
        type="email"
        required
        value={form.email}
        onChange={(e) => setForm((f) => ({ ...f, email: e.target.value }))}
        error={errors.email}
        description={target ? t.emailChangeNote : undefined}
      />
      <TextInput
        label={t.fieldPhone}
        inputMode="tel"
        value={form.phone}
        onChange={(e) => setForm((f) => ({ ...f, phone: e.target.value }))}
        error={errors.phone}
        description={t.fieldPhoneHint}
      />
      {target === null && (
        <>
          <TextInput
            label={t.fieldPassword}
            type="password"
            required
            autoComplete="new-password"
            value={form.password}
            onChange={(e) => setForm((f) => ({ ...f, password: e.target.value }))}
            error={errors.password}
            description={t.fieldPasswordHint}
          />
          {roles.length > 1 && (
            <SelectInput
              label={t.fieldRole}
              options={roles.map((role) => ({ id: role, label: roleLabel(role) }))}
              selectedKey={form.role}
              onSelectionChange={(key) => setForm((f) => ({ ...f, role: (key as AccountRole) ?? roles[0] }))}
              error={errors.role}
            />
          )}
          {form.role !== "student" && (
            <NoteBox icon={form.role === "admin" ? ShieldCheck : UserCog}>
              {form.role === "admin" ? t.adminInfo : t.operatorInfo}
            </NoteBox>
          )}
        </>
      )}
    </FormModal>
  );
}

/** Deactivate (D14: never delete) or reactivate; both need a reason, which is audited with the actor. */
export function AccountStatusModal({
  target,
  onClose,
  onDone,
}: {
  target: AdminUser | null;
  onClose: () => void;
  onDone: (user: AdminUser) => void;
}) {
  return target ? <AccountStatusForm key={target.id} target={target} onClose={onClose} onDone={onDone} /> : null;
}

function AccountStatusForm({
  target,
  onClose,
  onDone,
}: {
  target: AdminUser;
  onClose: () => void;
  onDone: (user: AdminUser) => void;
}) {
  const t = useUsersT();
  const [reason, setReason] = useState("");
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  const deactivating = target.is_active;
  const submit = async () => {
    setSaving(true);
    setError("");
    try {
      const user = deactivating
        ? await api.adminUsers.deactivate(target.id, reason.trim())
        : await api.adminUsers.reactivate(target.id, reason.trim());
      toast.success(deactivating ? t.accountDeactivated : t.accountReactivated);
      onDone(user);
      onClose();
    } catch (err) {
      setError(err instanceof ApiError ? (err.fields?.reason ?? err.message) : t.loadErrorDescription);
      toast.danger(t.actionFailed, { description: errorText(err, t.loadErrorDescription) });
    } finally {
      setSaving(false);
    }
  };

  return (
    <FormModal
      isOpen
      onOpenChange={(open) => !open && onClose()}
      title={deactivating ? t.deactivateTitle : t.reactivateTitle}
      description={target.name}
      footer={
        <>
          <Button variant="ghost" onPress={onClose}>
            {t.cancel}
          </Button>
          <Button variant={deactivating ? "danger" : "primary"} onPress={submit} isDisabled={saving || reason.trim() === ""}>
            {saving ? t.saving : deactivating ? t.deactivateConfirm : t.reactivateConfirm}
          </Button>
        </>
      }
    >
      <p className="text-small text-muted">{deactivating ? t.deactivateDescription : t.reactivateDescription}</p>
      <TextareaInput
        label={t.fieldReason}
        required
        rows={3}
        placeholder={t.fieldReasonPlaceholder}
        value={reason}
        onChange={(e) => setReason(e.target.value)}
        error={error}
      />
    </FormModal>
  );
}
