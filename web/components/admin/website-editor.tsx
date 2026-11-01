"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Button, toast } from "@heroui/react";
import { CloudOff, Eye, History, Save, ShieldAlert, TriangleAlert } from "lucide-react";
import { AppShell } from "../app-shell";
import { DataTable, type Column } from "../data-table";
import { EmptyState } from "../empty-state";
import { Icon } from "../icon";
import { LoadingNote } from "../admin-detail";
import { StatusChip } from "../status-badge";
import { FormModal } from "../ui/form-modal";
import { TextInput, TextareaInput } from "../ui/form";
import { programSlots, type ProgramSlot } from "../marketing/hero-exam-preview";
import {
  api,
  ApiError,
  type WebsiteContent,
  type WebsitePublication,
  type WebsiteWorkbench,
} from "@/lib/api/client";
import { cn } from "@/lib/cn";
import { useWebsiteT, type WebsiteDict } from "@/lib/i18n/dict/website";
import { useLocale, type Locale } from "@/lib/i18n/locale-context";

export const contentLocales = ["id", "en"] as const;
const homeKeys = ["hero_title", "hero_body", "program_cta", "practice_cta"] as const;
const programKeys = ["title", "summary", "schedule", "location", "fees", "contact"] as const;
export type FieldKey = (typeof homeKeys)[number] | (typeof programKeys)[number];
export type FieldScope = "home" | ProgramSlot;

export const fieldLimits: Record<FieldKey, number> = {
  hero_title: 90,
  hero_body: 360,
  program_cta: 36,
  practice_cta: 36,
  title: 80,
  summary: 320,
  schedule: 160,
  location: 160,
  fees: 160,
  contact: 160,
};
const requiredKeys: readonly FieldKey[] = ["hero_title", "hero_body", "program_cta", "practice_cta", "title", "summary"];

export interface FieldRef {
  locale: Locale;
  scope: FieldScope;
  key: FieldKey;
}

export const fieldPath = (f: FieldRef) =>
  f.scope === "home" ? `locales.${f.locale}.home.${f.key}` : `locales.${f.locale}.programs.${f.scope}.${f.key}`;

const allFields: FieldRef[] = contentLocales.flatMap((locale) => [
  ...homeKeys.map((key): FieldRef => ({ locale, scope: "home", key })),
  ...programSlots.flatMap((scope) => programKeys.map((key): FieldRef => ({ locale, scope, key }))),
]);

const bag = (c: WebsiteContent, f: FieldRef) =>
  (f.scope === "home" ? c.locales[f.locale].home : c.locales[f.locale].programs[f.scope]) as unknown as Record<string, string>;
export const readField = (c: WebsiteContent, f: FieldRef): string => bag(c, f)[f.key];

function writeField(c: WebsiteContent, f: FieldRef, value: string): WebsiteContent {
  const next = structuredClone(c);
  bag(next, f)[f.key] = value;
  return next;
}

const sameContent = (a: WebsiteContent, b: WebsiteContent) => allFields.every((f) => readField(a, f).trim() === readField(b, f).trim());

// Code points, like the server: an emoji counts once.
export const codePoints = (value: string) => Array.from(value.trim()).length;

function validate(content: WebsiteContent): Record<string, "website_required" | "website_too_long"> {
  const out: Record<string, "website_required" | "website_too_long"> = {};
  for (const f of allFields) {
    const n = codePoints(readField(content, f));
    if (n === 0 && requiredKeys.includes(f.key)) out[fieldPath(f)] = "website_required";
    else if (n > fieldLimits[f.key]) out[fieldPath(f)] = "website_too_long";
  }
  return out;
}

export interface DiffEntry extends FieldRef {
  before: string | null;
  after: string;
}

export function diffContent(published: WebsiteContent | null, draft: WebsiteContent): DiffEntry[] {
  return allFields
    .filter((f) => !published || readField(published, f) !== readField(draft, f))
    .map((f) => ({ ...f, before: published ? readField(published, f) : null, after: readField(draft, f) }));
}

type SaveState =
  | { status: "idle" | "saving" | "saved" | "invalid" | "conflict" | "checking" }
  | { status: "failed"; message?: string; requestId?: string };
