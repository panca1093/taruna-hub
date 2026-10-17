"use client";

import { useState } from "react";
import { Button, Card, toast } from "@heroui/react";
import { KeyRound, User as UserIcon } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { GradientAvatar } from "@/components/gradient-avatar";
import { SelectInput, TextInput } from "@/components/ui/form";
import { useUser } from "@/lib/session-context";
import { programs } from "@/lib/mock-data";
import type { ProgramCode } from "@/lib/types";
import { useStudentProfileT } from "@/lib/i18n/dict/student-profile";

const programOptions = programs.map((p) => ({ id: p.code, label: `${p.name} (${p.code})` }));

export default function StudentProfilePage() {
  const user = useUser();
  const t = useStudentProfileT();
  const [name, setName] = useState(user.name);
  const [email, setEmail] = useState(user.email ?? "");
  const [program, setProgram] = useState<ProgramCode | null>(null);
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");

  const handleSaveProfile = (e: React.FormEvent) => {
    e.preventDefault();
    toast.success(t.toastProfileUpdated);
  };

  const handleChangePassword = (e: React.FormEvent) => {
    e.preventDefault();
    if (!currentPassword || !newPassword) return;
    toast.success(t.toastPasswordUpdated);
    setCurrentPassword("");
    setNewPassword("");
  };

  return (
    <AppShell title={t.title} description={t.description}>
      <div className="mx-auto max-w-2xl space-y-6">
        <Card className="gap-4 p-5">
          <div className="flex items-center gap-2">
            <UserIcon className="size-4 text-accent" />
            <p className="text-sm font-semibold text-foreground">{t.accountInfo}</p>
          </div>
          <div className="flex items-center gap-3">
            <GradientAvatar user={user} size="lg" />
            <p className="text-sm font-semibold text-foreground">{user.name}</p>
          </div>
          <form onSubmit={handleSaveProfile} className="flex flex-col gap-4 border-t border-border pt-4">
            <TextInput label={t.fullName} required value={name} onChange={(e) => setName(e.target.value)} />
            <TextInput label={t.email} type="email" required value={email} onChange={(e) => setEmail(e.target.value)} />
            <SelectInput
              label={t.programLabel}
              options={programOptions}
              selectedKey={program}
              onSelectionChange={(k) => setProgram(k as ProgramCode | null)}
            />
            <Button type="submit" variant="primary" className="self-start">
              {t.saveChanges}
            </Button>
          </form>
        </Card>

        <Card className="gap-4 p-5">
          <div className="flex items-center gap-2">
            <KeyRound className="size-4 text-accent" />
            <p className="text-sm font-semibold text-foreground">{t.changePassword}</p>
          </div>
          <form onSubmit={handleChangePassword} className="flex flex-col gap-4">
            <TextInput
              label={t.currentPassword}
              type="password"
              value={currentPassword}
              onChange={(e) => setCurrentPassword(e.target.value)}
            />
            <TextInput
              label={t.newPassword}
              type="password"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
            />
            <Button type="submit" variant="secondary" className="self-start" isDisabled={!currentPassword || !newPassword}>
              {t.updatePassword}
            </Button>
          </form>
        </Card>
      </div>
    </AppShell>
  );
}
