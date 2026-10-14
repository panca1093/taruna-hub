"use client";

import { useEffect, useState } from "react";
import { Card } from "@heroui/react";
import { CheckCircle2, XCircle } from "lucide-react";
import { api, ApiError, ServiceUnavailableError, type Ready } from "@/lib/api/client";
import { useDevApiCheckT } from "@/lib/i18n/dict/dev-api-check";

// Dev proof of browser -> API -> PostgreSQL: renders only what /readyz returned, no mock data.
export default function ApiCheckPage() {
  const t = useDevApiCheckT();
  const [ready, setReady] = useState<Ready | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api
      .ready()
      .then(setReady)
      .catch((e: unknown) => {
        if (e instanceof ServiceUnavailableError) {
          setError(e.reason);
        } else if (e instanceof ApiError) {
          setError(`${e.code}: ${e.message}${e.requestId ? ` (request ${e.requestId})` : ""}`);
        } else {
          setError(e instanceof Error ? e.message : t.unknownFailure);
        }
      });
  }, [t]);

  return (
    <main className="mx-auto max-w-lg px-5 py-16">
      <h1 className="font-display text-xl font-semibold text-foreground">{t.title}</h1>
      <p className="mt-1 text-sm text-muted">{t.subtitle}</p>

      <Card className="mt-6 gap-3 p-5">
        {!ready && !error && <p className="text-sm text-muted">{t.loading}</p>}

        {error && (
          <div className="flex items-start gap-2">
            <XCircle className="mt-0.5 size-4 shrink-0 text-danger" />
            <div>
              <p className="text-sm font-medium text-danger">{t.failure}</p>
              <p className="mt-0.5 font-mono text-xs break-words text-muted">{error}</p>
            </div>
          </div>
        )}

        {ready && (
          <div className="flex items-center gap-2">
            <CheckCircle2 className="size-4 shrink-0 text-success" />
            <div>
              <p className="text-sm font-medium text-foreground">{t.ok}</p>
              <p className="mt-0.5 font-mono text-xs text-muted">{t.okStatus(ready.status)}</p>
            </div>
          </div>
        )}
      </Card>
    </main>
  );
}
