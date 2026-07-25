"use client";

import { Button } from "@heroui/react";
import { useLocale } from "@/lib/i18n/locale-context";
import { useCommonT } from "@/lib/i18n/dict/common";

export function LanguageToggle({ className }: { className?: string }) {
  const { locale, setLocale } = useLocale();
  const t = useCommonT();
  const next = locale === "id" ? "en" : "id";

  return (
    <Button
      variant="ghost"
      size="sm"
      className={className}
      aria-label={locale === "id" ? t.switchToEnglish : t.switchToIndonesian}
      onPress={() => setLocale(next)}
    >
      {next.toUpperCase()}
    </Button>
  );
}
