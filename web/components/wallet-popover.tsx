"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Button, Modal } from "@heroui/react";
import { ArrowRight, Wallet as WalletIcon } from "lucide-react";
import { cn } from "@/lib/cn";
import { api, type Wallet, type WalletEntry } from "@/lib/api/client";
import { formatIDR, formatSignedIDR } from "@/lib/format-money";
import { useStudentWalletT } from "@/lib/i18n/dict/student-wallet";
import { useWalletTopupT } from "@/lib/i18n/dict/wallet-topup";

const RECENT = 4;

export function WalletPopover({ className }: { className?: string }) {
  const t = useWalletTopupT();
  const ledgerT = useStudentWalletT();
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [wallet, setWallet] = useState<Wallet | null>(null);
  const [entries, setEntries] = useState<WalletEntry[] | null>(null);
  const [failed, setFailed] = useState(false);

  const load = useCallback(() => {
    Promise.all([api.wallet.get(), api.wallet.entries({ limit: RECENT })])
      .then(([head, page]) => {
        setWallet(head);
        setEntries(page.entries);
        setFailed(false);
      })
      .catch(() => setFailed(true));
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const onOpenChange = (next: boolean) => {
    setOpen(next);
    if (next) load();
  };

  const go = (href: string) => {
    setOpen(false);
    router.push(href);
  };

  return (
    <>
      <Button
        variant="ghost"
        size="sm"
        className={cn("gap-2", className)}
        aria-haspopup="dialog"
        aria-label={t.popoverTitle}
        onPress={() => onOpenChange(true)}
      >
        <WalletIcon className="size-4" strokeWidth={2} />
        <span className="font-medium tabular-nums">{wallet === null ? "—" : formatIDR(wallet.balance)}</span>
      </Button>

      <Modal isOpen={open} onOpenChange={onOpenChange}>
        <Modal.Backdrop>
          <Modal.Container size="sm" scroll="inside">
            <Modal.Dialog>
              <Modal.Header>
                <Modal.Heading className="text-lg font-semibold text-foreground">{t.popoverTitle}</Modal.Heading>
                <p className="mt-1 text-sm text-muted">{t.popoverDescription}</p>
                <Modal.CloseTrigger aria-label={t.popoverClose} />
              </Modal.Header>
              <Modal.Body className="flex flex-col gap-5">
                {failed ? (
                  <div className="space-y-2">
                    <p className="text-sm text-danger">{t.popoverLoadFailed}</p>
                    <Button size="sm" variant="secondary" onPress={load}>
                      {t.popoverRetry}
                    </Button>
                  </div>
                ) : (
                  <div>
                    <p className="font-display text-3xl font-semibold tracking-tight text-foreground">
                      {wallet === null ? "—" : formatIDR(wallet.balance)}
                    </p>
                    <p className="mt-1 text-sm text-muted">{t.popoverAvailable}</p>
                  </div>
                )}
                <div className="flex flex-wrap gap-2">
                  <Button variant="primary" size="sm" onPress={() => go("/student/wallet/topup")}>
                    {t.popoverTopUp}
                    <ArrowRight className="size-4" />
                  </Button>
                  <Button variant="secondary" size="sm" onPress={() => go("/student/wallet")}>
                    {t.popoverHistory}
                    <ArrowRight className="size-4" />
                  </Button>
                </div>
                <div>
                  <p className="border-b border-border pb-2 text-sm font-semibold text-foreground">{t.popoverRecent}</p>
                  {entries === null ? (
                    <p className="py-3 text-sm text-muted">{failed ? "" : t.loading}</p>
                  ) : entries.length === 0 ? (
                    <p className="py-3 text-sm text-muted">{t.popoverRecentEmpty}</p>
                  ) : (
                    <ul>
                      {entries.map((entry) => {
                        const credit = !entry.amount.startsWith("-");
                        return (
                          <li
                            key={entry.id}
                            className="flex items-center justify-between gap-3 border-b border-border py-2.5 text-sm last:border-b-0"
                          >
                            <div className="min-w-0">
                              <p className="truncate font-medium text-foreground">
                                {entry.reason === "" ? (ledgerT.typeLabel[entry.kind] ?? entry.kind) : entry.reason}
                              </p>
                              <p className="text-xs text-muted">{new Date(entry.created_at).toLocaleDateString()}</p>
                            </div>
                            <span
                              className={cn("font-mono font-medium whitespace-nowrap", credit ? "text-success" : "text-foreground")}
                            >
                              {formatSignedIDR(entry.amount)}
                            </span>
                          </li>
                        );
                      })}
                    </ul>
                  )}
                </div>
              </Modal.Body>
            </Modal.Dialog>
          </Modal.Container>
        </Modal.Backdrop>
      </Modal>
    </>
  );
}
