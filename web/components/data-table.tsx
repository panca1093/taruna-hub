import { cn } from "@/lib/cn";

export interface Column<T> {
  key: string;
  header: string;
  render: (row: T) => React.ReactNode;
  className?: string;
  headerClassName?: string;
}

export function DataTable<T>({
  columns,
  rows,
  rowKey,
  className,
  onRowClick,
  footer,
}: {
  columns: Column<T>[];
  rows: T[];
  rowKey: (row: T) => string;
  className?: string;
  onRowClick?: (row: T) => void;
  /** Summary or pagination strip under the rows (prototype .table-foot). */
  footer?: React.ReactNode;
}) {
  return (
    <div className={cn("overflow-hidden rounded-tile border border-border bg-surface", className)}>
      <div className="relative overflow-x-auto">
        <table className="w-full border-collapse text-caption">
          <thead>
            <tr className="border-b border-border bg-surface-secondary">
              {columns.map((col) => (
                <th
                  key={col.key}
                  className={cn(
                    "px-5 py-3 text-left text-label font-medium whitespace-nowrap text-muted",
                    col.headerClassName,
                  )}
                >
                  {col.header}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr
                key={rowKey(row)}
                onClick={onRowClick ? () => onRowClick(row) : undefined}
                onKeyDown={
                  onRowClick
                    ? (e) => {
                        if (e.key !== "Enter" && e.key !== " ") return;
                        e.preventDefault();
                        onRowClick(row);
                      }
                    : undefined
                }
                tabIndex={onRowClick ? 0 : undefined}
                className={cn(
                  "border-b border-border last:border-b-0",
                  onRowClick &&
                    "cursor-pointer outline-none hover:bg-surface-secondary focus-visible:bg-surface-secondary focus-visible:ring-2 focus-visible:ring-focus focus-visible:ring-inset",
                )}
              >
                {columns.map((col) => (
                  <td key={col.key} className={cn("px-5 py-4 align-middle text-caption text-foreground", col.className)}>
                    {col.render(row)}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {footer && <div className="flex items-center justify-between gap-3 border-t border-border px-6 py-3.5 text-label text-muted">{footer}</div>}
    </div>
  );
}
