import Image from "next/image";
import Link from "next/link";
import { cn } from "@/lib/cn";

export function Logo({ collapsed = false, className }: { collapsed?: boolean; className?: string }) {
  return (
    <Link href="/" className={cn("flex items-center outline-none", className)}>
      {collapsed ? (
        <Image src="/taruna-eduka-icon.png" alt="Taruna Eduka" width={187} height={230} className="h-8 w-auto" priority />
      ) : (
        <Image src="/taruna-eduka-logo.png" alt="Taruna Eduka" width={750} height={250} className="h-9 w-auto" priority />
      )}
    </Link>
  );
}