type PublishState =
  | { status: "idle" | "publishing" | "conflict" | "checking" }
  | { status: "failed"; message?: string; requestId?: string }
  | { status: "success"; revision: number; changed: boolean | null };
type LoadState = { status: "loading" | "error" | "denied" | "ready" };
type HistoryState = { status: "loading" | "error" | "ready"; items: WebsitePublication[]; next: string | null; more: boolean; moreFailed: boolean };

export const previewHref = (slot?: ProgramSlot) => (slot ? `/admin/website/preview?program=${slot}` : "/admin/website/preview");

interface Editor {
  workbench: WebsiteWorkbench;
  local: WebsiteContent;
  dirty: boolean;
  clientErrors: Record<string, string>;
  fieldErrors: Record<string, string>;
  diff: DiffEntry[];
  save: SaveState;
  publishState: PublishState;
  history: HistoryState;
  contentLocale: Locale;
  setContentLocale: (l: Locale) => void;
  setField: (f: FieldRef, value: string) => void;
  doSave: () => Promise<boolean>;
  saveAndPreview: (slot?: ProgramSlot) => Promise<void>;
  askReload: () => void;
  doPublish: () => Promise<void>;
  loadMoreHistory: () => void;
  retryHistory: () => void;
}

interface Ctx {
  load: LoadState;
  reload: () => void;
  editor: Editor | null;
}

const EditorContext = createContext<Ctx | null>(null);

function useCtx() {
  const ctx = useContext(EditorContext);
  if (!ctx) throw new Error("WebsiteEditorProvider is missing");
  return ctx;
}

/** Only valid below a WebsiteGate, which renders children once the workbench has loaded. */
export function useWebsiteEditor(): Editor {
  const { editor } = useCtx();
  if (!editor) throw new Error("website workbench is not loaded");
  return editor;
}

const HISTORY_PAGE = 20;

