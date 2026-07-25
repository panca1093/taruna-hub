"use client";

import { useEffect, useState } from "react";
import { Card } from "@heroui/react";
import { CheckCircle2, XCircle } from "lucide-react";
import { api, ApiError, type User } from "@/lib/api/client";

/**
 * S0 walking-skeleton proof: this page holds no mock data, so anything it renders
 * came from Postgres through the generated typed client. Delete once S1 wires real
 * auth and a real page consumes the API.
 */
export default function ApiCheckPage() {
  const [user, setUser] = useState<User | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api
      .me()
      .then(setUser)
      .catch((e: unknown) => {
        if (e instanceof ApiError) {
          setError(`${e.code}: ${e.message}${e.requestId ? ` (request ${e.requestId})` : ""}`);
        } else {
          setError(e instanceof Error ? e.message : "Unknown failure");
        }
      });
  }, []);

  return (
    <main className="mx-auto max-w-lg px-5 py-16">
      <h1 className="font-display text-xl font-semibold text-foreground">S0 — API check</h1>
      <p className="mt-1 text-sm text-muted">
        GET <code className="font-mono">/api/v1/me</code> via the generated client. No mock data on this page.
      </p>

      <Card className="mt-6 gap-3 p-5">
        {!user && !error && <p className="text-sm text-muted">Loading…</p>}

        {error && (
          <div className="flex items-start gap-2">
            <XCircle className="mt-0.5 size-4 shrink-0 text-danger" />
            <div>
              <p className="text-sm font-medium text-danger">Request failed</p>
              <p className="mt-0.5 font-mono text-xs text-muted">{error}</p>
            </div>
          </div>
        )}

        {user && (
          <>
            <div className="flex items-center gap-2">
              <CheckCircle2 className="size-4 text-success" />
              <p className="text-sm font-medium text-foreground">Live from Postgres</p>
            </div>
            <dl className="divide-y divide-border border-t border-border pt-1 text-sm">
              {[
                ["id", user.id],
                ["name", user.name],
                ["email", user.email ?? "—"],
                ["phone", user.phone ?? "—"],
                ["role", user.role],
                ["balance", String(user.balance)],
              ].map(([k, v]) => (
                <div key={k} className="flex justify-between gap-4 py-2">
                  <dt className="text-muted">{k}</dt>
                  <dd className="font-mono text-xs text-foreground">{v}</dd>
                </div>
              ))}
            </dl>
          </>
        )}
      </Card>
    </main>
  );
}
