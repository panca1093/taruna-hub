import { Avatar } from "@heroui/react";
import { cn } from "@/lib/cn";
import { initials } from "@/lib/mock-data";

const sizeMap = { sm: "size-8.5 text-caption", md: "size-9.5 text-title", lg: "size-12 text-heading-sm" } as const;

/**
 * Accounts carry no avatar colors, so the default is the prototype's flat peach
 * disc with accent initials. Explicit avatarColorFrom/To still render a gradient.
 */
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
  return (
    <Avatar className={cn(sizeMap[size], className)}>
      <Avatar.Fallback
        className={cn("font-display font-semibold", sizeMap[size], custom ? "text-white" : "bg-peach text-accent")}
        style={custom ? { background: `linear-gradient(135deg, ${user.avatarColorFrom}, ${user.avatarColorTo})` } : undefined}
      >
        {initials(user.name)}
      </Avatar.Fallback>
    </Avatar>
  );
}