export function WebsiteEditorProvider({ children }: { children: React.ReactNode }) {
  const t = useWebsiteT();
  const router = useRouter();
  const { locale: uiLocale } = useLocale();
  const [load, setLoad] = useState<LoadState>({ status: "loading" });
  const [workbench, setWorkbench] = useState<WebsiteWorkbench | null>(null);
  const [local, setLocal] = useState<WebsiteContent | null>(null);
  const [save, setSave] = useState<SaveState>({ status: "idle" });
  const [serverErrors, setServerErrors] = useState<Record<string, string>>({});
  const [publishState, setPublishState] = useState<PublishState>({ status: "idle" });
  const [history, setHistory] = useState<HistoryState>({ status: "loading", items: [], next: null, more: false, moreFailed: false });
  const [contentLocale, setContentLocale] = useState<Locale>(uiLocale);
  const [discardOpen, setDiscardOpen] = useState(false);
  const [leaveTo, setLeaveTo] = useState<string | null>(null);

  const wbRef = useRef<WebsiteWorkbench | null>(null);
  const busy = useRef(false);
  const saveUnknown = useRef(false);
  const publishUnknown = useRef(false);

  const commit = useCallback((next: WebsiteWorkbench, localWas: WebsiteContent | "discard") => {
    const prev = wbRef.current;
    wbRef.current = next;
    setWorkbench(next);
    setLocal((cur) => {
      if (localWas === "discard" || !cur || !prev) return next.draft;
      return sameContent(cur, localWas) ? next.draft : cur;
    });
  }, []);

  const loadHistoryPage = useCallback((cursor?: string) => {
    return api.website
      .history({ limit: HISTORY_PAGE, cursor })
      .then((page) =>
        setHistory((h) => ({ status: "ready", items: cursor ? [...h.items, ...page.items] : page.items, next: page.next_cursor, more: false, moreFailed: false })),
      )
      .catch(() => setHistory((h) => ({ ...h, status: cursor ? "ready" : "error", more: false, moreFailed: !!cursor })));
  }, []);

  const loadWorkbench = useCallback(() => {
    return api.website
      .workbench()
      .then((wb) => {
        commit(wb, "discard");
        setLoad({ status: "ready" });
      })
      .catch((err) => setLoad({ status: err instanceof ApiError && (err.status === 401 || err.status === 403) ? "denied" : "error" }));
  }, [commit]);

  useEffect(() => {
    void loadWorkbench();
    void loadHistoryPage();
  }, [loadWorkbench, loadHistoryPage]);

  const clientErrors = useMemo(() => (local ? validate(local) : {}), [local]);
  const dirty = !!local && !!workbench && !sameContent(local, workbench.draft);
  const diff = useMemo(() => (workbench ? diffContent(workbench.published, workbench.draft) : []), [workbench]);

  useEffect(() => {
    if (!dirty) return;
    const warn = (e: BeforeUnloadEvent) => e.preventDefault();
    const intercept = (e: MouseEvent) => {
      if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
      const a = (e.target as Element | null)?.closest?.("a[href]") as HTMLAnchorElement | null;
      if (!a || a.target === "_blank" || a.origin !== window.location.origin || a.pathname.startsWith("/admin/website")) return;
      e.preventDefault();
      e.stopPropagation();
      setLeaveTo(a.pathname + a.search + a.hash);
    };
    window.addEventListener("beforeunload", warn);
    document.addEventListener("click", intercept, true);
    return () => {
      window.removeEventListener("beforeunload", warn);
      document.removeEventListener("click", intercept, true);
    };
  }, [dirty]);

  const setField = useCallback((f: FieldRef, value: string) => {
    setLocal((cur) => (cur ? writeField(cur, f, value) : cur));
    setServerErrors((errs) => {
      const path = fieldPath(f);
      if (!(path in errs)) return errs;
      return Object.fromEntries(Object.entries(errs).filter(([p]) => p !== path));
    });
    setSave((s) => (s.status === "saved" || s.status === "invalid" || s.status === "failed" ? { status: "idle" } : s));
  }, []);

  // After a lost response: did the server apply `sent`? "retry" means it did not and a resend is safe.
  const verifySave = useCallback(
    async (sent: WebsiteContent, expected: number): Promise<"saved" | "conflict" | "retry" | "unreachable"> => {
      try {
        const fresh = await api.website.workbench();
        if (sameContent(fresh.draft, sent)) {
          commit(fresh, sent);
          return "saved";
        }
        return fresh.draft_revision === expected ? "retry" : "conflict";
      } catch {
        return "unreachable";
      }
    },
    [commit],
  );

  const doSave = useCallback(async (): Promise<boolean> => {
    const wb = wbRef.current;
    if (!wb || !local || busy.current || Object.keys(clientErrors).length > 0) return false;
    busy.current = true;
    const sent = local;
    const expected = wb.draft_revision;
    setServerErrors({});
    setSave({ status: saveUnknown.current ? "checking" : "saving" });
    try {
      if (saveUnknown.current) {
        const verdict = await verifySave(sent, expected);
        if (verdict === "saved") {
          saveUnknown.current = false;
          setSave({ status: "saved" });
          return true;
        }
        if (verdict === "conflict") {
          saveUnknown.current = false;
          setSave({ status: "conflict" });
          return false;
        }
        if (verdict === "unreachable") {
          setSave({ status: "failed" });
          return false;
        }
        saveUnknown.current = false;
        setSave({ status: "saving" });
      }
      const res = await api.website.save({ expected_draft_revision: expected, content: sent });
      commit(res, sent);
      setSave({ status: "saved" });
      toast.success(t.savedToast);
      return true;
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.code === "website_revision_conflict") setSave({ status: "conflict" });
        else if (err.fields && (err.code === "website_content_invalid" || err.code === "validation_failed")) {
          setServerErrors(err.fields);
          setSave({ status: "invalid" });
        } else setSave({ status: "failed", message: err.message, requestId: err.requestId });
        return false;
      }
      saveUnknown.current = true;
      setSave({ status: "checking" });
      const verdict = await verifySave(sent, expected);
      if (verdict === "saved") {
        saveUnknown.current = false;
        setSave({ status: "saved" });
        return true;
      }
      if (verdict === "conflict") {
        saveUnknown.current = false;
        setSave({ status: "conflict" });
      } else {
        if (verdict === "retry") saveUnknown.current = false;
        setSave({ status: "failed" });
      }
      return false;
    } finally {
      busy.current = false;
    }
  }, [local, clientErrors, commit, verifySave, t.savedToast]);

  const saveAndPreview = useCallback(
    async (slot?: ProgramSlot) => {
      if (dirty && !(await doSave())) return;
      router.push(previewHref(slot));
    },
    [dirty, doSave, router],
  );

  const reloadDiscarding = useCallback(async () => {
    setDiscardOpen(false);
    try {
      commit(await api.website.workbench(), "discard");
      saveUnknown.current = false;
      setServerErrors({});
      setSave({ status: "idle" });
    } catch {
      setSave({ status: "failed" });
    }
  }, [commit]);

  const refreshAfterPublish = useCallback(async (): Promise<WebsiteWorkbench | null> => {
    try {
      const [wb] = await Promise.all([api.website.workbench(), loadHistoryPage()]);
      commit(wb, wbRef.current?.draft ?? "discard");
      return wb;
    } catch {
      return null;
    }
  }, [commit, loadHistoryPage]);

  const doPublish = useCallback(async () => {
    const wb = wbRef.current;
    if (!wb || busy.current || (local && !sameContent(local, wb.draft))) return;
    busy.current = true;
    setPublishState({ status: publishUnknown.current ? "checking" : "publishing" });
    let current = wb;
    try {
      if (publishUnknown.current) {
        const fresh = await refreshAfterPublish();
        if (!fresh) {
          setPublishState({ status: "failed" });
          return;
        }
        publishUnknown.current = false;
        if (fresh.published_revision !== wb.published_revision) {
          setPublishState({ status: "success", revision: fresh.published_revision, changed: null });
          return;
        }
        current = fresh;
        setPublishState({ status: "publishing" });
      }
      const res = await api.website.publish({
        expected_draft_revision: current.draft_revision,
        expected_published_revision: current.published_revision,
      });
      await refreshAfterPublish();
      setPublishState({ status: "success", revision: res.published_revision, changed: res.changed });
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.code === "website_revision_conflict") {
          await refreshAfterPublish();
          setPublishState({ status: "conflict" });
        } else setPublishState({ status: "failed", message: err.message, requestId: err.requestId });
        return;
      }
      publishUnknown.current = true;
      setPublishState({ status: "checking" });
      const fresh = await refreshAfterPublish();
      if (fresh && fresh.published_revision !== current.published_revision) {
        publishUnknown.current = false;
        setPublishState({ status: "success", revision: fresh.published_revision, changed: null });
      } else {
        if (fresh) publishUnknown.current = false;
        setPublishState({ status: "failed" });
      }
    } finally {
      busy.current = false;
    }
  }, [local, refreshAfterPublish]);

  const loadMoreHistory = useCallback(() => {
    if (!history.next) return;
    setHistory((h) => ({ ...h, more: true, moreFailed: false }));
    void loadHistoryPage(history.next);
  }, [history.next, loadHistoryPage]);

  const retryHistory = useCallback(() => {
    setHistory({ status: "loading", items: [], next: null, more: false, moreFailed: false });
    void loadHistoryPage();
  }, [loadHistoryPage]);

  const reload = useCallback(() => {
    setLoad({ status: "loading" });
    void loadWorkbench();
  }, [loadWorkbench]);

  const fieldErrors = useMemo(() => ({ ...serverErrors, ...clientErrors }), [serverErrors, clientErrors]);

  const editor: Editor | null =
    workbench && local
      ? {
          workbench,
          local,
          dirty,
          clientErrors,
          fieldErrors,
          diff,
          save,
          publishState,
          history,
          contentLocale,
          setContentLocale,
          setField,
          doSave,
          saveAndPreview,
          askReload: () => setDiscardOpen(true),
          doPublish,
          loadMoreHistory,
          retryHistory,
        }
      : null;

  return (
    <EditorContext.Provider value={{ load, reload, editor }}>
      {children}
      <FormModal
        isOpen={discardOpen}
        onOpenChange={setDiscardOpen}
        size="sm"
        title={t.discardTitle}
        description={t.discardBody}
        footer={
          <>
            <Button variant="ghost" onPress={() => setDiscardOpen(false)}>
              {t.cancel}
            </Button>
            <Button variant="danger" onPress={reloadDiscarding}>
              {t.discardConfirm}
            </Button>
          </>
        }
      >
        {null}
      </FormModal>
      <FormModal
        isOpen={leaveTo !== null}
        onOpenChange={(open) => !open && setLeaveTo(null)}
        size="sm"
        title={t.leaveTitle}
        description={t.leaveBody}
        footer={
          <>
            <Button variant="ghost" onPress={() => setLeaveTo(null)}>
              {t.leaveStay}
            </Button>
            <Button
              variant="danger"
              onPress={() => {
                const to = leaveTo;
                setLeaveTo(null);
                if (to) router.push(to);
              }}
            >
              {t.leaveConfirm}
            </Button>
          </>
        }
      >
        {null}
      </FormModal>
    </EditorContext.Provider>
  );
}

