"use client";

import { useEffect, useMemo } from "react";
import { EditorContent, useEditor } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";
import { TableKit } from "@tiptap/extension-table";
import { Placeholder } from "@tiptap/extensions";
import { cn } from "@/lib/cn";
import type { RichDoc } from "@/lib/rich-doc";
import { ImageNode, MathNode, ShapeNode } from "./rich-text/nodes";
import { useRichTextGroup } from "./rich-text/group";

export { RichTextGroup } from "./rich-text/group";
export { RichTextToolbar } from "./rich-text/toolbar";

const SIZE_CLASS = { option: "min-h-6", explanation: "min-h-26", content: "min-h-55" } as const;

/** One editor of a RichTextGroup; it keeps a single instance and pushes its JSON up on every change. */
export function RichTextField({
  label,
  ariaLabel,
  value,
  onChange,
  placeholder,
  size = "option",
  invalid,
  correct,
  className,
}: {
  label: string;
  ariaLabel?: string;
  value: RichDoc;
  onChange: (doc: RichDoc) => void;
  placeholder?: string;
  size?: keyof typeof SIZE_CLASS;
  invalid?: boolean;
  correct?: boolean;
  className?: string;
}) {
  const group = useRichTextGroup();
  const { activate, release } = group;

  const extensions = useMemo(
    () => [
      StarterKit.configure({
        heading: false,
        codeBlock: false,
        blockquote: false,
        horizontalRule: false,
        code: false,
        strike: false,
        link: false,
      }),
      TableKit.configure({ table: { resizable: false } }),
      MathNode,
      ShapeNode,
      ImageNode,
      Placeholder.configure({ placeholder: placeholder ?? "" }),
    ],
    // eslint-disable-next-line react-hooks/exhaustive-deps -- a placeholder change must not rebuild the editor
    [],
  );

  const editor = useEditor({
    immediatelyRender: false,
    extensions,
    content: value,
    editorProps: {
      attributes: { class: cn("rich-text px-3.5 py-3 outline-none", SIZE_CLASS[size]), "aria-label": ariaLabel ?? label, role: "textbox", "aria-multiline": "true" },
    },
    onUpdate: ({ editor: instance }) => onChange(instance.getJSON() as RichDoc),
    onFocus: ({ editor: instance }) => activate(instance, label),
  });

  useEffect(() => {
    if (!editor) return;
    return () => release(editor);
  }, [editor, release]);

  const focused = !!editor && group.active === editor;

  return (
    <div
      className={cn(
        "cursor-text rounded-control border transition-colors",
        invalid
          ? "border-danger"
          : focused
            ? "border-accent ring-3 ring-accent-soft"
            : correct
              ? "border-success bg-success-soft"
              : "border-border bg-field",
        className,
      )}
    >
      <EditorContent editor={editor} />
    </div>
  );
}
