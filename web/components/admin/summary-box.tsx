import { Panel } from "@/components/page-blocks";

export interface SummaryLine {
  label: string;
  value: React.ReactNode;
}

export function SummaryBox({ title, lines, children }: { title: string; lines: SummaryLine[]; children?: React.ReactNode }) {
  return (
    <Panel as="aside" className="self-start lg:sticky lg:top-6">
      <h2 className="mb-5 font-display text-heading-sm">{title}</h2>
      <dl>
        {lines.map((line) => (
          <div key={line.label} className="flex justify-between gap-4 border-b border-border py-3 text-caption last:border-b-0">
            <dt className="text-muted">{line.label}</dt>
            <dd className="font-semibold tabular-nums">{line.value}</dd>
          </div>
        ))}
      </dl>
      {children && <div className="mt-5">{children}</div>}
    </Panel>
  );
}