/** Loading, failure and denied states shared by every route; children only render once loaded. */
export function WebsiteGate({ children }: { children: React.ReactNode }) {
  const t = useWebsiteT();
  const { load, reload } = useCtx();
  if (load.status === "loading") return <LoadingNote>{t.loading}</LoadingNote>;
  if (load.status === "denied") return <EmptyState icon={ShieldAlert} title={t.deniedTitle} description={t.deniedBody} />;
  if (load.status === "error")
    return (
      <EmptyState
        icon={CloudOff}
        title={t.loadErrorTitle}
        description={t.loadErrorBody}
        action={
          <Button variant="primary" size="sm" onPress={reload}>
            {t.retry}
          </Button>
        }
      />
    );
  return <>{children}</>;
}

type TabId = "overview" | "home" | "programs" | "review";
const tabHref: Record<TabId, string> = {
  overview: "/admin/website",
  home: "/admin/website/home",
  programs: "/admin/website/programs",
  review: "/admin/website/review",
};

function formatWhen(iso: string, locale: Locale) {
  return new Intl.DateTimeFormat(locale === "en" ? "en-GB" : "id-ID", { dateStyle: "medium", timeStyle: "short" }).format(new Date(iso));
}
export { formatWhen };

function StatusStrip() {
  const t = useWebsiteT();
  const { locale } = useLocale();
  const ed = useWebsiteEditor();
  const { workbench: wb } = ed;
  const chip =
    ed.save.status === "saving"
      ? { tone: "neutral" as const, text: t.statusSaving }
      : ed.dirty
        ? { tone: "warning" as const, text: t.statusUnsaved }
        : wb.published_revision === 0
          ? { tone: "accent" as const, text: t.statusNeverPublished }
          : ed.diff.length === 0
            ? { tone: "success" as const, text: t.statusInSync }
            : { tone: "accent" as const, text: t.statusDraftAhead };
  return (
    <div role="status" className="mb-6 flex flex-wrap items-center gap-x-4 gap-y-2 text-label text-muted">
      <StatusChip tone={chip.tone} dot>
        {chip.text}
      </StatusChip>
      <span>{wb.published_revision === 0 ? t.notPublished : t.publishedRev(wb.published_revision)}</span>
      <span>{t.draftRev(wb.draft_revision)}</span>
      <span>{t.lastSaved(formatWhen(wb.updated_at, locale))}</span>
    </div>
  );
}

