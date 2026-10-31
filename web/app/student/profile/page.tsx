"use client";

import { useState } from "react";
import { Button, Card } from "@heroui/react";
import { CheckCircle2, KeyRound, User as UserIcon } from "lucide-react";
import { AppShell } from "@/components/app-shell";
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
      <p role="alert" className="text-sm text-danger">
        {outcome.error}
      </p>
    );
  }
  if (outcome.saved) {
    return (
      <p role="status" className="flex items-center gap-1.5 text-sm text-success">
        <CheckCircle2 className="size-4" />
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
            <TextInput
              label={t.fullName}
              required
              maxLength={100}
              error={profile.fields.name}
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
            <div className="flex flex-col gap-1.5">
              <p className="text-[13px] font-medium text-foreground">{t.email}</p>
              <p className="text-sm text-foreground">{user.email ?? "-"}</p>
              <p className="text-xs text-muted">{t.emailHint}</p>
            </div>
            <TextInput
              label={t.phone}
              type="tel"
              description={t.phoneHint}
              error={profile.fields.phone}
              value={phone}
              onChange={(e) => setPhone(e.target.value)}
            />
            <Feedback outcome={profile} savedText={t.profileSaved} />
            <Button type="submit" variant="primary" className="self-start" isDisabled={savingProfile}>
              {savingProfile ? t.saving : t.saveChanges}
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
        </Card>
      </div>
    </AppShell>
  );
}
