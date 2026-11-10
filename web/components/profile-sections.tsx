"use client";

import { useEffect, useId, useRef, useState } from "react";
import { Button } from "@heroui/react";
import { CheckCircle2, KeyRound } from "lucide-react";
import { AvatarPicker } from "@/components/avatar-picker";
import { GradientAvatar } from "@/components/gradient-avatar";
import { Icon } from "@/components/icon";
import { CardKicker, Panel } from "@/components/page-blocks";
import { TextInput } from "@/components/ui/form";
import { ApiError, api, errorText } from "@/lib/api/client";
import { useSession, useUser } from "@/lib/session-context";
import { useStudentProfileT } from "@/lib/i18n/dict/student-profile";

export type Outcome = { saved: boolean; error: string; fields: Record<string, string> };
export const idle: Outcome = { saved: false, error: "", fields: {} };

// A field error is shown on its input; any other failure becomes the form-level message.
export function failure(err: unknown, fallback: string): Outcome {
  if (err instanceof ApiError && err.fields && Object.keys(err.fields).length > 0) {
    return { saved: false, error: "", fields: err.fields };
  }
  return { saved: false, error: errorText(err, fallback), fields: {} };
}

export function Feedback({ outcome, savedText }: { outcome: Outcome; savedText: string }) {
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

/** The signed-in user's avatar and name with the inline avatar chooser; every role. */
export function AvatarSection() {
  const user = useUser();
  const { reloadUser } = useSession();
  const t = useStudentProfileT();
  const [open, setOpen] = useState(false);
  const [picked, setPicked] = useState<string | null>(null);
  const [outcome, setOutcome] = useState<Outcome>(idle);
  const [saving, setSaving] = useState(false);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const chooserId = useId();

  const save = async () => {
    if (!picked) return;
    setOutcome(idle);
    setSaving(true);
    try {
      await api.setAvatar(picked);
      await reloadUser().catch(() => {});
      setOpen(false);
      triggerRef.current?.focus();
      setOutcome({ ...idle, saved: true });
    } catch (err) {
      setOutcome(failure(err, t.networkError));
    } finally {
      setSaving(false);
    }
  };

  return (
    <>
      <div className="flex flex-wrap items-center gap-4">
        <GradientAvatar user={user} size="xl" />
        <p className="min-w-0 flex-1 basis-40 font-display text-heading-sm">{user.name}</p>
        <Button
          ref={triggerRef}
          variant="outline"
          aria-expanded={open}
          aria-controls={open ? chooserId : undefined}
          onPress={() => {
            setPicked(user.avatar);
            setOutcome(idle);
            setOpen((current) => !current);
          }}
        >
          {t.changeAvatar}
        </Button>
      </div>
      {open && (
        <div id={chooserId} className="mt-5 flex flex-col gap-4 border-t border-border pt-5">
          <p className="text-caption text-muted">{t.avatarHint}</p>
          <AvatarPicker value={picked} onChange={setPicked} groupLabel={t.avatarGroup} optionLabel={t.avatarOption} />
          <div className="flex flex-wrap gap-2.5">
            <Button variant="primary" isDisabled={!picked || saving} onPress={save}>
              {saving ? t.saving : t.saveAvatar}
            </Button>
            <Button
              variant="ghost"
              onPress={() => {
                setOpen(false);
                triggerRef.current?.focus();
              }}
            >
              {t.cancel}
            </Button>
          </div>
        </div>
      )}
      {(outcome.saved || outcome.error) && (
        <div className="mt-4">
          <Feedback outcome={outcome} savedText={t.avatarSaved} />
        </div>
      )}
    </>
  );
}

/** Password card: one row until the user asks to change it, then the two fields. */
export function PasswordPanel() {
  const t = useStudentProfileT();
  const [open, setOpen] = useState(false);
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [outcome, setOutcome] = useState<Outcome>(idle);
  const [saving, setSaving] = useState(false);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const wasOpen = useRef(false);

  useEffect(() => {
    if (wasOpen.current && !open) triggerRef.current?.focus();
    wasOpen.current = open;
  }, [open]);

  const close = (result: Outcome) => {
    setOpen(false);
    setCurrent("");
    setNext("");
    setOutcome(result);
  };

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setOutcome(idle);
    setSaving(true);
    try {
      await api.changePassword({ current_password: current, new_password: next });
      close({ ...idle, saved: true });
    } catch (err) {
      setOutcome(failure(err, t.networkError));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Panel>
      <CardKicker icon={KeyRound} className="mb-5">
        {t.changePassword}
      </CardKicker>
      {!open ? (
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div className="min-w-0">
            <p className="text-small font-medium">{t.passwordTitle}</p>
            <p className="text-caption text-muted">{t.passwordHidden}</p>
            {outcome.saved && (
              <div className="mt-2">
                <Feedback outcome={outcome} savedText={t.passwordSaved} />
              </div>
            )}
          </div>
          <Button
            ref={triggerRef}
            variant="outline"
            onPress={() => {
              setOutcome(idle);
              setOpen(true);
            }}
          >
            {t.changePassword}
          </Button>
        </div>
      ) : (
        <form onSubmit={submit} className="flex flex-col gap-5">
          <div className="grid gap-5 sm:grid-cols-2">
            <TextInput
              label={t.currentPassword}
              type="password"
              autoComplete="current-password"
              autoFocus
              error={outcome.fields.current_password}
              value={current}
              onChange={(e) => setCurrent(e.target.value)}
            />
            <TextInput
              label={t.newPassword}
              type="password"
              autoComplete="new-password"
              description={t.newPasswordHint}
              error={outcome.fields.new_password}
              value={next}
              onChange={(e) => setNext(e.target.value)}
            />
          </div>
          <Feedback outcome={outcome} savedText={t.passwordSaved} />
          <div className="flex flex-wrap gap-2.5">
            <Button type="submit" variant="primary" isDisabled={!current || !next || saving}>
              {saving ? t.updatingPassword : t.updatePassword}
            </Button>
            <Button type="button" variant="ghost" onPress={() => close(idle)}>
              {t.cancel}
            </Button>
          </div>
        </form>
      )}
    </Panel>
  );
}
