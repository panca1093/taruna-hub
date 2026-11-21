"use client";

import { useMemo, useRef, useState, type ReactNode } from "react";
import { Button, Input, Popover } from "@heroui/react";
import { useEditorState, type Editor } from "@tiptap/react";
import katex from "katex";
import {
  Image as ImageIcon,
  List,
  ListOrdered,
  Redo2,
  Shapes,
  Sigma,
  Table as TableIcon,
  Undo2,
  type LucideIcon,
} from "lucide-react";
import { SolidShape } from "@/components/solid-shape";
import { api, errorText } from "@/lib/api/client";
import { cn } from "@/lib/cn";
import { SHAPE_KINDS, type ShapeKind } from "@/lib/rich-doc";
import { useAdminRichTextEditorT } from "@/lib/i18n/dict/admin-rich-text-editor";
import { useRichTextGroup, type MathEdit, type PopoverKind } from "./group";

const MAX_IMAGE_BYTES = 5 * 1024 * 1024;
const IMAGE_TYPES = ["image/png", "image/jpeg", "image/webp"];
const TABLE_ROWS = 5;
const TABLE_COLS = 6;

const toolClass = (pressed = false, danger = false) =>
  cn(
    "inline-flex h-8.5 min-w-8.5 items-center justify-center gap-1.5 rounded-chip px-2 text-label font-medium whitespace-nowrap transition-colors outline-none",
    "focus-visible:outline-2 focus-visible:outline-accent disabled:pointer-events-none disabled:opacity-35",
    pressed ? "bg-accent-soft text-accent" : "text-foreground hover:bg-surface-tertiary",
    danger && "hover:text-danger",
  );

const keepSelection = (e: React.MouseEvent) => e.preventDefault();

function ToolButton({
  label,
  pressed,
  disabled,
  danger,
  onPress,
  children,
}: {
  label: string;
  pressed?: boolean;
  disabled?: boolean;
  danger?: boolean;
  onPress: () => void;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      aria-label={label}
      aria-pressed={pressed}
      disabled={disabled}
      className={toolClass(pressed, danger)}
      onMouseDown={keepSelection}
      onClick={onPress}
    >
      {children}
    </button>
  );
}

const Divider = () => <span aria-hidden="true" className="mx-1 h-4.5 w-px bg-border" />;

function PopoverButton({
  kind,
  icon: Icon,
  label,
  disabled,
  title,
  children,
}: {
  kind: PopoverKind;
  icon: LucideIcon;
  label: string;
  disabled: boolean;
  title: string;
  children: (editor: Editor, close: () => void) => ReactNode;
}) {
  const group = useRichTextGroup();
  const open = group.popover?.kind === kind;
  return (
    <Popover isOpen={open && !!group.active} onOpenChange={(next) => (next && group.active ? group.openPopover(kind) : group.closePopover())}>
      <Popover.Trigger<"button">
        render={(props) => (
          <button {...props} type="button" disabled={disabled} aria-expanded={open} aria-haspopup="dialog" className={toolClass(open)} />
        )}
      >
        <Icon className="size-4" />
        {label}
      </Popover.Trigger>
      <Popover.Content placement="bottom start" className="max-w-popover">
        <Popover.Dialog className="flex w-80 max-w-full flex-col gap-3 p-3.5 outline-none" aria-label={title}>
          {group.active && children(group.active, group.closePopover)}
        </Popover.Dialog>
      </Popover.Content>
    </Popover>
  );
}

function PopoverHeading({ children }: { children: ReactNode }) {
  return <p className="text-caption font-semibold text-foreground">{children}</p>;
}

const MATH_CHIPS = (t: ReturnType<typeof useAdminRichTextEditorT>) => [
  { label: t.mathChips.fraction, latex: "\\frac{a}{b}" },
  { label: t.mathChips.power, latex: "x^{2}" },
  { label: t.mathChips.root, latex: "\\sqrt{x}" },
  { label: "π", latex: "\\pi" },
  { label: "×", latex: "\\times" },
  { label: "÷", latex: "\\div" },
  { label: "≤", latex: "\\le" },
  { label: "≥", latex: "\\ge" },
  { label: t.mathChips.degree, latex: "^{\\circ}" },
];

