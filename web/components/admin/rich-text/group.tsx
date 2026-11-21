"use client";

import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from "react";
import type { Editor } from "@tiptap/react";

export type PopoverKind = "math" | "table" | "image" | "shape";

export interface MathEdit {
  pos: number;
  latex: string;
}

interface Group {
  active: Editor | null;
  label: string;
  popover: { kind: PopoverKind; math?: MathEdit } | null;
  activate: (editor: Editor, label: string) => void;
  release: (editor: Editor) => void;
  openPopover: (kind: PopoverKind, math?: MathEdit) => void;
  closePopover: (kind?: PopoverKind) => void;
}

const GroupContext = createContext<Group | null>(null);

/** One toolbar serves every editor inside: it acts on the editor that last took focus. */
export function RichTextGroup({ children }: { children: ReactNode }) {
  const [focus, setFocus] = useState<{ editor: Editor; label: string } | null>(null);
  const [popover, setPopover] = useState<Group["popover"]>(null);

  const activate = useCallback((editor: Editor, label: string) => setFocus({ editor, label }), []);
  const release = useCallback((editor: Editor) => {
    setFocus((f) => (f?.editor === editor ? null : f));
    setPopover(null);
  }, []);
  const openPopover = useCallback((kind: PopoverKind, math?: MathEdit) => setPopover({ kind, math }), []);
  // A late close from a popover that is no longer the open one (an upload finishing) must not close its successor.
  const closePopover = useCallback((kind?: PopoverKind) => setPopover((p) => (kind === undefined || p?.kind === kind ? null : p)), []);

  const value = useMemo<Group>(
    () => ({ active: focus?.editor ?? null, label: focus?.label ?? "", popover, activate, release, openPopover, closePopover }),
    [focus, popover, activate, release, openPopover, closePopover],
  );
  return <GroupContext value={value}>{children}</GroupContext>;
}

export function useRichTextGroup(): Group {
  const group = useContext(GroupContext);
  if (!group) throw new Error("RichTextField and RichTextToolbar must sit inside RichTextGroup");
  return group;
}
