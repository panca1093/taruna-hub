"use client";

import { useEffect, useRef, useState } from "react";
import { Button, toast } from "@heroui/react";
import { CheckCircle2, Download, FileUp, XCircle } from "lucide-react";
import { FormModal } from "@/components/ui/form-modal";
import { TextInput } from "@/components/ui/form";
import { EmojiImage, ShapeIcon } from "@/components/admin/item-element";
import {
  api,
  ApiError,
  errorText,
  type ElementUpload,
  type ItemCatalog,
  type ItemElementPreview,
  type ItemPackageType,
} from "@/lib/api/client";
import { cn } from "@/lib/cn";
import { useItemPackagesT } from "@/lib/i18n/dict/item-packages";

export function ItemPackageAddModal({
  packageId,
  type,
  isOpen,
  onOpenChange,
  onSaved,
}: {
  packageId: string;
  type: ItemPackageType;
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  onSaved: () => void;
}) {
  const t = useItemPackagesT();
  const [text, setText] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [files, setFiles] = useState<File[]>([]);
  const [ids, setIds] = useState<Set<string>>(new Set());
  const [query, setQuery] = useState("");
  const [catalog, setCatalog] = useState<ItemCatalog | null>(null);
  const [catalogFailed, setCatalogFailed] = useState(false);
  const [preview, setPreview] = useState<ItemElementPreview | null>(null);
  const [pending, setPending] = useState(false);
  const [dragging, setDragging] = useState(false);
  const [inlineError, setInlineError] = useState<string | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);
  const latestRequest = useRef(0);

  const needsCatalog = isOpen && type !== "image";
  useEffect(() => {
    if (!needsCatalog || catalog) return;
    let cancelled = false;
    api.itemPackages
      .catalog()
      .then((loaded) => {
        if (!cancelled) setCatalog(loaded);
      })
      .catch(() => {
        if (!cancelled) setCatalogFailed(true);
      });
    return () => {
      cancelled = true;
    };
  }, [needsCatalog, catalog]);

  const input = (): ElementUpload => {
    if (type === "symbol") return { symbols: text, file: file ?? undefined };
    if (type === "image") return { files };
    return { ids: [...ids] };
  };
  const hasInput =
    (type === "symbol" && (text.trim() !== "" || file !== null)) ||
    (type === "image" && files.length > 0) ||
    ((type === "emoji" || type === "shape") && ids.size > 0);

  const reset = () => {
    latestRequest.current++;
    setText("");
    setFile(null);
    setFiles([]);
    setIds(new Set());
    setQuery("");
    setPreview(null);
    setInlineError(null);
    setPending(false);
    setDragging(false);
  };

  // Any change of the input invalidates the preview: the user confirms exactly what was checked.
  const edited = () => {
    latestRequest.current++;
    setPreview(null);
    setInlineError(null);
    setPending(false);
  };

  const showError = (err: unknown) => {
    const message =
      err instanceof ApiError
        ? (err.fields?.file ?? err.fields?.elements ?? err.message)
        : errorText(err, t.add.failedDescription);
    setInlineError(message);
    toast.danger(t.add.failedTitle, { description: message });
  };

  const runPreview = async () => {
    setPending(true);
    setInlineError(null);
    const request = ++latestRequest.current;
    try {
      const result = await api.itemPackages.preview(packageId, input());
      if (request === latestRequest.current) setPreview(result);
    } catch (err) {
      if (request === latestRequest.current) showError(err);
    } finally {
      if (request === latestRequest.current) setPending(false);
    }
  };

  const commit = async () => {
    setPending(true);
    try {
      const result = await api.itemPackages.commit(packageId, input());
      toast.success(t.add.committedTitle, {
        description: t.add.committedDescription(result.added, result.already_present, result.rejected),
      });
      onOpenChange(false);
      reset();
      onSaved();
    } catch (err) {
      showError(err);
    } finally {
      setPending(false);
    }
  };

  const downloadTemplate = async (format: "xlsx" | "csv") => {
    try {
      const blob = await api.itemPackages.symbolTemplate(format);
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = `template-simbol.${format}`;
      document.body.appendChild(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(url);
      toast.success(t.add.templateDownloaded, { description: `template-simbol.${format}` });
    } catch (err) {
      toast.danger(t.add.templateFailed, { description: errorText(err, t.add.templateFailed) });
    }
  };

  const toggleId = (value: string) => {
    edited();
    setIds((previous) => {
      const next = new Set(previous);
      if (next.has(value)) next.delete(value);
      else next.add(value);
      return next;
    });
  };

  const pickImages = (picked: FileList | null) => {
    if (!picked || picked.length === 0) return;
    if (fileRef.current) fileRef.current.value = "";
    edited();
    setFiles(Array.from(picked));
  };

  const needle = query.trim().toLowerCase();
  // Picked emoji stay visible while searching, so a selection can always be seen and undone.
  const matchingEmoji = (catalog?.emoji ?? []).filter(
    (item) => !needle || ids.has(item.id) || item.name.toLowerCase().includes(needle),
  );
  const flatShapes = (catalog?.shapes ?? []).filter((shape) => shape.group === "flat");
  const solidShapes = (catalog?.shapes ?? []).filter((shape) => shape.group === "solid");

  const reasonText = (reason: string | undefined, fallback: string | undefined) =>
    reason ? (t.reasons[reason] ?? fallback ?? reason) : "";

  const catalogGrid = (kind: "emoji" | "shape", items: { id: string; name: string }[]) => {
    if (catalogFailed) return <p className="text-xs text-danger">{t.add.catalogFailed}</p>;
    if (!catalog) return <p className="text-xs text-muted">{t.add.catalogLoading}</p>;
    return <CatalogGrid kind={kind} items={items} selected={ids} disabled={pending} onToggle={toggleId} />;
  };

  return (
    <FormModal
      isOpen={isOpen}
      onOpenChange={(open) => {
        onOpenChange(open);
        if (!open) reset();
      }}
      title={t.add.title(t.types[type])}
      description={t.add.description}
      size="lg"
      footer={
        <>
          <Button variant="ghost" onPress={() => onOpenChange(false)}>
            {t.cancel}
          </Button>
          {preview ? (
            <Button variant="primary" isDisabled={pending || preview.accepted === 0} onPress={() => void commit()}>
              {pending ? t.add.committing : t.add.confirm(preview.accepted)}
            </Button>
          ) : (
            <Button variant="primary" isDisabled={pending || !hasInput} onPress={() => void runPreview()}>
              {pending ? t.add.previewing : t.add.previewAction}
            </Button>
          )}
        </>
      }
    >
      {type === "symbol" && (
        <>
          <div>
            <label htmlFor="symbol-text" className="mb-1.5 block text-[13px] font-medium text-foreground">
              {t.add.symbolTextLabel}
            </label>
            <textarea
              id="symbol-text"
              rows={3}
              value={text}
              placeholder={t.add.symbolTextPlaceholder}
              onChange={(e) => {
                edited();
                setText(e.target.value);
              }}
              className="w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm text-foreground placeholder:text-muted focus-visible:ring-2 focus-visible:ring-focus focus-visible:outline-none"
            />
            <p className="mt-1 text-xs text-muted">{t.add.symbolTextHelp}</p>
          </div>
          <div>
            <p className="mb-1.5 text-[13px] font-medium text-foreground">{t.add.paletteHeading}</p>
            <div className="flex flex-wrap gap-1.5">
              {(catalog?.symbol_palette ?? []).map((symbol) => (
                <button
                  key={symbol}
                  type="button"
                  className="size-8 rounded-md border border-border bg-surface text-base text-foreground hover:border-accent/50"
                  onClick={() => {
                    edited();
                    setText((prev) => (prev && !/\s$/.test(prev) ? `${prev} ${symbol}` : `${prev}${symbol}`));
                  }}
                >
                  {symbol}
                </button>
              ))}
              {!catalog && <span className="text-xs text-muted">{catalogFailed ? t.add.catalogFailed : t.add.catalogLoading}</span>}
            </div>
            <p className="mt-1 text-xs text-muted">{t.add.paletteHelp}</p>
          </div>
          <div>
            <p className="mb-1.5 text-[13px] font-medium text-foreground">{t.add.fileHeading}</p>
            <input
              ref={fileRef}
              type="file"
              accept=".csv,.txt,.xlsx"
              className="hidden"
              onChange={(e) => {
                edited();
                setFile(e.target.files?.[0] ?? null);
                if (fileRef.current) fileRef.current.value = "";
              }}
            />
            <div className="flex flex-wrap items-center gap-2">
              <Button variant="secondary" size="sm" onPress={() => fileRef.current?.click()}>
                <FileUp className="size-3.5" /> {file?.name ?? t.add.chooseFile}
              </Button>
              <Button variant="ghost" size="sm" onPress={() => void downloadTemplate("xlsx")}>
                <Download className="size-3.5" /> {t.add.templateXlsx}
              </Button>
              <Button variant="ghost" size="sm" onPress={() => void downloadTemplate("csv")}>
                <Download className="size-3.5" /> {t.add.templateCsv}
              </Button>
            </div>
          </div>
        </>
      )}

      {type === "image" && (
        <>
          <input
            ref={fileRef}
            type="file"
            multiple
            accept=".png,.jpg,.jpeg,.webp,.zip"
            className="hidden"
            onChange={(e) => pickImages(e.target.files)}
          />
          <button
            type="button"
            onClick={() => fileRef.current?.click()}
            onDragOver={(e) => {
              e.preventDefault();
              setDragging(true);
            }}
            onDragLeave={() => setDragging(false)}
            onDrop={(e) => {
              e.preventDefault();
              setDragging(false);
              pickImages(e.dataTransfer.files);
            }}
            className={cn(
              "flex flex-col items-center gap-2 rounded-xl border border-dashed border-border py-8 text-center transition-colors hover:border-accent/50 hover:bg-accent-soft/30",
              dragging && "border-accent bg-accent-soft/40",
            )}
          >
            <FileUp className="size-5 text-muted" />
            <span className="text-sm font-medium text-foreground">
              {files.length > 0 ? files.map((f) => f.name).slice(0, 3).join(", ") + (files.length > 3 ? ` +${files.length - 3}` : "") : t.add.imagePrompt}
            </span>
          </button>
          <p className="text-xs text-muted">{t.add.imageHelp}</p>
        </>
      )}

      {(type === "emoji" || type === "shape") && (
        <div>
          <p className="mb-1.5 text-[13px] font-medium text-foreground">
            {type === "emoji" ? t.add.emojiHeading : t.add.shapeHeading}
            <span className="ml-2 font-normal text-muted">
              {type === "emoji" ? t.add.emojiSelected(ids.size) : t.add.shapeSelected(ids.size)}
            </span>
          </p>
          {type === "emoji" && (
            <>
              <TextInput
                type="search"
                aria-label={t.add.emojiSearchLabel}
                placeholder={t.add.emojiSearchPlaceholder}
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                className="mb-2"
              />
              {catalog && matchingEmoji.length === 0 ? (
                <p className="text-xs text-muted">{t.add.emojiNoMatch}</p>
              ) : (
                catalogGrid("emoji", matchingEmoji)
              )}
            </>
          )}
          {type === "shape" && (
            <>
              <div role="group" aria-labelledby="shape-group-flat">
                {catalog && (
                  <p id="shape-group-flat" className="mb-1.5 text-xs font-medium text-muted">
                    {t.add.shapeGroupFlat}
                  </p>
                )}
                {catalogGrid("shape", flatShapes)}
              </div>
              {catalog && (
                <div role="group" aria-labelledby="shape-group-solid">
                  <p id="shape-group-solid" className="mt-3 mb-1.5 text-xs font-medium text-muted">
                    {t.add.shapeGroupSolid}
                  </p>
                  {catalogGrid("shape", solidShapes)}
                </div>
              )}
            </>
          )}
          {type === "emoji" && catalog && (
            <p className="mt-2 text-xs text-muted">
              {t.add.attribution}: {catalog.emoji_attribution}
            </p>
          )}
        </div>
      )}

      {inlineError && (
        <p className="flex items-start gap-1.5 rounded-lg border border-danger/30 bg-danger-soft px-3 py-2 text-xs text-danger-soft-foreground">
          <XCircle className="mt-px size-3.5 shrink-0" /> {inlineError}
        </p>
      )}

      {preview && (
        <div>
          <p className="mb-2 flex flex-wrap items-center gap-1.5 text-xs font-medium text-success">
            <CheckCircle2 className="size-3.5" /> {t.add.summary(preview.accepted, preview.rejected)}
            <button type="button" className="ml-2 font-normal text-muted underline" onClick={edited}>
              {t.add.reset}
            </button>
          </p>
          {preview.items.length === 0 ? (
            <p className="text-xs text-muted">{t.add.nothingNew}</p>
          ) : (
            <div className="max-h-72 overflow-auto rounded-lg border border-border">
              <table className="w-full text-xs">
                <thead className="sticky top-0 bg-surface-secondary text-muted">
                  <tr>
                    <th className="px-3 py-2 text-left font-medium">{t.add.colItem}</th>
                    <th className="px-3 py-2 text-left font-medium">{t.add.colStatus}</th>
                    <th className="px-3 py-2 text-left font-medium">{t.add.colReason}</th>
                  </tr>
                </thead>
                <tbody>
                  {preview.items.map((item, at) => (
                    <tr key={`${item.label}-${at}`} className={cn("border-t border-border", !item.accepted && "bg-danger-soft/40")}>
                      <td className="max-w-48 truncate px-3 py-2 align-top text-foreground">{item.label}</td>
                      <td className="px-3 py-2 align-top">
                        {item.accepted ? (
                          <span className="flex items-center gap-1 text-success">
                            <CheckCircle2 className="size-3.5" /> {t.add.accepted}
                          </span>
                        ) : (
                          <span className="flex items-center gap-1 text-danger">
                            <XCircle className="size-3.5" /> {t.add.rejected}
                          </span>
                        )}
                      </td>
                      <td className="px-3 py-2 align-top text-muted">{reasonText(item.reason, item.message)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}
    </FormModal>
  );
}

function CatalogGrid({
  kind,
  items,
  selected,
  disabled,
  onToggle,
}: {
  kind: "emoji" | "shape";
  items: { id: string; name: string }[];
  selected: Set<string>;
  disabled: boolean;
  onToggle: (id: string) => void;
}) {
  return (
    <div className="grid max-h-64 grid-cols-6 gap-2 overflow-auto sm:grid-cols-8">
      {items.map((item) => {
        const on = selected.has(item.id);
        return (
          <label
            key={item.id}
            title={item.name}
            className={cn(
              "flex cursor-pointer items-center justify-center rounded-lg border border-border p-2 transition-colors hover:border-accent/50 has-focus-visible:ring-2 has-focus-visible:ring-focus",
              on && "border-accent bg-accent-soft/40",
            )}
          >
            <input
              type="checkbox"
              className="sr-only"
              aria-label={item.name}
              checked={on}
              disabled={disabled}
              onChange={() => onToggle(item.id)}
            />
            {kind === "emoji" ? <EmojiImage id={item.id} label={item.name} /> : <ShapeIcon shape={item.id} className="text-foreground" />}
          </label>
        );
      })}
    </div>
  );
}
