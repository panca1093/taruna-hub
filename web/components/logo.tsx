import Image from "next/image";
import Link from "next/link";
import { cn } from "@/lib/cn";

/** `size="md"` is the 132px-wide topbar logo; `plate` puts the light plate behind it in dark theme. */
export function Logo({ collapsed = false, size = "sm", plate, className }: { collapsed?: boolean; size?: "sm" | "md"; plate?: boolean; className?: string }) {
  return (
    <Link href="/" className={cn("flex items-center outline-none", plate && "logo-plate", className)}>
      {collapsed ? (
        <Image src="/taruna-eduka-icon.png" alt="Taruna Eduka" width={187} height={230} className="h-8 w-auto" priority />
      ) : (
        <Image src="/taruna-eduka-logo.png" alt="Taruna Eduka" width={750} height={250} className={cn("w-auto", size === "md" ? "h-11" : "h-9")} priority />
      )}
    </Link>
  );
}
