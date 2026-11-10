import { Avatar } from "@heroui/react";
import { cn } from "@/lib/cn";

const sizeMap = { sm: "size-8.5", md: "size-9.5", lg: "size-12" } as const;

const tints = [
  { disc: "bg-peach", body: "var(--accent)" },
  { disc: "bg-viz-blue-soft", body: "var(--viz-blue)" },
  { disc: "bg-viz-green-soft", body: "var(--viz-green)" },
  { disc: "bg-viz-amber-soft", body: "var(--viz-amber)" },
  { disc: "bg-accent-soft", body: "var(--rose)" },
] as const;

const hairPaths = [
  "M13 15a7 7 0 0 1 14 0c-2-3-4.5-4-7-4s-5 1-7 4z",
  "M12 23v-8a8 8 0 0 1 16 0v8h-2.2v-8a5.8 5.8 0 0 0-11.6 0v8z",
  "M13.2 14a7 7 0 0 1 13.6 0c-3-1.6-10.6-1.6-13.6 0z",
] as const;

const hashOf = (name: string) => [...name].reduce((sum, char) => sum + char.charCodeAt(0), 0);

/** Accounts carry no photo, so the figure's tint and hair are picked from the name. Explicit avatarColorFrom/To still render a gradient disc. */
export function GradientAvatar({
  user,
  size = "md",
  className,
}: {
  user: { name: string; avatarColorFrom?: string; avatarColorTo?: string };
  size?: keyof typeof sizeMap;
  className?: string;
}) {
  const custom = user.avatarColorFrom && user.avatarColorTo;
  const hash = hashOf(user.name);
  const tint = tints[hash % tints.length];
  return (
    <Avatar className={cn(sizeMap[size], className)}>
      <Avatar.Fallback
        className={cn("overflow-hidden", sizeMap[size], !custom && tint.disc)}
        style={custom ? { background: `linear-gradient(135deg, ${user.avatarColorFrom}, ${user.avatarColorTo})` } : undefined}
      >
        <svg viewBox="0 0 40 40" className="size-full" role="img" aria-label={user.name}>
          <path d="M5 40c0-8.5 6.5-13.5 15-13.5S35 31.5 35 40z" fill={custom ? "var(--surface)" : tint.body} fillOpacity={custom ? 0.85 : 1} />
          <circle cx="20" cy="17" r="7" fill="var(--surface)" />
          <path d={hairPaths[(hash >> 2) % hairPaths.length]} fill="var(--foreground)" fillOpacity={0.8} />
        </svg>
      </Avatar.Fallback>
    </Avatar>
  );
}
