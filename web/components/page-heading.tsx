import Link from "next/link";
import { ChevronRight } from "lucide-react";
import { cn } from "@/lib/cn";

export interface Crumb {
  label: string;
  href?: string;
}

export function Breadcrumb({ items, className }: { items: Crumb[]; className?: string }) {
  return (
    <nav aria-label="Breadcrumb" className={cn("mb-6 flex flex-wrap items-center gap-2 text-label text-muted", className)}>
      {items.map((item, index) => {
        const last = index === items.length - 1;
        return (
          <span key={`${item.label}-${index}`} className="inline-flex items-center gap-2">
            {item.href && !last ? (
              <Link href={item.href} className="rounded-chip outline-none hover:text-accent focus-visible:ring-2 focus-visible:ring-focus">
                {item.label}
              </Link>
            ) : (
              <span aria-current={last ? "page" : undefined} className={last ? "text-foreground" : undefined}>{item.label}</span>
            )}
            {!last && <ChevronRight className="size-3.5" aria-hidden />}
          </span>
        );
      })}
    </nav>
  );
}

/** Page title block: optional breadcrumb, Geist h1, muted description, actions on the right. */
export function PageHeading({
  title,
  description,
  breadcrumb,
  actions,
  size = "display",
  className,
}: {
  title: string;
  description?: string;
  breadcrumb?: Crumb[];
  actions?: React.ReactNode;
  /** "display" = 36px (student), "compact" = 30px (admin, dense pages). */
  size?: "display" | "compact";
  className?: string;
}) {
  return (
    <div className={cn("mb-7", className)}>
      {breadcrumb && breadcrumb.length > 0 && <Breadcrumb items={breadcrumb} />}
      <div className="flex flex-wrap items-start justify-between gap-x-6 gap-y-4">
        <div className="min-w-0">
          <h1 className={cn("font-display text-balance", size === "display" ? "text-display-sm md:text-display" : "text-display-sm")}>{title}</h1>
          {description && <p className="mt-2 max-w-prose text-small text-muted">{description}</p>}
        </div>
        {actions && <div className="flex flex-wrap items-center gap-2.5">{actions}</div>}
      </div>
    </div>
  );
}
