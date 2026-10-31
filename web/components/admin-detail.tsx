import { FileImage } from "lucide-react";
import { cn } from "@/lib/cn";
import { Icon } from "./icon";

/** Label/value pairs in the prototype's .metadata grid, used inside admin detail dialogs. */
export function DetailGrid({ children, className }: { children: React.ReactNode; className?: string }) {
  return <dl className={cn("grid grid-cols-2 gap-x-6 gap-y-5", className)}>{children}</dl>;
}

export function DetailItem({ label, wide, children }: { label: string; wide?: boolean; children: React.ReactNode }) {
  return (
    <div className={cn("min-w-0", wide && "col-span-2")}>
      <dt className="mb-1 text-label text-muted">{label}</dt>
      <dd className="text-small font-medium break-words text-foreground">{children}</dd>
    </div>
  );
}

/** Large amount line at the top of a payment dialog (prototype .order-summary .amount). */
export function DetailAmount({ children }: { children: React.ReactNode }) {
  return <p className="font-display text-metric-lg tabular-nums text-foreground">{children}</p>;
}

/** Proof-of-payment card that opens the stored file (prototype proof panel). */
export function ProofButton({ title, hint, onPress }: { title: string; hint: string; onPress: () => void }) {
  return (
    <button
      type="button"
      onClick={onPress}
      className="flex items-center gap-3.5 rounded-tile border border-dashed border-border bg-background p-4 text-left outline-none transition-colors hover:border-accent focus-visible:ring-2 focus-visible:ring-focus"
    >
      <span className="grid size-10 shrink-0 place-items-center rounded-control bg-peach text-accent">
        <Icon icon={FileImage} size="md" />
      </span>
      <span className="min-w-0">
        <span className="block text-small font-medium text-foreground">{title}</span>
        <span className="mt-0.5 block text-label text-muted">{hint}</span>
      </span>
    </button>
  );
}

/** Inline loading line for admin tables. */
export function LoadingNote({ children }: { children: React.ReactNode }) {
  return <p className="py-14 text-center text-small text-muted">{children}</p>;
}
