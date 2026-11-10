"use client";

import { useState } from "react";
import { Button } from "@heroui/react";
import { User as UserIcon } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { CardKicker, Panel } from "@/components/page-blocks";
import { AvatarSection, Feedback, PasswordPanel, failure, idle, type Outcome } from "@/components/profile-sections";
import { SelectInput, TextInput } from "@/components/ui/form";
import { api } from "@/lib/api/client";
import { useSession, useUser } from "@/lib/session-context";
import { useStudentProfileT } from "@/lib/i18n/dict/student-profile";

export default function StudentProfilePage() {
  const user = useUser();
  const { reloadUser } = useSession();
  const t = useStudentProfileT();
  const [name, setName] = useState(user.name);
  const [phone, setPhone] = useState(user.phone ?? "");
  const [gender, setGender] = useState<string | null>(user.gender);
  const [birthDate, setBirthDate] = useState(user.birthDate ?? "");
  const [profile, setProfile] = useState<Outcome>(idle);
  const [savingProfile, setSavingProfile] = useState(false);

  const handleSaveProfile = async (e: React.FormEvent) => {
    e.preventDefault();
    setProfile(idle);
    setSavingProfile(true);
    try {
      const updated = await api.updateProfile({ name, phone, gender: gender ?? "", birth_date: birthDate });
      setName(updated.name);
      setPhone(updated.phone ?? "");
      setGender(updated.gender);
      setBirthDate(updated.birth_date ?? "");
      setProfile({ ...idle, saved: true });
      await reloadUser().catch(() => {});
    } catch (err) {
      setProfile(failure(err, t.networkError));
    } finally {
      setSavingProfile(false);
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
          <AvatarSection />
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
              <SelectInput
                label={t.gender}
                description={t.personalHint}
                error={profile.fields.gender}
                options={[{ id: "male", label: t.genderMale }, { id: "female", label: t.genderFemale }]}
                selectedKey={gender}
                onSelectionChange={setGender}
              />
              <TextInput
                label={t.birthDate}
                type="date"
                max={new Date().toLocaleDateString("en-CA")}
                error={profile.fields.birth_date}
                value={birthDate}
                onChange={(e) => setBirthDate(e.target.value)}
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

        <PasswordPanel />
      </div>
    </AppShell>
  );
}