export function WebsiteFrame({
  active,
  title,
  description,
  actions,
  breadcrumb,
  children,
}: {
  active: TabId;
  title: string;
  description?: string;
  actions?: React.ReactNode;
  breadcrumb?: { label: string; href?: string }[];
  children: React.ReactNode;
}) {
  const t = useWebsiteT();
  return (
    <AppShell title={title} description={description} actions={actions} breadcrumb={breadcrumb}>
      <WebsiteGate>
        <nav aria-label={t.tabsLabel} className="mb-5 flex gap-1 overflow-x-auto border-b border-border">
          {(Object.keys(tabHref) as TabId[]).map((id) => (
            <Link
              key={id}
              href={tabHref[id]}
              aria-current={active === id ? "page" : undefined}
              className={cn(
                "-mb-px flex min-h-11 items-center border-b-2 px-4 text-caption whitespace-nowrap outline-none focus-visible:ring-2 focus-visible:ring-focus",
                active === id ? "border-accent font-medium text-foreground" : "border-transparent text-muted hover:text-foreground",
              )}
            >
              {t.tabs[id]}
            </Link>
          ))}
        </nav>
        <StatusStrip />
        {children}
      </WebsiteGate>
    </AppShell>
  );
}

export function ContentLocaleButtons() {
  const t = useWebsiteT();
  const ed = useWebsiteEditor();
  return (
    <div role="group" aria-label={t.contentLanguage} className="flex gap-1.5">
      {contentLocales.map((l) => (
        <Button key={l} size="sm" variant={ed.contentLocale === l ? "primary" : "secondary"} aria-pressed={ed.contentLocale === l} onPress={() => ed.setContentLocale(l)}>
          {l.toUpperCase()}
        </Button>
      ))}
    </div>
  );
}

