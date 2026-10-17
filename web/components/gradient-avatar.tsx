import { Avatar } from "@heroui/react";
import { cn } from "@/lib/cn";
import { initials } from "@/lib/mock-data";

const sizeMap = { sm: "size-7 text-[11px]", md: "size-9 text-xs", lg: "size-12 text-sm" } as const;

// Accounts carry no avatar colors, so derive a stable pair from the name.
function hueColors(name: string): [string, string] {
  let h = 0;
  for (const ch of name) h = (h * 31 + ch.charCodeAt(0)) % 360;
  return [`hsl(${h} 70% 55%)`, `hsl(${(h + 40) % 360} 70% 45%)`];
}

export function GradientAvatar({
  user,
  size = "md",
  className,
}: {
  user: { name: string; avatarColorFrom?: string; avatarColorTo?: string };
  size?: keyof typeof sizeMap;
  className?: string;
}) {
  const [from, to] = user.avatarColorFrom && user.avatarColorTo ? [user.avatarColorFrom, user.avatarColorTo] : hueColors(user.name);
  return (
    <Avatar className={cn(sizeMap[size], className)}>
      <Avatar.Fallback
        className={cn("font-display font-semibold text-white", sizeMap[size])}
        style={{ background: `linear-gradient(135deg, ${from}, ${to})` }}
      >
        {initials(user.name)}
      </Avatar.Fallback>
    </Avatar>
  );
}
