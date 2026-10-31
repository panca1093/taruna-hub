"use client";

import { use, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Button, toast } from "@heroui/react";
import { CloudOff, Eye, Plus, Shapes, Trash2 } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { RequireRole } from "@/components/require-role";
import { EmptyState } from "@/components/empty-state";
import { ItemElementView } from "@/components/admin/item-element";
import { ItemPackageAddModal } from "@/components/admin/item-package-add-modal";
import { ItemPackageSampleModal } from "@/components/admin/item-package-sample-modal";
import { api, errorText, type ItemPackageDetail } from "@/lib/api/client";
import { useItemPackagesT } from "@/lib/i18n/dict/item-packages";

export default function ItemPackageDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const t = useItemPackagesT();
  const router = useRouter();
  const [detail, setDetail] = useState<ItemPackageDetail | null>(null);
  const [loadFailed, setLoadFailed] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);
  const [addOpen, setAddOpen] = useState(false);
  const [sampleOpen, setSampleOpen] = useState(false);
  const [removing, setRemoving] = useState<string | null>(null);
  const [attribution, setAttribution] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    api.itemPackages
      .get(id)
      .then((loaded) => {
        if (cancelled) return;
        setDetail(loaded);
        setLoadFailed(false);
      })
      .catch(() => {
        if (!cancelled) setLoadFailed(true);
      });
    return () => {
      cancelled = true;
    };
  }, [id, reloadKey]);

  const isEmoji = detail?.package.type === "emoji";
  useEffect(() => {
    if (!isEmoji) return;
    let cancelled = false;
    api.itemPackages
      .catalog()
      .then((catalog) => {
        if (!cancelled) setAttribution(catalog.emoji_attribution);
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [isEmoji]);

  const reload = () => setReloadKey((key) => key + 1);

  const retry = () => {
    setDetail(null);
    setLoadFailed(false);
    reload();
  };

  const remove = async (elementId: string) => {
    setRemoving(elementId);
    try {
      await api.itemPackages.removeElement(id, elementId);
      toast.success(t.toastElementRemoved);
      reload();
    } catch (err) {
      // The guard answers with a message naming the minimum a test still needs; it is the useful copy.
      toast.danger(t.toastCannotRemove, { description: errorText(err, t.toastCannotRemove) });
    } finally {
      setRemoving(null);
    }
  };

  const pkg = detail?.package;

  return (
    <RequireRole roles={["admin", "operator"]} title={t.requireRoleTitle} deniedMessage={t.requireRoleDenied}>
      <AppShell
        title={pkg?.name ?? t.title}
        description={pkg ? `${t.types[pkg.type]} · ${t.revisionLabel(pkg.revision)} · ${t.elementsCount(pkg.element_count)}` : undefined}
        breadcrumb={[{ label: t.title, href: "/admin/item-packages" }, { label: pkg?.name ?? "…" }]}
        actions={
          <div className="flex gap-2">
            {pkg && (
              <>
                <Button variant="secondary" size="sm" isDisabled={pkg.element_count === 0} onPress={() => setSampleOpen(true)}>
                  <Eye className="size-4" /> {t.sampleAction}
                </Button>
                <Button variant="primary" size="sm" onPress={() => setAddOpen(true)}>
                  <Plus className="size-4" /> {t.addElements}
                </Button>
              </>
            )}
          </div>
        }
      >
        {loadFailed ? (
          <EmptyState
            icon={CloudOff}
            title={t.loadErrorTitle}
            description={t.loadErrorDescription}
            action={
              <div className="flex gap-2">
                <Button variant="primary" size="sm" onPress={retry}>{t.retry}</Button>
                <Button variant="ghost" size="sm" onPress={() => router.push("/admin/item-packages")}>{t.back}</Button>
              </div>
            }
          />
        ) : detail === null || !pkg ? (
          <p className="py-14 text-center text-small text-muted">{t.loading}</p>
        ) : (
          <>
            {pkg.required_count > 0 && <p className="mb-3 text-caption text-muted">{t.requiredHint(pkg.required_count)}</p>}
            {detail.elements.length === 0 ? (
              <EmptyState
                icon={Shapes}
                title={t.elementsEmptyTitle}
                description={t.elementsEmptyDescription}
                action={<Button variant="primary" size="sm" onPress={() => setAddOpen(true)}>{t.addElements}</Button>}
              />
            ) : (
              <section aria-label={t.elementsHeading} className="grid grid-cols-3 gap-3 sm:grid-cols-5 lg:grid-cols-8">
                {detail.elements.map((element) => (
                  <div key={element.id} className="group relative flex aspect-square flex-col items-center justify-center rounded-tile border border-border bg-surface p-2 transition-colors hover:border-accent/40">
                    <ItemElementView element={element} className="size-10 text-3xl" />
                    <span className="mt-1 max-w-full truncate text-label text-muted">{element.kind === "symbol" ? "" : element.label}</span>
                    <Button
                      variant="ghost"
                      size="sm"
                      isIconOnly
                      aria-label={t.removeElement(element.label)}
                      className="absolute top-1 right-1 size-6 min-w-0 text-danger"
                      isDisabled={removing === element.id}
                      onPress={() => void remove(element.id)}
                    >
                      <Trash2 className="size-3" />
                    </Button>
                  </div>
                ))}
              </section>
            )}
            {isEmoji && attribution && <p className="mt-6 text-caption text-muted">{attribution}</p>}
            <ItemPackageAddModal packageId={id} type={pkg.type} isOpen={addOpen} onOpenChange={setAddOpen} onSaved={reload} />
            <ItemPackageSampleModal packageId={id} isOpen={sampleOpen} onOpenChange={setSampleOpen} />
          </>
        )}
      </AppShell>
    </RequireRole>
  );
}
