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
  selectedKeys,
  onSelectionChange,
  isRowSelectable,
  maxSelected,
  isSelectionDisabled,
  selectAllLabel,
  selectRowLabel,
}: {
  columns: Column<T>[];
  rows: T[];
  rowKey: (row: T) => string;
  className?: string;
  onRowClick?: (row: T) => void;
  /** Summary or pagination strip under the rows (prototype .table-foot). */
  footer?: React.ReactNode;
  /** Passing `selectedKeys` and `onSelectionChange` adds a checkbox column; `rowKey` identifies the selected rows. */
  selectedKeys?: ReadonlySet<string>;
  onSelectionChange?: (keys: Set<string>) => void;
  /** Rows that return false cannot be ticked; defaults to every row. */
  isRowSelectable?: (row: T) => boolean;
  /** Most rows that may be ticked at once: further rows are disabled and the header checkbox stops there. */
  maxSelected?: number;
  /** Disables every checkbox, e.g. while the selection is being processed. */
  isSelectionDisabled?: boolean;
  selectAllLabel?: string;
  selectRowLabel?: (row: T) => string;
}) {
  const selectable = selectedKeys !== undefined && onSelectionChange !== undefined;
  const canTick = (row: T) => (isRowSelectable ? isRowSelectable(row) : true);
  const candidates = selectable ? rows.filter(canTick).map(rowKey) : [];
  const selectedCount = candidates.filter((key) => selectedKeys?.has(key)).length;
  const allSelected = candidates.length > 0 && selectedCount === candidates.length;
  const atMax = maxSelected !== undefined && (selectedKeys?.size ?? 0) >= maxSelected;

  const toggleRow = (key: string) => {
    if (!selectedKeys || !onSelectionChange) return;
    const next = new Set(selectedKeys);
    if (!next.delete(key)) next.add(key);
    onSelectionChange(next);
  };

  const toggleAll = () => {
    if (!selectedKeys || !onSelectionChange) return;
    const next = new Set(selectedKeys);
    if (allSelected || atMax) {
      candidates.forEach((key) => next.delete(key));
    } else {
      for (const key of candidates) {
        if (maxSelected !== undefined && next.size >= maxSelected) break;
        next.add(key);
      }
    }
    onSelectionChange(next);
  };

  return (
    <div className={cn("overflow-hidden rounded-tile border border-border bg-surface", className)}>
      <div className="relative overflow-x-auto">
        <table className="w-full border-collapse text-caption">
          <thead>
            <tr className="border-b border-border bg-surface-secondary">
              {selectable && (
                <th className="w-10 px-5 py-3">
                  <input
                    type="checkbox"
                    aria-label={selectAllLabel}
                    ref={(el) => {
                      if (el) el.indeterminate = selectedCount > 0 && !allSelected;
                    }}
                    checked={allSelected}
                    disabled={isSelectionDisabled || candidates.length === 0}
                    onChange={toggleAll}
                    className="size-4.5 accent-accent"
                  />
                </th>
              )}
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
                {selectable && (
                  <td
                    className="w-10 px-5 py-4 align-middle"
                    onClick={(e) => e.stopPropagation()}
                    onKeyDown={(e) => e.stopPropagation()}
                  >
                    <input
                      type="checkbox"
                      aria-label={selectRowLabel?.(row)}
                      checked={selectedKeys.has(rowKey(row))}
                      disabled={isSelectionDisabled || !canTick(row) || (!selectedKeys.has(rowKey(row)) && atMax)}
                      onChange={() => toggleRow(rowKey(row))}
                      className="size-4.5 accent-accent"
                    />
                  </td>
                )}
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
