"use client";

import { ThemeProvider } from "next-themes";
import { Toast, ToastProvider } from "@heroui/react";
import { SessionProvider } from "@/lib/session-context";
import { LocaleProvider } from "@/lib/i18n/locale-context";

export function Providers({ children }: { children: React.ReactNode }) {
  return (
    <ThemeProvider attribute="class" defaultTheme="light" enableSystem={false}>
      <LocaleProvider>
        <SessionProvider>
          {children}
          <ToastProvider placement="bottom end">
            {({ toast }) => <Toast toast={toast} />}
          </ToastProvider>
        </SessionProvider>
      </LocaleProvider>
    </ThemeProvider>
  );
}
