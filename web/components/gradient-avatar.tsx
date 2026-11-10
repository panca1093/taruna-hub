import Image from "next/image";
import { Avatar } from "@heroui/react";
import { User } from "lucide-react";
import { cn } from "@/lib/cn";

const sizeMap = { sm: "size-8.5", md: "size-9.5", lg: "size-12", xl: "size-20", fill: "size-full" } as const;

// Keys are the API's avatar values (a01..a35), one file each in public/avatars.
export const AVATAR_KEYS = Array.from({ length: 35 }, (_, index) => `a${String(index + 1).padStart(2, "0")}`);

const placeholderTints = [
  "bg-peach text-accent",
  "bg-viz-blue-soft text-viz-blue",
  "bg-viz-green-soft text-viz-green",
  "bg-viz-amber-soft text-viz-amber",
  "bg-accent-soft text-accent",
] as const;

const hashOf = (name: string) => [...name].reduce((sum, char) => sum + char.charCodeAt(0), 0);

/** The built-in avatar a user picked. Without a pick it is a person glyph on a name-picked tint, never a face the user did not choose. */
export function GradientAvatar({
  user,
  size = "md",
  label,
  className,
}: {
  user: { name: string; avatar?: string | null };
  size?: keyof typeof sizeMap;
  label?: string;
  className?: string;
}) {
  const picked = user.avatar && AVATAR_KEYS.includes(user.avatar) ? user.avatar : null;
  return (
    <Avatar className={cn("overflow-hidden rounded-full", sizeMap[size], className)}>
      {picked ? (
        <Avatar.Fallback className="size-full">
          <Image src={`/avatars/${picked}.webp`} alt={label ?? user.name} width={192} height={192} unoptimized className="size-full object-cover" />
        </Avatar.Fallback>
      ) : (
        <Avatar.Fallback className={cn("grid size-full place-items-center", placeholderTints[hashOf(user.name) % placeholderTints.length])}>
          <User role="img" aria-label={label ?? user.name} className="size-3/5" />
        </Avatar.Fallback>
      )}
    </Avatar>
  );
}
