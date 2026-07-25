import Link from "next/link";
import { buttonVariants, type ButtonVariants } from "@heroui/react";
import { cn } from "@/lib/cn";

interface LinkButtonProps extends ButtonVariants {
  href: string;
  className?: string;
  children: React.ReactNode;
  target?: string;
}

export function LinkButton({ href, className, children, target, ...variants }: LinkButtonProps) {
  return (
    <Link href={href} target={target} className={cn(buttonVariants(variants), className)}>
      {children}
    </Link>
  );
}
