"use client";

import { useRef } from "react";
import { Button, toast } from "@heroui/react";
import { Bold, ImagePlus, Italic, List, ListOrdered, Sigma, type LucideIcon } from "lucide-react";
import { cn } from "@/lib/cn";
import { useAdminRichTextEditorT } from "@/lib/i18n/dict/admin-rich-text-editor";

const MAX_IMAGE_BYTES = 5 * 1024 * 1024;

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
  const fileInputRef = useRef<HTMLInputElement>(null);

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

  const insertImage = (file: File) => {
    if (file.size > MAX_IMAGE_BYTES) {
      toast.danger(t.toastImageTooBig, { description: t.toastImageTooBigDescription });
      return;
    }
    const reader = new FileReader();
    reader.onload = () => {
      const dataUrl = reader.result as string;
      const el = textareaRef.current;
      const cursor = el ? el.selectionStart : value.length;
      const needsLeadingBreak = cursor > 0 && value[cursor - 1] !== "\n";
      const insertText = `${needsLeadingBreak ? "\n" : ""}![${t.imageAlt}](${dataUrl})\n`;
      const next = value.slice(0, cursor) + insertText + value.slice(cursor);
      onChange(next);
      requestAnimationFrame(() => {
        el?.focus();
        el?.setSelectionRange(cursor + insertText.length, cursor + insertText.length);
      });
    };
    reader.readAsDataURL(file);
  };

  return (
    <div className={cn("flex flex-col gap-1.5", className)}>
      {label && (
        <label className="text-[13px] font-medium text-foreground">
          {label} {required && <span className="text-danger">*</span>}
        </label>
      )}
      <div className="overflow-hidden rounded-lg border border-border bg-field focus-within:border-accent">
        <div className="flex items-center gap-0.5 border-b border-border bg-surface-secondary px-1.5 py-1">
          <ToolbarButton icon={Bold} label={t.bold} onPress={() => applyWrap("**", "**", t.boldPlaceholder)} />
          <ToolbarButton icon={Italic} label={t.italic} onPress={() => applyWrap("*", "*", t.italicPlaceholder)} />
          <span className="mx-1 h-4 w-px bg-border" />
          <ToolbarButton icon={List} label={t.bulletList} onPress={() => applyLinePrefix("- ")} />
          <ToolbarButton icon={ListOrdered} label={t.numberedList} onPress={() => applyLinePrefix("1. ")} />
          <span className="mx-1 h-4 w-px bg-border" />
          <ToolbarButton icon={Sigma} label={t.insertLatex} onPress={() => applyWrap("$", "$", "x^2")} />
          <ToolbarButton icon={ImagePlus} label={t.insertImage} onPress={() => fileInputRef.current?.click()} />
          <input
            ref={fileInputRef}
            type="file"
            accept="image/*"
            className="hidden"
            onChange={(e) => {
              const file = e.target.files?.[0];
              if (file) insertImage(file);
              e.target.value = "";
            }}
          />
        </div>
        <textarea
          ref={textareaRef}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={placeholder}
          rows={rows}
          required={required}
          className="w-full resize-y bg-transparent px-3.5 py-3 text-sm text-field-foreground outline-none"
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