function MathBody({ editor, edit, onDone }: { editor: Editor; edit?: MathEdit; onDone: () => void }) {
  const t = useAdminRichTextEditorT();
  const [latex, setLatex] = useState(edit?.latex ?? "");
  const input = useRef<HTMLInputElement>(null);
  const trimmed = latex.trim();
  const preview = useMemo(() => (trimmed ? katex.renderToString(trimmed, { throwOnError: false }) : ""), [trimmed]);

  const addChip = (value: string) => {
    const el = input.current;
    const start = el?.selectionStart ?? latex.length;
    const end = el?.selectionEnd ?? start;
    setLatex(latex.slice(0, start) + value + latex.slice(end));
    requestAnimationFrame(() => {
      el?.focus();
      el?.setSelectionRange(start + value.length, start + value.length);
    });
  };

  const submit = () => {
    if (!trimmed) return;
    if (edit) {
      editor.chain().focus().setNodeSelection(edit.pos).updateAttributes("math", { latex: trimmed }).run();
    } else {
      editor
        .chain()
        .focus()
        .insertContent([{ type: "math", attrs: { latex: trimmed } }, { type: "text", text: " " }])
        .run();
    }
    onDone();
  };

  return (
    <>
      <PopoverHeading>{edit ? t.mathEditTitle : t.mathInsertTitle}</PopoverHeading>
      <div className="flex flex-wrap gap-1.5">
        {MATH_CHIPS(t).map((chip) => (
          <button
            key={chip.latex}
            type="button"
            onClick={() => addChip(chip.latex)}
            className="h-8 rounded-chip border border-border bg-surface px-2.5 text-label text-foreground transition-colors hover:border-accent hover:text-accent"
          >
            {chip.label}
          </button>
        ))}
      </div>
      <label className="flex flex-col gap-1 text-label font-medium text-foreground">
        {t.mathInputLabel}
        <Input
          ref={input}
          autoFocus
          fullWidth
          autoComplete="off"
          value={latex}
          placeholder={t.mathInputPlaceholder}
          onChange={(e) => setLatex(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault();
              submit();
            }
          }}
        />
      </label>
      <div
        aria-live="polite"
        className={cn(
          "grid min-h-13 place-items-center overflow-x-auto rounded-control bg-surface-secondary p-2",
          preview ? "text-small" : "text-label text-muted",
        )}
      >
        {preview ? <span dangerouslySetInnerHTML={{ __html: preview }} /> : t.mathPreviewEmpty}
      </div>
      <Button variant="primary" size="sm" isDisabled={!trimmed} onPress={submit}>
        {edit ? t.mathUpdate : t.mathInsert}
      </Button>
    </>
  );
}

function TableBody({ editor, onDone }: { editor: Editor; onDone: () => void }) {
  const t = useAdminRichTextEditorT();
  const [size, setSize] = useState<{ rows: number; cols: number } | null>(null);
  const cells = Array.from({ length: TABLE_ROWS * TABLE_COLS }, (_, k) => ({ rows: Math.floor(k / TABLE_COLS) + 1, cols: (k % TABLE_COLS) + 1 }));
  return (
    <>
      <PopoverHeading>{t.tableTitle}</PopoverHeading>
      <div className="grid grid-cols-6 gap-1">
        {cells.map((cell) => (
          <button
            key={`${cell.rows}-${cell.cols}`}
            type="button"
            aria-label={t.tableSize(cell.rows, cell.cols)}
            onMouseEnter={() => setSize(cell)}
            onFocus={() => setSize(cell)}
            onClick={() => {
              editor.chain().focus().insertTable({ rows: cell.rows, cols: cell.cols, withHeaderRow: true }).run();
              onDone();
            }}
            className={cn(
              "size-7 rounded-chip border transition-colors",
              size && cell.rows <= size.rows && cell.cols <= size.cols ? "border-accent bg-accent-soft" : "border-border bg-surface",
            )}
          />
        ))}
      </div>
      <p className="text-label text-muted" aria-live="polite">
        {size ? t.tableSize(size.rows, size.cols) : t.tablePickSize}
      </p>
    </>
  );
}

