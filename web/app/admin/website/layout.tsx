"use client";

import { RequireRole } from "@/components/require-role";
import { WebsiteEditorProvider } from "@/components/admin/website-editor";
import { useWebsiteT } from "@/lib/i18n/dict/website";

export default function WebsiteLayout({ children }: { children: React.ReactNode }) {
  const t = useWebsiteT();
  return (
    <RequireRole roles={["admin"]} title={t.pageTitle} deniedMessage={t.deniedMessage}>
      <WebsiteEditorProvider>{children}</WebsiteEditorProvider>
    </RequireRole>
  );
}