export function LocaleSwitch() {
  const t = useWebsiteT();
  const ed = useWebsiteEditor();
  return (
    <div role="group" aria-label={t.contentLanguage} className="flex flex-wrap items-center gap-2">
      <span className="text-label text-muted">{t.contentLanguage}</span>
      {contentLocales.map((l) => {
        const failing = allFields.some((f) => f.locale === l && fieldPath(f) in ed.fieldErrors);
        return (
          <Button key={l} size="sm" variant={ed.contentLocale === l ? "primary" : "secondary"} aria-pressed={ed.contentLocale === l} onPress={() => ed.setContentLocale(l)}>
            {t.localeNames[l]}
            {failing && <TriangleAlert className="size-3.5" aria-label={`${t.localeNames[l]}: ${t.localeHasErrors}`} />}
          </Button>
        );
      })}
    </div>
  );
}

function reasonText(t: WebsiteDict, reason: string, key: FieldKey) {
  if (reason === "website_required") return t.reasons.required;
  if (reason === "website_too_long") return t.reasons.tooLong(fieldLimits[key]);
  return t.reasons.unknown;
}

export function ContentField({ scope, fieldKey, multiline, rows, help }: { scope: FieldScope; fieldKey: FieldKey; multiline?: boolean; rows?: number; help?: string }) {
  const t = useWebsiteT();
  const ed = useWebsiteEditor();
  const ref: FieldRef = { locale: ed.contentLocale, scope, key: fieldKey };
  const value = readField(ed.local, ref);
  const reason = ed.fieldErrors[fieldPath(ref)];
  const count = codePoints(value);
  const max = fieldLimits[fieldKey];
  const common = {
    label: t.fields[fieldKey],
    description: help,
    error: reason ? reasonText(t, reason, fieldKey) : undefined,
    required: requiredKeys.includes(fieldKey),
    value,
    onChange: (e: { target: { value: string } }) => ed.setField(ref, e.target.value),
  };
  return (
    <div>
      {multiline ? <TextareaInput {...common} rows={rows ?? 3} /> : <TextInput {...common} />}
      <p className={cn("mt-1 text-right text-label tabular-nums", count > max ? "text-danger" : "text-muted")}>{t.counter(count, max)}</p>
    </div>
  );
}

function Banner({ tone, title, children }: { tone: "danger" | "warning" | "success"; title: string; children?: React.ReactNode }) {
  const toneClass = {
    danger: "border-danger bg-danger-soft text-danger-soft-foreground",
    warning: "border-warning bg-warning-soft text-warning-soft-foreground",
    success: "border-success bg-success-soft text-success-soft-foreground",
  }[tone];
  return (
    <div role={tone === "success" ? "status" : "alert"} className={cn("rounded-tile border p-4 text-caption", toneClass)}>
      <p className="font-medium">{title}</p>
      {children && <div className="mt-1 space-y-3">{children}</div>}
    </div>
  );
}
export { Banner };

