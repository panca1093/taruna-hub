"use client";

import { useEffect, useMemo } from "react";
import { EditorContent, useEditor } from "@tiptap/react";
import type { EditorView } from "@tiptap/pm/view";
import StarterKit from "@tiptap/starter-kit";
import { ListItem } from "@tiptap/extension-list";
import { TableCell, TableHeader, TableKit } from "@tiptap/extension-table";
import { Placeholder } from "@tiptap/extensions";
import { cn } from "@/lib/cn";
import type { RichDoc } from "@/lib/rich-doc";
import { ImageNode, MathNode, ShapeNode } from "./rich-text/nodes";
import { useRichTextGroup } from "./rich-text/group";

export { RichTextGroup } from "./rich-text/group";
export { RichTextToolbar } from "./rich-text/toolbar";

// These content rules are the API's allow-list; the editor schema must not accept what the server rejects.
const SchemaListItem = ListItem.extend({ content: "paragraph (paragraph | bulletList | orderedList)*" });
const CELL_CONTENT = "(paragraph | bulletList | orderedList | shape | image)+";
const SchemaTableCell = TableCell.extend({ content: CELL_CONTENT });
const SchemaTableHeader = TableHeader.extend({ content: CELL_CONTENT });

/** Scrolls only the nearest scrollable ancestor (the modal body), clear of the sticky toolbar; the page behind never moves. */
function scrollSelectionIntoView(view: EditorView) {
  let box: HTMLElement | null = view.dom.parentElement;
  while (box && !/(auto|scroll)/.test(getComputedStyle(box).overflowY)) box = box.parentElement;
  if (!box) return false;
  const caret = view.coordsAtPos(view.state.selection.head);
  const bounds = box.getBoundingClientRect();
  const toolbar = box.querySelector<HTMLElement>("[data-rich-toolbar]");
  const top = bounds.top + (toolbar?.offsetHeight ?? 0) + SCROLL_MARGIN;
  const bottom = bounds.bottom - SCROLL_MARGIN;
  if (caret.top < top) box.scrollTop -= top - caret.top;
  else if (caret.bottom > bottom) box.scrollTop += caret.bottom - bottom;
  return true;
}

const SCROLL_MARGIN = 16;

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
        listItem: false,
      }),
      SchemaListItem,
      TableKit.configure({ table: { resizable: false }, tableCell: false, tableHeader: false }),
      SchemaTableCell,
      SchemaTableHeader,
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
    // A doc with no blocks has nowhere to put the cursor, so an empty value starts as Tiptap's own empty paragraph.
    content: value.content.length > 0 ? value : undefined,
    editorProps: {
      handleScrollToSelection: scrollSelectionIntoView,
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