function ImageBody({ editor, onDone }: { editor: Editor; onDone: () => void }) {
  const t = useAdminRichTextEditorT();
  const [state, setState] = useState<{ status: "idle" | "uploading" } | { status: "error"; message: string }>({ status: "idle" });

  const pick = async (file: File | undefined) => {
    if (!file) return;
    if (!IMAGE_TYPES.includes(file.type)) return setState({ status: "error", message: t.imageWrongType });
    if (file.size > MAX_IMAGE_BYTES) return setState({ status: "error", message: t.imageTooLarge });
    setState({ status: "uploading" });
    try {
      const asset = await api.assets.uploadQuestionImage(file);
      if (!editor.isDestroyed) editor.chain().focus().insertContent({ type: "image", attrs: { asset_id: asset.id, alt: file.name, src: null } }).run();
      onDone();
    } catch (err) {
      setState({ status: "error", message: errorText(err, t.imageUploadFailed) });
    }
  };

  return (
    <>
      <PopoverHeading>{t.imageTitle}</PopoverHeading>
      <label className="relative grid cursor-pointer place-items-center gap-0.5 rounded-control border-2 border-dashed border-border px-3 py-5 text-center text-label text-muted focus-within:border-accent hover:border-accent">
        <input
          type="file"
          accept={IMAGE_TYPES.join(",")}
          disabled={state.status === "uploading"}
          className="sr-only"
          onChange={(e) => {
            void pick(e.target.files?.[0]);
            e.target.value = "";
          }}
        />
        <span className="font-medium text-accent">{state.status === "uploading" ? t.imageUploading : t.imagePick}</span>
        <span>{t.imageHint}</span>
      </label>
      {state.status === "error" && (
        <p role="alert" className="text-label text-danger">
          {state.message}
        </p>
      )}
    </>
  );
}

function ShapeBody({ editor, onDone }: { editor: Editor; onDone: () => void }) {
  const t = useAdminRichTextEditorT();
  const [kind, setKind] = useState<ShapeKind>(SHAPE_KINDS[0]);
  const [caption, setCaption] = useState("");
  const submit = () => {
    editor.chain().focus().insertContent({ type: "shape", attrs: { kind, caption: caption.trim() } }).run();
    onDone();
  };
  return (
    <>
      <PopoverHeading>{t.shapeTitle}</PopoverHeading>
      <div className="grid grid-cols-4 gap-1.5">
        {SHAPE_KINDS.map((option) => (
          <button
            key={option}
            type="button"
            aria-pressed={kind === option}
            onClick={() => setKind(option)}
            className={cn(
              "flex flex-col items-center rounded-chip border px-0.5 pt-1.5 pb-1 text-micro transition-colors",
              kind === option ? "border-accent bg-accent-soft text-accent" : "border-border bg-surface text-foreground",
            )}
          >
            <SolidShape kind={option} className="size-9.5" />
            {t.shapes[option]}
          </button>
        ))}
      </div>
      <label className="flex flex-col gap-1 text-label font-medium text-foreground">
        {t.shapeCaptionLabel}
        <Input
          fullWidth
          autoComplete="off"
          value={caption}
          placeholder={t.shapeCaptionPlaceholder}
          onChange={(e) => setCaption(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault();
              submit();
            }
          }}
        />
      </label>
      <Button variant="primary" size="sm" onPress={submit}>
        {t.shapeInsert}
      </Button>
    </>
  );
}

