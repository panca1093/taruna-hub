"use client";

import Link from "next/link";
import { Logo } from "./logo";
import { ThemeToggle } from "./theme-toggle";
import { LanguageToggle } from "./language-toggle";
import { LinkButton } from "./ui/link-button";
import { usePublicNavT } from "@/lib/i18n/dict/public-nav";

export function PublicNav() {
  const t = usePublicNavT();
  return (
    <header className="sticky top-0 z-30 border-b border-border bg-surface/85 backdrop-blur-sm">
      <div className="mx-auto flex h-16 max-w-6xl items-center gap-6 px-5">
        <Logo />
        <nav className="hidden items-center gap-6 text-sm font-medium text-muted md:flex">
          <Link href="/catalog" className="transition-colors hover:text-foreground">
            {t.catalog}
          </Link>
          <Link href="/#cara-kerja" className="transition-colors hover:text-foreground">
            {t.howItWorks}
          </Link>
          <Link href="/#program" className="transition-colors hover:text-foreground">
            {t.programs}
          </Link>
        </nav>
        <div className="ml-auto flex items-center gap-2">
          <LanguageToggle />
          <ThemeToggle />
          <LinkButton href="/login" variant="ghost" size="sm">
            {t.login}
          </LinkButton>
          <LinkButton href="/register" variant="primary" size="sm">
            {t.register}
          </LinkButton>
        </div>
      </div>
    </header>
  );
}
