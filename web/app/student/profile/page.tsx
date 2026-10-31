"use client";

import { useState } from "react";
import { Button } from "@heroui/react";
import { CheckCircle2, KeyRound, User as UserIcon } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { Icon } from "@/components/icon";
import { CardKicker, Panel } from "@/components/page-blocks";
import { GradientAvatar } from "@/components/gradient-avatar";
import { TextInput } from "@/components/ui/form";
import { ApiError, api, errorText } from "@/lib/api/client";
import { useSession, useUser } from "@/lib/session-context";
import { useStudentProfileT } from "@/lib/i18n/dict/student-profile";

type Outcome = { saved: boolean; error: string; fields: Record<string, string> };
const idle: Outcome = { saved: false, error: "", fields: {} };

// A field error is shown on its input; any other failure becomes the form-level message.
function failure(err: unknown, fallback: string): Outcome {
  if (err instanceof ApiError && err.fields && Object.keys(err.fields).length > 0) {
    return { saved: false, error: "", fields: err.fields };
  }
  return { saved: false, error: errorText(err, fallback), fields: {} };
}

function Feedback({ outcome, savedText }: { outcome: Outcome; savedText: string }) {
  if (outcome.error) {
    return (
      <p role="alert" className="text-small text-danger">
        {outcome.error}
      </p>
    );
  }
  if (outcome.saved) {
    return (
      <p role="status" className="flex items-center gap-1.5 text-small text-success">
        <Icon icon={CheckCircle2} size="sm" />
        {savedText}
      </p>
    );
  }
  return null;
}

export default function StudentProfilePage() {
  const user = useUser();
  const { reloadUser } = useSession();
  const t = useStudentProfileT();
  const [name, setName] = useState(user.name);
  const [phone, setPhone] = useState(user.phone ?? "");
  const [profile, setProfile] = useState<Outcome>(idle);
  const [savingProfile, setSavingProfile] = useState(false);
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [password, setPassword] = useState<Outcome>(idle);
  const [savingPassword, setSavingPassword] = useState(false);

  const handleSaveProfile = async (e: React.FormEvent) => {
    e.preventDefault();
    setProfile(idle);
    setSavingProfile(true);
    try {
      const updated = await api.updateProfile({ name, phone });
      setName(updated.name);
      setPhone(updated.phone ?? "");
      setProfile({ ...idle, saved: true });
      await reloadUser().catch(() => {});
    } catch (err) {
      setProfile(failure(err, t.networkError));
    } finally {
      setSavingProfile(false);
    }
  };

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault();
    setPassword(idle);
    setSavingPassword(true);
    try {
      await api.changePassword({ current_password: currentPassword, new_password: newPassword });
      setCurrentPassword("");
      setNewPassword("");
      setPassword({ ...idle, saved: true });
    } catch (err) {
      setPassword(failure(err, t.networkError));
    } finally {
      setSavingPassword(false);
    }
  };

  return (
    <AppShell
      title={t.title}
      description={t.description}
      maxWidth="max-w-3xl"
      breadcrumb={[{ label: t.crumbDashboard, href: "/student" }, { label: t.title }]}
    >
      <div className="space-y-6">
        <Panel>
          <CardKicker icon={UserIcon} className="mb-5">
            {t.accountInfo}
          </CardKicker>
          <div className="flex items-center gap-3">
            <GradientAvatar user={user} size="lg" />
            <p className="font-display text-heading-sm">{user.name}</p>
          </div>
          <form onSubmit={handleSaveProfile} className="mt-5 flex flex-col gap-5 border-t border-border pt-5">
            <div className="grid gap-5 sm:grid-cols-2">
              <TextInput
                label={t.fullName}
                required
                maxLength={100}
                error={profile.fields.name}
                value={name}
                onChange={(e) => setName(e.target.value)}
              />
              <TextInput
                label={t.phone}
                type="tel"
                description={t.phoneHint}
                error={profile.fields.phone}
                value={phone}
                onChange={(e) => setPhone(e.target.value)}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <p className="text-caption font-medium text-foreground">{t.email}</p>
              <p className="text-small text-foreground">{user.email ?? "-"}</p>
              <p className="text-label text-muted">{t.emailHint}</p>
            </div>
            <Feedback outcome={profile} savedText={t.profileSaved} />
            <Button type="submit" variant="primary" className="self-start" isDisabled={savingProfile}>
              {savingProfile ? t.saving : t.saveChanges}
            </Button>
          </form>
        </Panel>

        <Panel>
          <CardKicker icon={KeyRound} className="mb-5">
            {t.changePassword}
          </CardKicker>
          <form onSubmit={handleChangePassword} className="flex flex-col gap-5">
            <div className="grid gap-5 sm:grid-cols-2">
              <TextInput
                label={t.currentPassword}
                type="password"
                autoComplete="current-password"
                error={password.fields.current_password}
                value={currentPassword}
                onChange={(e) => setCurrentPassword(e.target.value)}
              />
              <TextInput
                label={t.newPassword}
                type="password"
                autoComplete="new-password"
                description={t.newPasswordHint}
                error={password.fields.new_password}
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
              />
            </div>
            <Feedback outcome={password} savedText={t.passwordSaved} />
            <Button
              type="submit"
              variant="secondary"
              className="self-start"
              isDisabled={!currentPassword || !newPassword || savingPassword}
            >
              {savingPassword ? t.updatingPassword : t.updatePassword}
            </Button>
          </form>
        </Panel>
      </div>
    </AppShell>
  );
}