export function RichTextToolbar({ className }: { className?: string }) {
  const t = useAdminRichTextEditorT();
  const group = useRichTextGroup();
  const { active } = group;

  const state = useEditorState({
    editor: active,
    selector: ({ editor }) =>
      editor
        ? {
            bold: editor.isActive("bold"),
            italic: editor.isActive("italic"),
            underline: editor.isActive("underline"),
            bulletList: editor.isActive("bulletList"),
            orderedList: editor.isActive("orderedList"),
            inTable: editor.isActive("table"),
            canUndo: editor.can().undo(),
            canRedo: editor.can().redo(),
          }
        : null,
  });

  const run = (command: (chain: ReturnType<Editor["chain"]>) => ReturnType<Editor["chain"]>) => () => {
    if (active) command(active.chain().focus()).run();
  };
  const off = !active;

  return (
    <div className={cn("sticky top-0 z-10 rounded-tile border border-border bg-surface shadow-soft", className)}>
      <div role="toolbar" aria-label={t.formatToolbar} className="flex flex-wrap items-center gap-0.5 px-2 py-1.5">
        <ToolButton label={t.undo} disabled={off || !state?.canUndo} onPress={run((c) => c.undo())}>
          <Undo2 className="size-4" />
        </ToolButton>
        <ToolButton label={t.redo} disabled={off || !state?.canRedo} onPress={run((c) => c.redo())}>
          <Redo2 className="size-4" />
        </ToolButton>
        <Divider />
        <ToolButton label={t.bold} pressed={state?.bold ?? false} disabled={off} onPress={run((c) => c.toggleBold())}>
          <span className="text-small font-bold">B</span>
        </ToolButton>
        <ToolButton label={t.italic} pressed={state?.italic ?? false} disabled={off} onPress={run((c) => c.toggleItalic())}>
          <span className="text-small font-semibold italic">I</span>
        </ToolButton>
        <ToolButton label={t.underline} pressed={state?.underline ?? false} disabled={off} onPress={run((c) => c.toggleUnderline())}>
          <span className="text-small font-semibold underline">U</span>
        </ToolButton>
        <Divider />
        <ToolButton label={t.bulletList} pressed={state?.bulletList ?? false} disabled={off} onPress={run((c) => c.toggleBulletList())}>
          <List className="size-4" />
          {t.bulletList}
        </ToolButton>
        <ToolButton label={t.numberedList} pressed={state?.orderedList ?? false} disabled={off} onPress={run((c) => c.toggleOrderedList())}>
          <ListOrdered className="size-4" />
          {t.numberedList}
        </ToolButton>
        <Divider />
        <PopoverButton kind="math" icon={Sigma} label={t.math} title={t.mathInsertTitle} disabled={off}>
          {(editor, close) => <MathBody key={group.popover?.math?.pos ?? "new"} editor={editor} edit={group.popover?.math} onDone={close} />}
        </PopoverButton>
        <PopoverButton kind="table" icon={TableIcon} label={t.table} title={t.tableTitle} disabled={off}>
          {(editor, close) => <TableBody editor={editor} onDone={close} />}
        </PopoverButton>
        <PopoverButton kind="image" icon={ImageIcon} label={t.image} title={t.imageTitle} disabled={off}>
          {(editor, close) => <ImageBody editor={editor} onDone={close} />}
        </PopoverButton>
        <PopoverButton kind="shape" icon={Shapes} label={t.shape} title={t.shapeTitle} disabled={off}>
          {(editor, close) => <ShapeBody editor={editor} onDone={close} />}
        </PopoverButton>
        <span className="sr-only" aria-live="polite">
          {active ? t.writingIn(group.label) : t.noFieldHint}
        </span>
      </div>
      {state?.inTable && (
        <div role="toolbar" aria-label={t.tableToolbar} className="flex flex-wrap items-center gap-0.5 rounded-b-tile border-t border-border bg-surface-secondary px-2 py-1.5">
          <span className="pr-1.5 pl-1 text-label font-semibold text-foreground">{t.tableLabel}</span>
          <ToolButton label={t.addRow} onPress={run((c) => c.addRowAfter())}>
            {t.addRow}
          </ToolButton>
          <ToolButton label={t.addColumn} onPress={run((c) => c.addColumnAfter())}>
            {t.addColumn}
          </ToolButton>
          <Divider />
          <ToolButton label={t.deleteRow} danger onPress={run((c) => c.deleteRow())}>
            {t.deleteRow}
          </ToolButton>
          <ToolButton label={t.deleteColumn} danger onPress={run((c) => c.deleteColumn())}>
            {t.deleteColumn}
          </ToolButton>
          <ToolButton label={t.deleteTable} danger onPress={run((c) => c.deleteTable())}>
            {t.deleteTable}
          </ToolButton>
        </div>
      )}
    </div>
  );
}
