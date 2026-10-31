"use client";

import { useRef } from "react";
import { Button } from "@heroui/react";
import { Bold, Italic, List, ListOrdered, Sigma, type LucideIcon } from "lucide-react";
import { cn } from "@/lib/cn";
import { useAdminRichTextEditorT } from "@/lib/i18n/dict/admin-rich-text-editor";

// No image button: uploads and assets are deferred (P06), and the API refuses `data:` URLs, so the
// editor only offers the formatting it can actually store.
export function RichTextEditor({
  label,
  value,
  onChange,
  placeholder,
  rows = 6,
  required,
  className,
}: {
  label?: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  rows?: number;
  required?: boolean;
  className?: string;
}) {
  const t = useAdminRichTextEditorT();
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  const applyWrap = (before: string, after: string, placeholderText: string) => {
    const el = textareaRef.current;
    if (!el) return;
    const start = el.selectionStart;
    const end = el.selectionEnd;
    const selected = value.slice(start, end) || placeholderText;
    const next = value.slice(0, start) + before + selected + after + value.slice(end);
    onChange(next);
    requestAnimationFrame(() => {
      el.focus();
      el.setSelectionRange(start + before.length, start + before.length + selected.length);
    });
  };

  const applyLinePrefix = (prefix: string) => {
    const el = textareaRef.current;
    if (!el) return;
    const start = el.selectionStart;
    const end = el.selectionEnd;
    const lineStart = value.lastIndexOf("\n", start - 1) + 1;
    const next = value.slice(0, lineStart) + prefix + value.slice(lineStart);
    onChange(next);
    requestAnimationFrame(() => {
      el.focus();
      el.setSelectionRange(start + prefix.length, end + prefix.length);
    });
  };

  return (
    <div className={cn("flex flex-col gap-1.5", className)}>
      {label && (
        <label className="text-caption font-medium text-foreground">
          {label} {required && <span className="text-danger">*</span>}
        </label>
      )}
      <div className="overflow-hidden rounded-control border border-border bg-field focus-within:border-accent">
        <div className="flex items-center gap-0.5 border-b border-border bg-surface-secondary px-1.5 py-1">
          <ToolbarButton icon={Bold} label={t.bold} onPress={() => applyWrap("**", "**", t.boldPlaceholder)} />
          <ToolbarButton icon={Italic} label={t.italic} onPress={() => applyWrap("*", "*", t.italicPlaceholder)} />
          <span className="mx-1 h-4 w-px bg-border" />
          <ToolbarButton icon={List} label={t.bulletList} onPress={() => applyLinePrefix("- ")} />
          <ToolbarButton icon={ListOrdered} label={t.numberedList} onPress={() => applyLinePrefix("1. ")} />
          <span className="mx-1 h-4 w-px bg-border" />
          <ToolbarButton icon={Sigma} label={t.insertLatex} onPress={() => applyWrap("$", "$", "x^2")} />
        </div>
        <textarea
          ref={textareaRef}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={placeholder}
          rows={rows}
          required={required}
          className="w-full resize-y bg-transparent px-3.5 py-3 text-small text-field-foreground outline-none"
        />
      </div>
    </div>
  );
}

function ToolbarButton({ icon: Icon, label, onPress }: { icon: LucideIcon; label: string; onPress: () => void }) {
  return (
    <Button type="button" variant="ghost" size="sm" isIconOnly aria-label={label} onPress={onPress}>
      <Icon className="size-3.5" />
    </Button>
  );
}
