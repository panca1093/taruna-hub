"use client";

import { useEffect, useState } from "react";
import { Card } from "@heroui/react";
import { CheckCircle2, XCircle } from "lucide-react";
import { api, ApiError, ServiceUnavailableError, type Ready, type Tenant } from "@/lib/api/client";
import { useDevApiCheckT } from "@/lib/i18n/dict/dev-api-check";

// Dev proof of browser -> API -> PostgreSQL: renders only what the API returned, no mock data.
export default function ApiCheckPage() {
  const t = useDevApiCheckT();
  const [ready, setReady] = useState<Ready | null>(null);
  const [readyError, setReadyError] = useState<string | null>(null);
  const [tenant, setTenant] = useState<Tenant | null>(null);
  const [tenantError, setTenantError] = useState<string | null>(null);
  const [tenantMissing, setTenantMissing] = useState(false);

  useEffect(() => {
    api
      .ready()
      .then(setReady)
      .catch((e: unknown) => setReadyError(describeFailure(e, t.unknownFailure)));
    api
      .tenant()
      .then(setTenant)
      .catch((e: unknown) => {
        setTenantMissing(e instanceof ApiError && e.status === 404);
        setTenantError(describeFailure(e, t.unknownFailure));
      });
  }, [t]);

  return (
    <main className="mx-auto max-w-lg px-5 py-16">
      <h1 className="font-display text-xl font-semibold text-foreground">{t.title}</h1>
      <p className="mt-1 text-sm text-muted">{t.subtitle}</p>

      <Card className="mt-6 gap-3 p-5">
        <div className="flex items-start gap-2">
          {ready && <CheckCircle2 className="mt-0.5 size-4 shrink-0 text-success" />}
          {readyError && <XCircle className="mt-0.5 size-4 shrink-0 text-danger" />}
          <div>
            {!ready && !readyError && <p className="text-sm text-muted">{t.loading}</p>}
            {readyError && (
              <>
                <p className="text-sm font-medium text-danger">{t.failure}</p>
                <p className="mt-0.5 font-mono text-xs break-words text-muted">{readyError}</p>
              </>
            )}
            {ready && (
              <>
                <p className="text-sm font-medium text-foreground">{t.ok}</p>
                <p className="mt-0.5 font-mono text-xs text-muted">{t.okStatus(ready.status)}</p>
              </>
            )}
          </div>
        </div>

        <div className="flex items-start gap-2">
          {tenant && <CheckCircle2 className="mt-0.5 size-4 shrink-0 text-success" />}
          {tenantError && <XCircle className="mt-0.5 size-4 shrink-0 text-danger" />}
          <div>
            {!tenant && !tenantError && <p className="text-sm text-muted">{t.loading}</p>}
            {tenantError && (
              <>
                <p className="text-sm font-medium text-danger">
                  {tenantMissing ? t.tenantFailure : t.failure}
                </p>
                <p className="mt-0.5 font-mono text-xs break-words text-muted">{tenantError}</p>
              </>
            )}
            {tenant && (
              <>
                <p className="text-sm font-medium text-foreground">{t.tenantOk(tenant.name)}</p>
                <p className="mt-0.5 font-mono text-xs text-muted">{t.tenantSlug(tenant.slug)}</p>
              </>
            )}
          </div>
        </div>
      </Card>
    </main>
  );
}

// A 503 from /readyz carries only a reason; every other failure is the error envelope.
function describeFailure(error: unknown, fallback: string): string {
  if (error instanceof ServiceUnavailableError) return error.reason;
  if (error instanceof ApiError) {
    return `${error.code}: ${error.message}${error.requestId ? ` (request ${error.requestId})` : ""}`;
  }
  return error instanceof Error ? error.message : fallback;
}
