import { Avatar } from "@heroui/react";
import { cn } from "@/lib/cn";
import { initials } from "@/lib/mock-data";
import type { AppUser } from "@/lib/types";

const sizeMap = { sm: "size-7 text-[11px]", md: "size-9 text-xs", lg: "size-12 text-sm" } as const;

export function GradientAvatar({
  user,
  size = "md",
  className,
}: {
  user: Pick<AppUser, "name" | "avatarColorFrom" | "avatarColorTo">;
  size?: keyof typeof sizeMap;
  className?: string;
}) {
  return (
    <Avatar className={cn(sizeMap[size], className)}>
      <Avatar.Fallback
        className={cn("font-display font-semibold text-white", sizeMap[size])}
        style={{ background: `linear-gradient(135deg, ${user.avatarColorFrom}, ${user.avatarColorTo})` }}
      >
        {initials(user.name)}
      </Avatar.Fallback>
    </Avatar>
  );
}