export function SaveFeedback() {
  const t = useWebsiteT();
  const ed = useWebsiteEditor();
  const invalidCount = Object.keys(ed.clientErrors).length;
  const serverOnly = Object.keys(ed.fieldErrors).filter((p) => !(p in ed.clientErrors)).length;
  const s = ed.save;
  return (
    <div className="space-y-3" aria-live="polite">
      {invalidCount > 0 && (
        <Banner tone="warning" title={t.invalidSummary(invalidCount)} />
      )}
      {s.status === "invalid" && serverOnly > 0 && <Banner tone="danger" title={t.serverInvalidTitle}>{t.serverInvalidBody}</Banner>}
      {s.status === "failed" && (
        <Banner tone="danger" title={t.saveFailedTitle}>
          <p>{s.message ?? t.saveFailedBody}</p>
          {s.requestId && <p className="font-mono text-label">{s.requestId}</p>}
        </Banner>
      )}
      {s.status === "checking" && (
        <Banner tone="warning" title={t.checkingTitle}>
          {t.checkingBody}
        </Banner>
      )}
      {s.status === "conflict" && (
        <Banner tone="danger" title={t.conflictTitle}>
          <p>{t.conflictBody}</p>
          <Button size="sm" variant="secondary" onPress={ed.askReload}>
            {t.conflictReload}
          </Button>
        </Banner>
      )}
      {ed.dirty && s.status !== "conflict" && <p className="text-label text-muted">{t.unsavedNote}</p>}
    </div>
  );
}

export function SaveBar({ slot }: { slot?: ProgramSlot }) {
  const t = useWebsiteT();
  const ed = useWebsiteEditor();
  const busy = ed.save.status === "saving" || ed.save.status === "checking";
  const blocked = busy || Object.keys(ed.clientErrors).length > 0 || ed.save.status === "conflict";
  return (
    <div className="flex flex-wrap items-center gap-2.5">
      <Button variant="secondary" size="sm" isDisabled={blocked || !ed.dirty} onPress={() => void ed.doSave()}>
        <Icon icon={Save} size="sm" /> {busy ? t.saving : t.save}
      </Button>
      <Link
        href={previewHref(slot)}
        className="inline-flex min-h-9 items-center gap-2 rounded-control border border-border bg-surface px-3 text-caption font-medium outline-none hover:bg-surface-secondary focus-visible:ring-2 focus-visible:ring-focus"
      >
        <Icon icon={Eye} size="sm" /> {t.preview}
      </Link>
      <Button variant="primary" size="sm" isDisabled={blocked} onPress={() => void ed.saveAndPreview(slot)}>
        <Icon icon={Eye} size="sm" /> {t.saveAndPreview}
      </Button>
    </div>
  );
}

export function HistoryPanel() {
  const t = useWebsiteT();
  const { locale } = useLocale();
  const { history, loadMoreHistory, retryHistory } = useWebsiteEditor();
  const columns: Column<WebsitePublication>[] = [
    { key: "rev", header: t.historyColumns.revision, render: (r) => <span className="font-medium tabular-nums">r{r.published_revision}</span> },
    { key: "src", header: t.historyColumns.source, render: (r) => <span className="tabular-nums text-muted">r{r.source_draft_revision}</span> },
    { key: "at", header: t.historyColumns.at, render: (r) => formatWhen(r.published_at, locale) },
    { key: "by", header: t.historyColumns.by, render: (r) => <span className="font-mono text-label text-muted">{r.published_by.slice(0, 8)}</span> },
  ];
  return (
    <section aria-labelledby="website-history" className="space-y-3">
      <div>
        <h2 id="website-history" className="font-display text-title text-foreground">{t.historyTitle}</h2>
        <p className="mt-1 text-label text-muted">{t.historyBody}</p>
      </div>
      {history.status === "loading" ? (
        <LoadingNote>{t.historyLoading}</LoadingNote>
      ) : history.status === "error" ? (
        <EmptyState icon={CloudOff} title={t.historyError} action={<Button variant="secondary" size="sm" onPress={retryHistory}>{t.retry}</Button>} />
      ) : history.items.length === 0 ? (
        <EmptyState icon={History} title={t.historyEmpty} />
      ) : (
        <>
          <DataTable columns={columns} rows={history.items} rowKey={(r) => String(r.published_revision)} />
          {history.moreFailed && (
            <p role="alert" className="text-label text-danger">
              {t.historyMoreError}
            </p>
          )}
          {history.next && (
            <Button variant="secondary" size="sm" isDisabled={history.more} onPress={loadMoreHistory}>
              {history.moreFailed ? t.retry : t.historyMore}
            </Button>
          )}
        </>
      )}
    </section>
  );
}
