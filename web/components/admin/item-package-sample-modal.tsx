"use client";

import { useState } from "react";
import { Button, toast } from "@heroui/react";
import { FormModal } from "@/components/ui/form-modal";
import { TextInput } from "@/components/ui/form";
import { ItemElementView } from "@/components/admin/item-element";
import { api, ApiError, errorText, type SampleColumn } from "@/lib/api/client";
import { useItemPackagesT } from "@/lib/i18n/dict/item-packages";

/** "Lihat contoh soal": one sample column, drawn on the server and never stored. */
export function ItemPackageSampleModal({
  packageId,
  isOpen,
  onOpenChange,
}: {
  packageId: string;
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const t = useItemPackagesT();
  const [itemLength, setItemLength] = useState("2");
  const [options, setOptions] = useState("5");
  const [soal, setSoal] = useState("5");
  const [column, setColumn] = useState<SampleColumn | null>(null);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const generate = async () => {
    setPending(true);
    setError(null);
    try {
      setColumn(
        await api.itemPackages.sample(packageId, {
          item_length: Number(itemLength),
          options_per_soal: Number(options),
          soal_per_kolom: Number(soal),
        }),
      );
    } catch (err) {
      const message = err instanceof ApiError ? (err.fields?.elements ?? err.fields?.params ?? err.message) : errorText(err, t.sample.failed);
      setError(message);
      toast.danger(t.sample.failed, { description: message });
    } finally {
      setPending(false);
    }
  };

  return (
    <FormModal
      isOpen={isOpen}
      onOpenChange={(open) => {
        onOpenChange(open);
        if (!open) setColumn(null);
      }}
      title={t.sample.title}
      description={t.sample.description}
      size="lg"
      footer={
        <>
          <Button variant="ghost" onPress={() => onOpenChange(false)}>
            {t.sample.close}
          </Button>
          <Button variant="primary" isDisabled={pending} onPress={() => void generate()}>
            {pending ? t.sample.generating : column ? t.sample.again : t.sample.generate}
          </Button>
        </>
      }
    >
      <div className="grid gap-3 sm:grid-cols-3">
        <TextInput label={t.sample.itemLength} type="number" min={1} max={5} value={itemLength} onChange={(e) => setItemLength(e.target.value)} />
        <TextInput label={t.sample.optionsPerSoal} type="number" min={2} value={options} onChange={(e) => setOptions(e.target.value)} />
        <TextInput label={t.sample.soalPerKolom} type="number" min={1} max={100} value={soal} onChange={(e) => setSoal(e.target.value)} />
      </div>
      {error && (
        <p className="rounded-lg border border-danger/30 bg-danger-soft px-3 py-2 text-xs text-danger-soft-foreground">{error}</p>
      )}
      {column && (
        <div className="max-h-80 space-y-3 overflow-auto">
          {column.soal.map((q, at) => (
            <div key={at} className="rounded-lg border border-border p-3">
              <p className="mb-2 text-xs font-medium text-muted">{t.sample.soalHeading(at + 1)}</p>
              <div className="flex flex-wrap gap-2">
                {q.items.map((item, i) => (
                  <div key={i} className="flex items-center gap-1 rounded-md bg-surface-secondary px-2 py-1">
                    {item.elements.map((el) => (
                      <ItemElementView key={el.id} element={el} className="size-6 text-xl" />
                    ))}
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      )}
    </FormModal>
  );
}
