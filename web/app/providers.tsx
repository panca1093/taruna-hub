"use client";

import { ThemeProvider } from "next-themes";
import { MotionConfig } from "framer-motion";
import { ToastProvider } from "@heroui/react";
import { SessionProvider } from "@/lib/session-context";
import { LocaleProvider } from "@/lib/i18n/locale-context";

export function Providers({ children }: { children: React.ReactNode }) {
  return (
    <ThemeProvider attribute="class" defaultTheme="light" enableSystem={false}>
      <MotionConfig reducedMotion="user">
        <LocaleProvider>
          <SessionProvider>
            {children}
            <ToastProvider placement="bottom end" />
          </SessionProvider>
        </LocaleProvider>
      </MotionConfig>
    </ThemeProvider>
  );
}
